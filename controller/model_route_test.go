package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func boolPointer(value bool) *bool { return &value }

func TestModelRouteWritesValidatesCatalogAndModes(t *testing.T) {
	catalog := []model.ModelRouteCatalog{
		{RootModel: "glm-5.3", CallPoint: "Bailian/glm-5.3"},
		{RootModel: "glm-5.3", CallPoint: "ZHIPU/GLM-5.3"},
	}
	writes, err := modelRouteWrites(modelRouteWriteBody{Routes: []modelRouteWriteRequest{
		{RootModel: "glm-5.3", Mode: "auto", FallbackEnabled: boolPointer(false)},
	}}, catalog, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || writes[0].CallPoint == nil || *writes[0].CallPoint != "" || writes[0].FallbackEnabled {
		t.Fatalf("auto write = %+v", writes)
	}

	writes, err = modelRouteWrites(modelRouteWriteBody{Routes: []modelRouteWriteRequest{
		{RootModel: "glm-5.3", Mode: "fixed", CallPoint: "Bailian/glm-5.3"},
	}}, catalog, false)
	if err != nil || len(writes) != 1 || writes[0].CallPoint == nil || *writes[0].CallPoint != "Bailian/glm-5.3" {
		t.Fatalf("fixed write = %+v, %v", writes, err)
	}

	if _, err := modelRouteWrites(modelRouteWriteBody{Routes: []modelRouteWriteRequest{
		{RootModel: "glm-5.3", Mode: "fixed", CallPoint: "Huawei/glm-5.3"},
	}}, catalog, false); err == nil {
		t.Fatal("unknown call point was accepted")
	}
	if _, err := modelRouteWrites(modelRouteWriteBody{Routes: []modelRouteWriteRequest{
		{RootModel: "glm-5.3", Mode: "inherit"},
	}}, catalog, false); err == nil {
		t.Fatal("user inherit was accepted")
	}
	if _, err := modelRouteWrites(modelRouteWriteBody{Routes: []modelRouteWriteRequest{
		{RootModel: "glm-5.3", Mode: "auto"},
		{RootModel: "glm-5.3", Mode: "auto"},
	}}, catalog, false); err == nil {
		t.Fatal("duplicate root model was accepted")
	}
}

func TestBuildModelRouteCatalogViewPrecedence(t *testing.T) {
	rows := []model.ModelRouteCatalog{
		{RootModel: "glm-5.3", CallPoint: "Bailian/glm-5.3", ProviderName: "Bailian", SyncedAt: 10},
		{RootModel: "glm-5.3", CallPoint: "ZHIPU/GLM-5.3", ProviderName: "ZHIPU", SyncedAt: 10},
	}
	view := buildModelRouteCatalogView(rows,
		[]model.UserModelRoute{{RootModel: "glm-5.3", CallPoint: "Bailian/glm-5.3", FallbackEnabled: true}},
		nil, false)
	if len(view.Models) != 1 || view.Models[0].Route.Source != "user" || view.Models[0].Route.CallPoint != "Bailian/glm-5.3" {
		t.Fatalf("user view = %+v", view)
	}
	if len(view.Models[0].Providers) != 2 {
		t.Fatalf("providers = %+v", view.Models[0].Providers)
	}

	view = buildModelRouteCatalogView(rows,
		[]model.UserModelRoute{{RootModel: "glm-5.3", CallPoint: "Bailian/glm-5.3", FallbackEnabled: true}},
		[]model.TokenModelRoute{{RootModel: "glm-5.3", CallPoint: "", FallbackEnabled: false}}, true)
	if view.Models[0].Route.Source != "token" || view.Models[0].Route.CallPoint != "" || view.Models[0].Route.Inherited {
		t.Fatalf("token view = %+v", view.Models[0].Route)
	}
}
