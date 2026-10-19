package model

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ModelRouteCatalog is one selectable upstream call point for a root model.
// The catalog is written by the sidecar sync and never by a request path.
type ModelRouteCatalog struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	RootModel       string `json:"root_model" gorm:"type:varchar(255);not null;uniqueIndex:idx_model_route_catalog_root_call,priority:1"`
	CallPoint       string `json:"call_point" gorm:"type:varchar(255);not null;uniqueIndex:idx_model_route_catalog_root_call,priority:2"`
	ProviderId      string `json:"provider_id" gorm:"type:varchar(128);default:''"`
	ProviderName    string `json:"provider_name" gorm:"type:varchar(255);default:''"`
	ProviderChannel string `json:"provider_channel" gorm:"type:varchar(255);default:''"`
	Priority        int    `json:"priority" gorm:"default:0"`
	SyncedAt        int64  `json:"synced_at" gorm:"bigint;default:0"`
}

func (ModelRouteCatalog) TableName() string { return "model_route_catalog" }

// UserModelRoute is a user's default for one root model. An empty CallPoint
// means "auto"; the row exists so the UI can distinguish auto from a fixed
// upstream.
type UserModelRoute struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"uniqueIndex:idx_user_model_route,priority:1"`
	RootModel       string `json:"root_model" gorm:"type:varchar(255);not null;uniqueIndex:idx_user_model_route,priority:2"`
	CallPoint       string `json:"call_point" gorm:"type:varchar(255);default:''"`
	FallbackEnabled bool   `json:"fallback_enabled" gorm:"not null;default:false"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;default:0"`
}

func (UserModelRoute) TableName() string { return "user_model_routes" }

// TokenModelRoute overrides a user route for one API key. Absence means
// "inherit the user route"; an empty CallPoint means "auto for this key".
type TokenModelRoute struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	TokenId         int    `json:"token_id" gorm:"uniqueIndex:idx_token_model_route,priority:1"`
	UserId          int    `json:"user_id" gorm:"index"`
	RootModel       string `json:"root_model" gorm:"type:varchar(255);not null;uniqueIndex:idx_token_model_route,priority:2"`
	CallPoint       string `json:"call_point" gorm:"type:varchar(255);default:''"`
	FallbackEnabled bool   `json:"fallback_enabled" gorm:"not null;default:false"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;default:0"`
}

func (TokenModelRoute) TableName() string { return "token_model_routes" }

// ModelRoutePreference is the resolved route for one request.
type ModelRoutePreference struct {
	CallPoint       string
	FallbackEnabled bool
	Source          string
}

type modelRouteCacheEntry struct {
	revision int64
	routes   map[string]ModelRoutePreference
}

var (
	modelRouteRevision atomic.Int64
	modelRouteCache    sync.Map
	modelRouteCatalog  atomic.Value // map[string]map[string]bool
	modelRouteSyncMu   sync.Mutex
)

// InvalidateModelRouteCache makes every cached user/key resolution stale.
func InvalidateModelRouteCache() { modelRouteRevision.Add(1) }

// ResolveModelRoute returns the effective route for a user/key/model. The
// caller must treat Source == "auto" as "no fixed upstream".
func ResolveModelRoute(userId, tokenId int, rootModel string) ModelRoutePreference {
	rootModel = strings.TrimSpace(rootModel)
	if rootModel == "" {
		return ModelRoutePreference{Source: "auto", FallbackEnabled: true}
	}
	key := struct {
		userId  int
		tokenId int
	}{userId, tokenId}
	revision := modelRouteRevision.Load()
	if value, ok := modelRouteCache.Load(key); ok {
		if entry, ok := value.(modelRouteCacheEntry); ok && entry.revision == revision {
			if route, exists := entry.routes[rootModel]; exists {
				return route
			}
			return ModelRoutePreference{Source: "auto", FallbackEnabled: true}
		}
	}
	routes := loadModelRoutes(userId, tokenId)
	modelRouteCache.Store(key, modelRouteCacheEntry{revision: revision, routes: routes})
	if route, exists := routes[rootModel]; exists {
		return route
	}
	return ModelRoutePreference{Source: "auto", FallbackEnabled: true}
}

func loadModelRoutes(userId, tokenId int) map[string]ModelRoutePreference {
	routes := make(map[string]ModelRoutePreference)
	if DB == nil {
		return routes
	}
	if userId > 0 {
		var rows []UserModelRoute
		if err := DB.Where("user_id = ?", userId).Find(&rows).Error; err == nil {
			for _, row := range rows {
				routes[row.RootModel] = ModelRoutePreference{
					CallPoint:       strings.TrimSpace(row.CallPoint),
					FallbackEnabled: row.FallbackEnabled,
					Source:          "user",
				}
			}
		}
	}
	if tokenId > 0 {
		var rows []TokenModelRoute
		if err := DB.Where("token_id = ?", tokenId).Find(&rows).Error; err == nil {
			for _, row := range rows {
				routes[row.RootModel] = ModelRoutePreference{
					CallPoint:       strings.TrimSpace(row.CallPoint),
					FallbackEnabled: row.FallbackEnabled,
					Source:          "token",
				}
			}
		}
	}
	return routes
}

