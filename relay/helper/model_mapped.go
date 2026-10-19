package helper

import (
	"errors"
	"fmt"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	hostreasoning "github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &relaycommon.ChannelMeta{}
	}

	defer func() {
		if info.IsModelMapped {
			c.Set("model_route_upstream_model", info.UpstreamModelName)
		}
	}()

	if c.GetBool("model_route_fallback") {
		c.Set("model_route_source", "fallback")
		c.Set("model_route_fallback_used", true)
		c.Set("model_route_upstream_model", info.OriginModelName)
		info.UpstreamModelName = info.OriginModelName
		if request != nil {
			request.SetModelName(info.OriginModelName)
		}
		return nil
	}

	if info.UserId > 0 || info.TokenId > 0 {
		preference := model.ResolveModelRoute(info.UserId, info.TokenId, info.OriginModelName)
		if preference.Source != "auto" {
			c.Set("model_route_source", preference.Source)
			c.Set("model_route_fallback_enabled", preference.FallbackEnabled)
			if preference.CallPoint != "" {
				if model.ModelRouteCallPointExists(info.OriginModelName, preference.CallPoint) {
					info.UpstreamModelName = preference.CallPoint
					info.IsModelMapped = true
					c.Set("model_route_pinned", true)
					c.Set("model_route_upstream_model", preference.CallPoint)
					if request != nil {
						request.SetModelName(preference.CallPoint)
					}
					return nil
				}
				c.Set("model_route_stale", true)
				c.Set("model_route_source", "stale")
			}
		}
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := rootcommon.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := info.OriginModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			mappedModel, exists := modelMap[currentModel]
			baseModel := hostreasoning.BaseModelName(currentModel)
			if (!exists || mappedModel == "") && baseModel != currentModel {
				mappedModel, exists = modelMap[baseModel]
			}
			if exists && mappedModel != "" {
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							return nil
						}

						info.IsModelMapped = true
						break
					}
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	}

	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}
