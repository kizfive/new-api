package modelroute

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	defaultSidecarTimeout = 8 * time.Second
	maxSidecarBodyBytes   = 1 << 20
)

// Client is the read-only HTTP client for omc-cost-sidecar. It never receives
// the CPA management key and never writes to OMC.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type CatalogResponse struct {
	UpdatedAtMS int64             `json:"updated_at_ms"`
	Providers   []CatalogProvider `json:"providers"`
}

type CatalogProvider struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Channel  string         `json:"channel"`
	Prefix   string         `json:"prefix"`
	Priority int            `json:"priority"`
	Models   []CatalogModel `json:"models"`
}

type CatalogModel struct {
	RootModel string `json:"root_model"`
	CallPoint string `json:"call_point"`
}

type UsageResponse struct {
	Found               bool   `json:"found"`
	RequestID           string `json:"request_id"`
	Provider            string `json:"provider"`
	Model               string `json:"model"`
	ModelAlias          string `json:"model_alias"`
	CostNanos           *int64 `json:"cost_nanos"`
	PricingStatus       string `json:"pricing_status"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheReadTokens     int64  `json:"cache_read_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	TotalTokens         int64  `json:"total_tokens"`
	LatencyMS           int64  `json:"latency_ms"`
	Failed              bool   `json:"failed"`
	Attempts            int    `json:"attempts"`
	TimestampMS         int64  `json:"timestamp_ms"`
}

func NewClient() (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OMC_SIDECAR_URL")), "/")
	token := strings.TrimSpace(os.Getenv("OMC_SIDECAR_TOKEN"))
	if baseURL == "" || token == "" {
		return nil, errors.New("OMC_SIDECAR_URL and OMC_SIDECAR_TOKEN are required")
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: defaultSidecarTimeout},
	}, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query string, target any) error {
	if c == nil || c.http == nil {
		return errors.New("model route sidecar client is not initialized")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+query, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSidecarBodyBytes))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("model route sidecar %s: status %d: %s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := common.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode model route sidecar %s: %w", path, err)
	}
	return nil
}

func (c *Client) Catalog(ctx context.Context) (CatalogResponse, error) {
	var result CatalogResponse
	err := c.getJSON(ctx, "/catalog", "", &result)
	return result, err
}

func (c *Client) Usage(ctx context.Context, requestID string, fromMS, toMS int64) (UsageResponse, error) {
	var result UsageResponse
	query := "?request_id=" + urlQueryEscape(requestID) + "&from_ms=" + int64String(fromMS) + "&to_ms=" + int64String(toMS)
	err := c.getJSON(ctx, "/usage", query, &result)
	return result, err
}

func urlQueryEscape(value string) string { return url.QueryEscape(value) }

func int64String(value int64) string { return strconv.FormatInt(value, 10) }
