/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { ModelRouteEditor } = await import('../model-route-editor')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  put: ApiMethod
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get

afterEach(() => {
  apiClient.get = originalGet
})

function renderEditor() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ModelRouteEditor />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

describe('ModelRouteEditor', () => {
  test('searches the route catalog by model and provider', async () => {
    apiClient.get = async (url) => {
      if (url === '/api/user/model-routes') {
        return {
          data: {
            success: true,
            data: {
              synced_at: 1,
              models: [
                {
                  root_model: 'glm-5.3',
                  providers: [
                    {
                      call_point: 'Bailian/glm-5.3',
                      provider_name: 'Bailian',
                      provider_channel: 'bailian',
                      priority: 1,
                    },
                  ],
                  route: {
                    call_point: '',
                    fallback_enabled: true,
                    source: 'auto',
                    inherited: false,
                  },
                },
              ],
            },
          },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    }

    renderEditor()

    expect(await screen.findByText('glm-5.3')).toBeVisible()
    const search = screen.getByRole('textbox', { name: 'Search models' })

    fireEvent.change(search, { target: { value: 'kimi' } })
    await waitFor(() => {
      expect(screen.queryByText('glm-5.3')).not.toBeInTheDocument()
      expect(screen.getByText('No models match your search')).toBeVisible()
    })

    fireEvent.change(search, { target: { value: 'bailian' } })
    await waitFor(() => {
      expect(screen.getByText('glm-5.3')).toBeVisible()
    })
  })
})
