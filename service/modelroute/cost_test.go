package modelroute

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newCostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func installCostTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldDB := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = oldDB })
}

func installSidecarEnv(t *testing.T, url string) {
	t.Helper()
	oldURL, oldToken := os.Getenv("OMC_SIDECAR_URL"), os.Getenv("OMC_SIDECAR_TOKEN")
	os.Setenv("OMC_SIDECAR_URL", url)
	os.Setenv("OMC_SIDECAR_TOKEN", "test-token")
	t.Cleanup(func() {
		os.Setenv("OMC_SIDECAR_URL", oldURL)
		os.Setenv("OMC_SIDECAR_TOKEN", oldToken)
	})
}

func TestProcessCostRowWritesProviderAndCost(t *testing.T) {
	db := newCostTestDB(t)
	installCostTestDB(t, db)
	now := time.Now().Unix()
	logRow := model.Log{
		UserId:       1,
		CreatedAt:    now,
		Type:         model.LogTypeConsume,
		ModelName:    "glm-5.3",
		CpaRequestId: "018f0c3e-7a1b-7c2d-9e3f-123456789abc",
		CostStatus:   "pending",
	}
	if err := db.Create(&logRow).Error; err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/usage" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("request_id") != logRow.CpaRequestId {
			t.Fatalf("request id = %q", request.URL.Query().Get("request_id"))
		}
		_, _ = writer.Write([]byte(`{"found":true,"request_id":"018f0c3e-7a1b-7c2d-9e3f-123456789abc","provider":"bailian","model":"glm-5.3","model_alias":"Bailian/glm-5.3","cost_nanos":2500,"pricing_status":"priced","input_tokens":10,"output_tokens":5,"cache_read_tokens":0,"cache_creation_tokens":0,"total_tokens":15,"latency_ms":100,"failed":false,"attempts":1,"timestamp_ms":1}`))
	}))
	defer server.Close()
	installSidecarEnv(t, server.URL)
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	processCostRow(client, pendingCostRow{
		Id:           logRow.Id,
		CreatedAt:    logRow.CreatedAt,
		CpaRequestId: logRow.CpaRequestId,
	}, now)

	var stored model.Log
	if err := db.First(&stored, logRow.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CostStatus != "priced" || stored.UpstreamProvider != "bailian" ||
		stored.UpstreamModel != "Bailian/glm-5.3" || stored.ReferenceCostNanos == nil ||
		*stored.ReferenceCostNanos != 2500 || stored.PricingStatus != "priced" {
		t.Fatalf("stored log = %+v", stored)
	}
}

func TestProcessCostRowExpiresOldPending(t *testing.T) {
	db := newCostTestDB(t)
	installCostTestDB(t, db)
	now := time.Now().Unix()
	logRow := model.Log{
		UserId:       1,
		CreatedAt:    now - costLookupTimeout - 1,
		Type:         model.LogTypeConsume,
		ModelName:    "glm-5.3",
		CpaRequestId: "expired-request",
		CostStatus:   "pending",
	}
	if err := db.Create(&logRow).Error; err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"found":false,"request_id":"expired-request"}`))
	}))
	defer server.Close()
	installSidecarEnv(t, server.URL)
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	processCostRow(client, pendingCostRow{
		Id:           logRow.Id,
		CreatedAt:    logRow.CreatedAt,
		CpaRequestId: logRow.CpaRequestId,
	}, now)

	var stored model.Log
	if err := db.First(&stored, logRow.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CostStatus != "missing" {
		t.Fatalf("cost status = %q, want missing", stored.CostStatus)
	}
}
