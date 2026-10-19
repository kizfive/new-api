import { createFileRoute } from '@tanstack/react-router'

import { ModelRoutes } from '@/features/model-routes'

export const Route = createFileRoute('/_authenticated/model-routes/')({
  component: ModelRoutes,
})