// ReplaceModelRouteCatalog atomically publishes a complete catalog snapshot.
// An empty snapshot is refused: a sidecar outage must not erase the last good
// catalog.
func ReplaceModelRouteCatalog(ctx context.Context, rows []ModelRouteCatalog) (int, error) {
	if DB == nil {
		return 0, errors.New("database is not initialized")
	}
	if len(rows) == 0 {
		return 0, errors.New("refusing to publish an empty model route catalog")
	}
	modelRouteSyncMu.Lock()
	defer modelRouteSyncMu.Unlock()
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&ModelRouteCatalog{}).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 200).Error
	})
	if err != nil {
		return 0, err
	}
	RefreshModelRouteCatalogCache()
	InvalidateModelRouteCache()
	return len(rows), nil
}

func ListModelRouteCatalog(ctx context.Context) ([]ModelRouteCatalog, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	var rows []ModelRouteCatalog
	err := DB.WithContext(ctx).Order("root_model asc, priority desc, provider_name asc, call_point asc").Find(&rows).Error
	return rows, err
}

// RefreshModelRouteCatalogCache builds the in-memory set used by the request
// path to reject a stale call point without a database read per request.
func RefreshModelRouteCatalogCache() {
	if DB == nil {
		modelRouteCatalog.Store(map[string]map[string]bool{})
		return
	}
	var rows []ModelRouteCatalog
	if err := DB.Select("root_model, call_point").Find(&rows).Error; err != nil {
		common.SysError(common.LogText("failed to refresh model route catalog cache: %s", err.Error()))
		return
	}
	cache := make(map[string]map[string]bool, len(rows))
	for _, row := range rows {
		if cache[row.RootModel] == nil {
			cache[row.RootModel] = make(map[string]bool)
		}
		cache[row.RootModel][row.CallPoint] = true
	}
	modelRouteCatalog.Store(cache)
}

func ModelRouteCallPointExists(rootModel, callPoint string) bool {
	value := modelRouteCatalog.Load()
	if value == nil {
		RefreshModelRouteCatalogCache()
		value = modelRouteCatalog.Load()
	}
	cache, ok := value.(map[string]map[string]bool)
	if !ok {
		return false
	}
	return cache[rootModel][callPoint]
}

// ModelRouteWrite is one upsert. CallPoint nil deletes the row (inherit/auto
// for a user); a pointer to "" stores an explicit auto choice.
type ModelRouteWrite struct {
	RootModel       string
	CallPoint       *string
	FallbackEnabled bool
}

