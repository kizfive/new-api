package modelroute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultRouteSyncInterval = 10 * time.Minute
	costLookupWindowMS       = 10 * 60 * 1000
	costLookupTimeout        = 60 * 60
)

var (
	costLogMu sync.Mutex
	costLogAt time.Time
)

// Start launches the catalog sync and the asynchronous cost backfill worker.
// It is a no-op when the sidecar is not configured.
func Start() {
	client, err := NewClient()
	if err != nil {
		common.SysLog(common.LogText("model route sidecar disabled: %s", err.Error()))
		return
	}
	model.RefreshModelRouteCatalogCache()
	go syncLoop(client)
	go costLoop(client)
}

// SyncNow pulls one complete catalog snapshot and publishes it atomically.
func SyncNow(ctx context.Context) (int, error) {
	client, err := NewClient()
	if err != nil {
		return 0, err
	}
	catalog, err := client.Catalog(ctx)
	if err != nil {
		return 0, err
	}
	syncedAt := time.Now().Unix()
	rows := make([]model.ModelRouteCatalog, 0)
	seen := make(map[string]bool)
	for _, provider := range catalog.Providers {
		for _, item := range provider.Models {
			rootModel := strings.TrimSpace(item.RootModel)
			callPoint := strings.TrimSpace(item.CallPoint)
			if rootModel == "" || callPoint == "" {
				continue
			}
			key := rootModel + "\x00" + callPoint
			if seen[key] {
				continue
			}
			seen[key] = true
			rows = append(rows, model.ModelRouteCatalog{
				RootModel:       rootModel,
				CallPoint:       callPoint,
				ProviderId:      provider.ID,
				ProviderName:    provider.Name,
				ProviderChannel: provider.Channel,
				Priority:        provider.Priority,
				SyncedAt:        syncedAt,
			})
		}
	}
	if len(rows) == 0 {
		return 0, errors.New("sidecar catalog contained no usable root model routes")
	}
	return model.ReplaceModelRouteCatalog(ctx, rows)
}

func syncLoop(client *Client) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	<-timer.C
	syncOnce(client)
	ticker := time.NewTicker(routeSyncInterval())
	defer ticker.Stop()
	for range ticker.C {
		syncOnce(client)
	}
}

func syncOnce(client *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	count, err := SyncNow(ctx)
	if err != nil {
		common.SysLog(common.LogText("model route catalog sync failed: %s", err.Error()))
		return
	}
	common.SysLog(common.LogText("model route catalog synced: %d entries", count))
}

func routeSyncInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("MODEL_ROUTE_SYNC_INTERVAL"))
	if raw == "" {
		return defaultRouteSyncInterval
	}
	if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
		return duration
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return defaultRouteSyncInterval
}

type pendingCostRow struct {
	Id            int
	CreatedAt     int64
	CpaRequestId  string
	CostAttempts  int
	CostCheckedAt int64
}

func costLoop(client *Client) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		processPendingCosts(client)
	}
}

func processPendingCosts(client *Client) {
	if model.LOG_DB == nil {
		return
	}
	now := time.Now().Unix()
	cutoff := now - costLookupTimeout
	// A row that outlived the lookup window can never be recovered by the
	// sidecar; mark it once so the UI stops showing "pending" forever.
	if err := model.LOG_DB.Model(&model.Log{}).
		Where("type = ? AND cost_status = ? AND created_at < ?", model.LogTypeConsume, "pending", cutoff).
		Update("cost_status", "missing").Error; err != nil {
		logCostError(fmt.Errorf("expire pending cost logs: %w", err))
	}
	var rows []pendingCostRow
	err := model.LOG_DB.Model(&model.Log{}).
		Select("id, created_at, cpa_request_id, cost_attempts, cost_checked_at").
		Where("type = ? AND cost_status = ? AND cpa_request_id <> '' AND created_at >= ?", model.LogTypeConsume, "pending", cutoff).
		Order("id asc").
		Limit(20).
		Find(&rows).Error
	if err != nil {
		logCostError(fmt.Errorf("query pending cost logs: %w", err))
		return
	}
	for _, row := range rows {
		if row.CostCheckedAt > 0 && now < row.CostCheckedAt+int64(costRetryDelay(row.CostAttempts).Seconds()) {
			continue
		}
		processCostRow(client, row, now)
	}
}

func processCostRow(client *Client, row pendingCostRow, now int64) {
	fromMS := row.CreatedAt*1000 - costLookupWindowMS
	toMS := row.CreatedAt*1000 + costLookupWindowMS
	response, err := client.Usage(context.Background(), row.CpaRequestId, fromMS, toMS)
	attempts := row.CostAttempts + 1
	updates := map[string]any{
		"cost_checked_at": now,
		"cost_attempts":   attempts,
	}
	if err != nil {
		if now-row.CreatedAt >= costLookupTimeout {
			updates["cost_status"] = "missing"
		}
		applyCostUpdates(row.Id, updates)
		logCostError(fmt.Errorf("lookup %s: %w", row.CpaRequestId, err))
		return
	}
	if !response.Found {
		if now-row.CreatedAt >= costLookupTimeout {
			updates["cost_status"] = "missing"
		}
		applyCostUpdates(row.Id, updates)
		return
	}
	upstreamModel := strings.TrimSpace(response.ModelAlias)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(response.Model)
	}
	updates["upstream_provider"] = strings.TrimSpace(response.Provider)
	updates["upstream_model"] = upstreamModel
	updates["pricing_status"] = strings.TrimSpace(response.PricingStatus)
	if response.CostNanos != nil && strings.EqualFold(strings.TrimSpace(response.PricingStatus), "priced") {
		updates["reference_cost_nanos"] = *response.CostNanos
		updates["cost_status"] = "priced"
	} else {
		updates["reference_cost_nanos"] = nil
		updates["cost_status"] = "unpriced"
	}
	applyCostUpdates(row.Id, updates)
}

func applyCostUpdates(logId int, updates map[string]any) {
	if err := model.LOG_DB.Model(&model.Log{}).Where("id = ?", logId).Updates(updates).Error; err != nil {
		logCostError(fmt.Errorf("update cost log %d: %w", logId, err))
	}
}

func costRetryDelay(attempts int) time.Duration {
	delays := []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	if attempts < 0 {
		attempts = 0
	}
	if attempts >= len(delays) {
		return delays[len(delays)-1]
	}
	return delays[attempts]
}

func logCostError(err error) {
	costLogMu.Lock()
	defer costLogMu.Unlock()
	if time.Since(costLogAt) < 30*time.Second {
		return
	}
	costLogAt = time.Now()
	common.SysLog(common.LogText("model cost backfill: %s", err.Error()))
}
