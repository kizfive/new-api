package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/modelroute"

	"github.com/gin-gonic/gin"
)

type modelRouteProviderView struct {
	CallPoint       string `json:"call_point"`
	ProviderName    string `json:"provider_name"`
	ProviderChannel string `json:"provider_channel"`
	Priority        int    `json:"priority"`
}

type modelRouteSelection struct {
	CallPoint       string `json:"call_point"`
	FallbackEnabled bool   `json:"fallback_enabled"`
	Source          string `json:"source"`
	Inherited       bool   `json:"inherited"`
}

type modelRouteModelView struct {
	RootModel string                   `json:"root_model"`
	Providers []modelRouteProviderView `json:"providers"`
	Route     modelRouteSelection      `json:"route"`
}

type modelRouteCatalogView struct {
	SyncedAt int64                 `json:"synced_at"`
	Models   []modelRouteModelView `json:"models"`
}

func buildModelRouteCatalogView(rows []model.ModelRouteCatalog, userRoutes []model.UserModelRoute, tokenRoutes []model.TokenModelRoute, forToken bool) modelRouteCatalogView {
	userByRoot := make(map[string]model.UserModelRoute, len(userRoutes))
	for _, route := range userRoutes {
		userByRoot[route.RootModel] = route
	}
	tokenByRoot := make(map[string]model.TokenModelRoute, len(tokenRoutes))
	for _, route := range tokenRoutes {
		tokenByRoot[route.RootModel] = route
	}
	byRoot := make(map[string]*modelRouteModelView)
	order := make([]string, 0)
	syncedAt := int64(0)
	for _, row := range rows {
		view := byRoot[row.RootModel]
		if view == nil {
			view = &modelRouteModelView{
				RootModel: row.RootModel,
				Route:     modelRouteSelection{FallbackEnabled: true, Source: "auto"},
			}
			byRoot[row.RootModel] = view
			order = append(order, row.RootModel)
		}
		if row.SyncedAt > syncedAt {
			syncedAt = row.SyncedAt
		}
		view.Providers = append(view.Providers, modelRouteProviderView{
			CallPoint:       row.CallPoint,
			ProviderName:    row.ProviderName,
			ProviderChannel: row.ProviderChannel,
			Priority:        row.Priority,
		})
	}
	for _, root := range order {
		view := byRoot[root]
		if tokenRoute, ok := tokenByRoot[root]; ok {
			view.Route = modelRouteSelection{
				CallPoint:       tokenRoute.CallPoint,
				FallbackEnabled: tokenRoute.FallbackEnabled,
				Source:          "token",
			}
		} else if userRoute, ok := userByRoot[root]; ok {
			view.Route = modelRouteSelection{
				CallPoint:       userRoute.CallPoint,
				FallbackEnabled: userRoute.FallbackEnabled,
				Source:          "user",
				Inherited:       forToken,
			}
		}
	}
	models := make([]modelRouteModelView, 0, len(order))
	for _, root := range order {
		models = append(models, *byRoot[root])
	}
	return modelRouteCatalogView{SyncedAt: syncedAt, Models: models}
}

func loadModelRouteCatalog(c *gin.Context) ([]model.ModelRouteCatalog, bool) {
	rows, err := model.ListModelRouteCatalog(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	return rows, true
}

func GetUserModelRoutes(c *gin.Context) {
	rows, ok := loadModelRouteCatalog(c)
	if !ok {
		return
	}
	userRoutes, err := model.ListUserModelRoutes(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildModelRouteCatalogView(rows, userRoutes, nil, false))
}

type modelRouteWriteRequest struct {
	RootModel       string `json:"root_model"`
	Mode            string `json:"mode"`
	CallPoint       string `json:"call_point"`
	FallbackEnabled *bool  `json:"fallback_enabled"`
}

type modelRouteWriteBody struct {
	Routes []modelRouteWriteRequest `json:"routes"`
}

func modelRouteWrites(body modelRouteWriteBody, catalog []model.ModelRouteCatalog, allowInherit bool) ([]model.ModelRouteWrite, error) {
	valid := make(map[string]map[string]bool)
	for _, row := range catalog {
		if valid[row.RootModel] == nil {
			valid[row.RootModel] = make(map[string]bool)
		}
		valid[row.RootModel][row.CallPoint] = true
	}
	writes := make([]model.ModelRouteWrite, 0, len(body.Routes))
	seen := make(map[string]bool)
	for _, route := range body.Routes {
		rootModel := strings.TrimSpace(route.RootModel)
		if rootModel == "" {
			return nil, errors.New("root_model is required")
		}
		if seen[rootModel] {
			return nil, errors.New("duplicate root_model: " + rootModel)
		}
		seen[rootModel] = true
		mode := strings.ToLower(strings.TrimSpace(route.Mode))
		if mode == "" {
			if strings.TrimSpace(route.CallPoint) == "" {
				mode = "auto"
			} else {
				mode = "fixed"
			}
		}
		fallback := true
		if route.FallbackEnabled != nil {
			fallback = *route.FallbackEnabled
		}
		switch mode {
		case "inherit":
			if !allowInherit {
				return nil, errors.New("inherit is only valid for token routes")
			}
			writes = append(writes, model.ModelRouteWrite{RootModel: rootModel, CallPoint: nil, FallbackEnabled: fallback})
		case "auto":
			empty := ""
			writes = append(writes, model.ModelRouteWrite{RootModel: rootModel, CallPoint: &empty, FallbackEnabled: fallback})
		case "fixed":
			callPoint := strings.TrimSpace(route.CallPoint)
			if callPoint == "" {
				return nil, errors.New("call_point is required for fixed mode")
			}
			if !valid[rootModel][callPoint] {
				return nil, errors.New("call_point is not in the current catalog: " + callPoint)
			}
			value := callPoint
			writes = append(writes, model.ModelRouteWrite{RootModel: rootModel, CallPoint: &value, FallbackEnabled: fallback})
		default:
			return nil, errors.New("mode must be inherit, auto, or fixed")
		}
	}
	return writes, nil
}

func PutUserModelRoutes(c *gin.Context) {
	rows, ok := loadModelRouteCatalog(c)
	if !ok {
		return
	}
	var body modelRouteWriteBody
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiError(c, err)
		return
	}
	writes, err := modelRouteWrites(body, rows, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveUserModelRoutes(c.Request.Context(), c.GetInt("id"), writes); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"count": len(writes)})
}

