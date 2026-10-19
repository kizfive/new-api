package model

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newModelRouteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ModelRouteCatalog{}, &UserModelRoute{}, &TokenModelRoute{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func installModelRouteTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldDB := DB
	DB = db
	modelRouteCache = sync.Map{}
	modelRouteCatalog = atomic.Value{}
	modelRouteRevision.Store(0)
	t.Cleanup(func() {
		DB = oldDB
		modelRouteCache = sync.Map{}
		modelRouteCatalog = atomic.Value{}
		modelRouteRevision.Store(0)
	})
}

func TestResolveModelRoutePrecedence(t *testing.T) {
	db := newModelRouteTestDB(t)
	installModelRouteTestDB(t, db)

	if got := ResolveModelRoute(1, 10, "glm-5.3"); got.Source != "auto" {
		t.Fatalf("empty routes: source = %q, want auto", got.Source)
	}

	if err := db.Create(&UserModelRoute{UserId: 1, RootModel: "glm-5.3", CallPoint: "Bailian/glm-5.3", FallbackEnabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	InvalidateModelRouteCache()
	if got := ResolveModelRoute(1, 10, "glm-5.3"); got.Source != "user" || got.CallPoint != "Bailian/glm-5.3" {
		t.Fatalf("user route = %+v", got)
	}

	if err := db.Create(&TokenModelRoute{TokenId: 10, UserId: 1, RootModel: "glm-5.3", CallPoint: "ZHIPU/GLM-5.3", FallbackEnabled: false}).Error; err != nil {
		t.Fatal(err)
	}
	InvalidateModelRouteCache()
	if got := ResolveModelRoute(1, 10, "glm-5.3"); got.Source != "token" || got.CallPoint != "ZHIPU/GLM-5.3" || got.FallbackEnabled {
		t.Fatalf("token route = %+v", got)
	}

	if err := db.Where("token_id = ?", 10).Delete(&TokenModelRoute{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&TokenModelRoute{TokenId: 10, UserId: 1, RootModel: "glm-5.3", CallPoint: "", FallbackEnabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	InvalidateModelRouteCache()
	if got := ResolveModelRoute(1, 10, "glm-5.3"); got.Source != "token" || got.CallPoint != "" {
		t.Fatalf("explicit token auto = %+v", got)
	}
}

func TestModelRouteCatalogCacheAndReplacement(t *testing.T) {
	db := newModelRouteTestDB(t)
	installModelRouteTestDB(t, db)
	if _, err := ReplaceModelRouteCatalog(context.Background(), nil); err == nil {
		t.Fatal("empty catalog was accepted")
	}
	rows := []ModelRouteCatalog{
		{RootModel: "glm-5.3", CallPoint: "Bailian/glm-5.3", ProviderName: "Bailian"},
		{RootModel: "glm-5.3", CallPoint: "ZHIPU/GLM-5.3", ProviderName: "ZHIPU"},
	}
	if count, err := ReplaceModelRouteCatalog(context.Background(), rows); err != nil || count != 2 {
		t.Fatalf("replace = %d, %v", count, err)
	}
	if !ModelRouteCallPointExists("glm-5.3", "Bailian/glm-5.3") {
		t.Fatal("catalog cache missed Bailian")
	}
	if ModelRouteCallPointExists("glm-5.3", "Huawei/glm-5.3") {
		t.Fatal("catalog cache reported a missing call point")
	}
}
