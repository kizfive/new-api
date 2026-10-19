package modelroute

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCostRetryDelay(t *testing.T) {
	if got := costRetryDelay(0); got != 2*time.Second {
		t.Fatalf("first delay = %s", got)
	}
	if got := costRetryDelay(2); got != 15*time.Second {
		t.Fatalf("third delay = %s", got)
	}
	if got := costRetryDelay(99); got != time.Hour {
		t.Fatalf("capped delay = %s", got)
	}
}

func TestSyncNowPublishesCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/catalog" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		_, _ = writer.Write([]byte(`{"updated_at_ms":1,"providers":[{"id":"p1","name":"Bailian","channel":"openai-compatible-bailian","prefix":"Bailian","priority":10,"models":[{"root_model":"glm-5.3","call_point":"Bailian/glm-5.3"}]}]}`))
	}))
	defer server.Close()

	oldURL, oldToken := os.Getenv("OMC_SIDECAR_URL"), os.Getenv("OMC_SIDECAR_TOKEN")
	os.Setenv("OMC_SIDECAR_URL", server.URL)
	os.Setenv("OMC_SIDECAR_TOKEN", "test-token")
	t.Cleanup(func() {
		os.Setenv("OMC_SIDECAR_URL", oldURL)
		os.Setenv("OMC_SIDECAR_TOKEN", oldToken)
	})

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModelRouteCatalog{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })

	count, err := SyncNow(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	var rows []model.ModelRouteCatalog
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].CallPoint != "Bailian/glm-5.3" || rows[0].ProviderName != "Bailian" {
		t.Fatalf("rows = %+v", rows)
	}
}