func ownedToken(c *gin.Context, id int) (*model.Token, bool) {
	token, err := model.GetTokenById(id)
	if err != nil || token == nil {
		common.ApiErrorStatus(c, http.StatusNotFound, errors.New("token not found"))
		return nil, false
	}
	if token.UserId != c.GetInt("id") && c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorStatus(c, http.StatusForbidden, errors.New("token does not belong to the current user"))
		return nil, false
	}
	return token, true
}

func GetTokenModelRoutes(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid token id"))
		return
	}
	token, ok := ownedToken(c, id)
	if !ok {
		return
	}
	rows, ok := loadModelRouteCatalog(c)
	if !ok {
		return
	}
	userRoutes, err := model.ListUserModelRoutes(c.Request.Context(), token.UserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tokenRoutes, err := model.ListTokenModelRoutes(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildModelRouteCatalogView(rows, userRoutes, tokenRoutes, true))
}

func PutTokenModelRoutes(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid token id"))
		return
	}
	token, ok := ownedToken(c, id)
	if !ok {
		return
	}
	rows, ok := loadModelRouteCatalog(c)
	if !ok {
		return
	}
	var body modelRouteWriteBody
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiError(c, err)
		return
	}
	writes, err := modelRouteWrites(body, rows, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveTokenModelRoutes(c.Request.Context(), id, token.UserId, writes); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"count": len(writes)})
}

func GetModelRouteCatalog(c *gin.Context) {
	rows, ok := loadModelRouteCatalog(c)
	if !ok {
		return
	}
	common.ApiSuccess(c, buildModelRouteCatalogView(rows, nil, nil, false))
}

func SyncModelRoutes(c *gin.Context) {
	count, err := modelroute.SyncNow(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"count": count})
}

// ApplyModelRouteChannel publishes the catalog's root models onto the CPA
// channel's client-visible model list. It never adds a prefixed call point:
// those belong in the routing table, not in /v1/models.
func ApplyModelRouteChannel(c *gin.Context) {
	var body map[string]int
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiError(c, err)
		return
	}
	channelID := body["channel_id"]
	if channelID <= 0 {
		common.ApiError(c, errors.New("channel_id is required"))
		return
	}
	rows, err := model.ListModelRouteCatalog(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	roots := model.ModelRouteCatalogRoots(rows)
	if len(roots) == 0 {
		common.ApiError(c, errors.New("routing catalog is empty; sync it first"))
		return
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel.Models = strings.Join(roots, ",")
	if err := channel.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"channel_id": channel.Id,
		"count":      len(roots),
		"models":     roots,
	})
}

func GetModelCostStats(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if endTimestamp == 0 {
		endTimestamp = common.GetTimestamp()
	}
	if startTimestamp == 0 {
		startTimestamp = endTimestamp - 7*24*60*60
	}
	if c.GetInt("role") < common.RoleAdminUser {
		var err error
		startTimestamp, err = limitLogTimeRange(startTimestamp, endTimestamp)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	filter := model.ModelCostFilter{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ModelName:      c.Query("model_name"),
		Provider:       c.Query("provider"),
		TokenName:      c.Query("token_name"),
		Group:          c.Query("group"),
	}
	if c.GetInt("role") < common.RoleAdminUser {
		filter.UserId = c.GetInt("id")
	}
	models, err := model.QueryModelCostStats(c.Request.Context(), filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"start_timestamp": startTimestamp,
		"end_timestamp":   endTimestamp,
		"models":          models,
	})
}
