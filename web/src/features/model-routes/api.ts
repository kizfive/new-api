import { api } from '@/lib/api'

import type {
  ModelCostModel,
  ModelRouteCatalog,
  ModelRouteWrite,
} from './types'

export async function getModelRouteCatalog(): Promise<{
  success: boolean
  message?: string
  data?: ModelRouteCatalog
}> {
  const res = await api.get('/api/model-routes/catalog')
  return res.data
}

export async function getUserModelRoutes(): Promise<{
  success: boolean
  message?: string
  data?: ModelRouteCatalog
}> {
  const res = await api.get('/api/user/model-routes')
  return res.data
}

export async function putUserModelRoutes(
  routes: ModelRouteWrite[]
): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/user/model-routes', { routes })
  return res.data
}

export async function getTokenModelRoutes(tokenId: number): Promise<{
  success: boolean
  message?: string
  data?: ModelRouteCatalog
}> {
  const res = await api.get(`/api/token/${tokenId}/model-routes`)
  return res.data
}

export async function putTokenModelRoutes(
  tokenId: number,
  routes: ModelRouteWrite[]
): Promise<{ success: boolean; message?: string }> {
  const res = await api.put(`/api/token/${tokenId}/model-routes`, { routes })
  return res.data
}

export async function syncModelRoutes(): Promise<{
  success: boolean
  message?: string
  data?: { count: number }
}> {
  const res = await api.post('/api/model-routes/sync')
  return res.data
}

export async function applyModelRoutesToChannel(channelId: number): Promise<{
  success: boolean
  message?: string
  data?: { channel_id: number; count: number; models: string[] }
}> {
  const res = await api.post('/api/model-routes/apply-channel', {
    channel_id: channelId,
  })
  return res.data
}

export async function getModelCostStats(params: {
  startTimestamp?: number
  endTimestamp?: number
  modelName?: string
  provider?: string
}): Promise<{
  success: boolean
  message?: string
  data?: {
    start_timestamp: number
    end_timestamp: number
    models: ModelCostModel[]
  }
}> {
  const query = new URLSearchParams()
  if (params.startTimestamp)
    query.set('start_timestamp', String(params.startTimestamp))
  if (params.endTimestamp)
    query.set('end_timestamp', String(params.endTimestamp))
  if (params.modelName) query.set('model_name', params.modelName)
  if (params.provider) query.set('provider', params.provider)
  const suffix = query.toString()
  const res = await api.get(`/api/log/model-cost${suffix ? `?${suffix}` : ''}`)
  return res.data
}
