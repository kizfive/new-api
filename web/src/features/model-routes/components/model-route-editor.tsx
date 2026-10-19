import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  getTokenModelRoutes,
  getUserModelRoutes,
  putTokenModelRoutes,
  putUserModelRoutes,
} from '../api'
import type { ModelRouteDraft, ModelRouteWrite } from '../types'

type ModelRouteEditorProps = {
  tokenId?: number
  onSaved?: () => void
  compact?: boolean
}

const DEFAULT_DRAFT: ModelRouteDraft = {
  mode: 'auto',
  callPoint: '',
  fallback: true,
}

export function ModelRouteEditor(props: ModelRouteEditorProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isToken = props.tokenId !== undefined
  const queryKey = ['model-routes', isToken ? props.tokenId : 'user']
  const query = useQuery({
    queryKey,
    queryFn: async () =>
      requireServerSuccess(
        isToken
          ? await getTokenModelRoutes(props.tokenId as number)
          : await getUserModelRoutes()
      ),
    staleTime: 0,
  })
  const catalog = query.data?.data
  const [drafts, setDrafts] = useState<Record<string, ModelRouteDraft>>({})
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!catalog) return
    const next: Record<string, ModelRouteDraft> = {}
    for (const model of catalog.models) {
      const route = model.route
      const mode = route.inherited
        ? 'inherit'
        : route.call_point
          ? 'fixed'
          : 'auto'
      next[model.root_model] = {
        mode,
        callPoint: route.call_point || '',
        fallback: route.fallback_enabled !== false,
      }
    }
    // The query result is the source of truth after a reload; drafts are local edits.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDrafts(next)
  }, [catalog])

  const providerCount = useMemo(
    () =>
      catalog?.models.reduce((sum, model) => sum + model.providers.length, 0) ??
      0,
    [catalog]
  )

  const updateDraft = (rootModel: string, patch: Partial<ModelRouteDraft>) => {
    setDrafts((current) => ({
      ...current,
      [rootModel]: { ...(current[rootModel] ?? DEFAULT_DRAFT), ...patch },
    }))
  }

  const handleSave = async () => {
    if (!catalog) return
    const routes: ModelRouteWrite[] = catalog.models.map((model) => {
      const draft = drafts[model.root_model] ?? DEFAULT_DRAFT
      if (draft.mode === 'inherit') {
        return { root_model: model.root_model, mode: 'inherit' }
      }
      if (draft.mode === 'auto') {
        return {
          root_model: model.root_model,
          mode: 'auto',
          fallback_enabled: draft.fallback,
        }
      }
      return {
        root_model: model.root_model,
        mode: 'fixed',
        call_point: draft.callPoint,
        fallback_enabled: draft.fallback,
      }
    })
    setSaving(true)
    try {
      const result = isToken
        ? await putTokenModelRoutes(props.tokenId as number, routes)
        : await putUserModelRoutes(routes)
      if (!result.success) {
        handleServerError(result, t('Failed to save model routes'))
        return
      }
      toast.success(t('Model routes saved'))
      await queryClient.invalidateQueries({ queryKey })
      props.onSaved?.()
    } catch (error) {
      handleServerError(error, t('Failed to save model routes'))
    } finally {
      setSaving(false)
    }
  }

  if (query.isLoading) {
    return (
      <div className='text-muted-foreground flex items-center gap-2 py-8 text-sm'>
        <Loader2 className='size-4 animate-spin' />
        {t('Loading model routes')}
      </div>
    )
  }

  if (query.isError) {
    return (
      <div className='text-destructive py-6 text-sm'>
        {t('Failed to load model routes')}
      </div>
    )
  }

  if (!catalog || catalog.models.length === 0 || providerCount === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border px-4 py-6 text-sm'>
        {t(
          'No routing catalog yet. Check the sidecar connection and run a catalog sync.'
        )}
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-4'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Root model')}</TableHead>
            <TableHead>{t('Mode')}</TableHead>
            <TableHead>{t('Upstream')}</TableHead>
            <TableHead>{t('Fallback')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {catalog.models.map((model) => {
            const draft = drafts[model.root_model] ?? DEFAULT_DRAFT
            const selectedProvider = model.providers.find(
              (provider) => provider.call_point === draft.callPoint
            )
            const stale =
              draft.mode === 'fixed' &&
              draft.callPoint !== '' &&
              !selectedProvider
            return (
              <TableRow key={model.root_model}>
                <TableCell className='font-mono text-xs'>
                  <div className='flex flex-col gap-0.5'>
                    <span>{model.root_model}</span>
                    <span className='text-muted-foreground text-[11px]'>
                      {model.providers.length} {t('providers')}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <NativeSelect
                    value={draft.mode}
                    onChange={(event) => {
                      const mode = event.target.value as ModelRouteDraft['mode']
                      const firstProvider = model.providers[0]?.call_point ?? ''
                      updateDraft(model.root_model, {
                        mode,
                        callPoint:
                          mode === 'fixed'
                            ? draft.callPoint || firstProvider
                            : '',
                      })
                    }}
                  >
                    {isToken ? (
                      <NativeSelectOption value='inherit'>
                        {t('Inherit user setting')}
                      </NativeSelectOption>
                    ) : null}
                    <NativeSelectOption value='auto'>
                      {t('Automatic')}
                    </NativeSelectOption>
                    <NativeSelectOption value='fixed'>
                      {t('Fixed upstream')}
                    </NativeSelectOption>
                  </NativeSelect>
                </TableCell>
                <TableCell>
                  {draft.mode === 'fixed' ? (
                    <div className='flex flex-col gap-1'>
                      <NativeSelect
                        value={draft.callPoint}
                        onChange={(event) =>
                          updateDraft(model.root_model, {
                            callPoint: event.target.value,
                          })
                        }
                      >
                        {model.providers.map((provider) => (
                          <NativeSelectOption
                            key={provider.call_point}
                            value={provider.call_point}
                          >
                            {provider.provider_name ||
                              provider.provider_channel ||
                              provider.call_point}{' '}
                            · {provider.call_point}
                          </NativeSelectOption>
                        ))}
                      </NativeSelect>
                      {stale ? (
                        <span className='text-destructive text-[11px]'>
                          {t(
                            'This upstream is no longer in the catalog; the request falls back to automatic.'
                          )}
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    <span className='text-muted-foreground text-xs'>—</span>
                  )}
                </TableCell>
                <TableCell>
                  <Switch
                    checked={draft.fallback}
                    disabled={draft.mode !== 'fixed'}
                    onCheckedChange={(checked) =>
                      updateDraft(model.root_model, { fallback: checked })
                    }
                    aria-label={t('Fallback to automatic')}
                  />
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
      <div className='flex items-center justify-between gap-3'>
        <span className='text-muted-foreground text-xs'>
          {t('Reference costs are informational; they do not change quota.')}
        </span>
        <Button onClick={handleSave} disabled={saving}>
          {saving ? <Loader2 className='size-4 animate-spin' /> : null}
          {t('Save')}
        </Button>
      </div>
    </div>
  )
}