func SaveUserModelRoutes(ctx context.Context, userId int, routes []ModelRouteWrite) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Delete(&UserModelRoute{}).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		rows := make([]UserModelRoute, 0, len(routes))
		for _, route := range routes {
			if route.CallPoint == nil {
				continue
			}
			rows = append(rows, UserModelRoute{
				UserId:          userId,
				RootModel:       route.RootModel,
				CallPoint:       *route.CallPoint,
				FallbackEnabled: route.FallbackEnabled,
				UpdatedAt:       now,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
	if err == nil {
		InvalidateModelRouteCache()
	}
	return err
}

func SaveTokenModelRoutes(ctx context.Context, tokenId, userId int, routes []ModelRouteWrite) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("token_id = ?", tokenId).Delete(&TokenModelRoute{}).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		rows := make([]TokenModelRoute, 0, len(routes))
		for _, route := range routes {
			if route.CallPoint == nil {
				continue
			}
			rows = append(rows, TokenModelRoute{
				TokenId:         tokenId,
				UserId:          userId,
				RootModel:       route.RootModel,
				CallPoint:       *route.CallPoint,
				FallbackEnabled: route.FallbackEnabled,
				UpdatedAt:       now,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
	if err == nil {
		InvalidateModelRouteCache()
	}
	return err
}

func ListUserModelRoutes(ctx context.Context, userId int) ([]UserModelRoute, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	var rows []UserModelRoute
	err := DB.WithContext(ctx).Where("user_id = ?", userId).Find(&rows).Error
	return rows, err
}

func ListTokenModelRoutes(ctx context.Context, tokenId int) ([]TokenModelRoute, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	var rows []TokenModelRoute
	err := DB.WithContext(ctx).Where("token_id = ?", tokenId).Find(&rows).Error
	return rows, err
}

// ModelRouteCatalogRoots returns the distinct root models in catalog order.
func ModelRouteCatalogRoots(rows []ModelRouteCatalog) []string {
	seen := make(map[string]bool)
	roots := make([]string, 0)
	for _, row := range rows {
		if !seen[row.RootModel] {
			seen[row.RootModel] = true
			roots = append(roots, row.RootModel)
		}
	}
	sort.Strings(roots)
	return roots
}

// ModelCostFilter scopes the provider/cost aggregate.
type ModelCostFilter struct {
	UserId         int
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	Provider       string
	TokenName      string
	Group          string
}

type ModelCostProviderRow struct {
	Provider           string `json:"provider"`
	Requests           int64  `json:"requests"`
	PromptTokens       int64  `json:"prompt_tokens"`
	CompletionTokens   int64  `json:"completion_tokens"`
	TotalTokens        int64  `json:"total_tokens"`
	ReferenceCostNanos int64  `json:"reference_cost_nanos"`
	Pending            int64  `json:"pending"`
	Priced             int64  `json:"priced"`
	Unpriced           int64  `json:"unpriced"`
	Missing            int64  `json:"missing"`
}

type ModelCostModelRow struct {
	ModelName          string                 `json:"model_name"`
	Requests           int64                  `json:"requests"`
	PromptTokens       int64                  `json:"prompt_tokens"`
	CompletionTokens   int64                  `json:"completion_tokens"`
	TotalTokens        int64                  `json:"total_tokens"`
	ReferenceCostNanos int64                  `json:"reference_cost_nanos"`
	Pending            int64                  `json:"pending"`
	Priced             int64                  `json:"priced"`
	Unpriced           int64                  `json:"unpriced"`
	Missing            int64                  `json:"missing"`
	Providers          []ModelCostProviderRow `json:"providers"`
}

// QueryModelCostStats groups consume logs by root model and actual provider.
func QueryModelCostStats(ctx context.Context, filter ModelCostFilter) ([]ModelCostModelRow, error) {
	if LOG_DB == nil {
		return nil, errors.New("log database is not initialized")
	}
	query := `SELECT model_name, upstream_provider,
		COUNT(*) AS requests,
		COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS total_tokens,
		COALESCE(SUM(reference_cost_nanos), 0) AS reference_cost_nanos,
		SUM(CASE WHEN cost_status = 'pending' THEN 1 ELSE 0 END) AS pending,
		SUM(CASE WHEN cost_status = 'priced' THEN 1 ELSE 0 END) AS priced,
		SUM(CASE WHEN cost_status = 'unpriced' THEN 1 ELSE 0 END) AS unpriced,
		SUM(CASE WHEN cost_status = 'missing' THEN 1 ELSE 0 END) AS missing
		FROM logs WHERE type = ? AND created_at >= ? AND created_at <= ?`
	args := []any{LogTypeConsume, filter.StartTimestamp, filter.EndTimestamp}
	if filter.UserId > 0 {
		query += " AND user_id = ?"
		args = append(args, filter.UserId)
	}
	if strings.TrimSpace(filter.ModelName) != "" {
		query += " AND model_name = ?"
		args = append(args, strings.TrimSpace(filter.ModelName))
	}
	if strings.TrimSpace(filter.Provider) != "" {
		query += " AND upstream_provider = ?"
		args = append(args, strings.TrimSpace(filter.Provider))
	}
	if strings.TrimSpace(filter.TokenName) != "" {
		query += " AND token_name = ?"
		args = append(args, strings.TrimSpace(filter.TokenName))
	}
	if strings.TrimSpace(filter.Group) != "" {
		query += " AND " + logGroupCol + " = ?"
		args = append(args, strings.TrimSpace(filter.Group))
	}
	query += " GROUP BY model_name, upstream_provider ORDER BY total_tokens DESC LIMIT 500"

	type row struct {
		ModelName          string
		UpstreamProvider   string
		Requests           int64
		PromptTokens       int64
		CompletionTokens   int64
		TotalTokens        int64
		ReferenceCostNanos int64
		Pending            int64
		Priced             int64
		Unpriced           int64
		Missing            int64
	}
	var rows []row
	if err := LOG_DB.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	byModel := make(map[string]*ModelCostModelRow)
	order := make([]string, 0)
	for _, item := range rows {
		modelRow := byModel[item.ModelName]
		if modelRow == nil {
			modelRow = &ModelCostModelRow{ModelName: item.ModelName}
			byModel[item.ModelName] = modelRow
			order = append(order, item.ModelName)
		}
		modelRow.Requests += item.Requests
		modelRow.PromptTokens += item.PromptTokens
		modelRow.CompletionTokens += item.CompletionTokens
		modelRow.TotalTokens += item.TotalTokens
		modelRow.ReferenceCostNanos += item.ReferenceCostNanos
		modelRow.Pending += item.Pending
		modelRow.Priced += item.Priced
		modelRow.Unpriced += item.Unpriced
		modelRow.Missing += item.Missing
		modelRow.Providers = append(modelRow.Providers, ModelCostProviderRow{
			Provider:           item.UpstreamProvider,
			Requests:           item.Requests,
			PromptTokens:       item.PromptTokens,
			CompletionTokens:   item.CompletionTokens,
			TotalTokens:        item.TotalTokens,
			ReferenceCostNanos: item.ReferenceCostNanos,
			Pending:            item.Pending,
			Priced:             item.Priced,
			Unpriced:           item.Unpriced,
			Missing:            item.Missing,
		})
	}
	result := make([]ModelCostModelRow, 0, len(order))
	for _, name := range order {
		result = append(result, *byModel[name])
	}
	return result, nil
}
