export type ModelRouteProvider = {
  call_point: string
  provider_name: string
  provider_channel: string
  priority: number
}

export type ModelRouteSelection = {
  call_point: string
  fallback_enabled: boolean
  source: string
  inherited: boolean
}

export type ModelRouteModel = {
  root_model: string
  providers: ModelRouteProvider[]
  route: ModelRouteSelection
}

export type ModelRouteCatalog = {
  synced_at: number
  models: ModelRouteModel[]
}

export type ModelRouteMode = 'inherit' | 'auto' | 'fixed'

export type ModelRouteDraft = {
  mode: ModelRouteMode
  callPoint: string
  fallback: boolean
}

export type ModelRouteWrite = {
  root_model: string
  mode: ModelRouteMode
  call_point?: string
  fallback_enabled?: boolean
}

export type ModelCostProvider = {
  provider: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  reference_cost_nanos: number
  pending: number
  priced: number
  unpriced: number
  missing: number
}

export type ModelCostModel = {
  model_name: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  reference_cost_nanos: number
  pending: number
  priced: number
  unpriced: number
  missing: number
  providers: ModelCostProvider[]
}
