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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Save, Search } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import { cn } from '@/lib/utils'

import {
  getTokenModelRoutes,
  getUserModelRoutes,
  putTokenModelRoutes,
  putUserModelRoutes,
} from '../api'
import type {
  ModelRouteDraft,
  ModelRouteModel,
  ModelRouteWrite,
} from '../types'

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

function matchesSearch(model: ModelRouteModel, search: string) {
  if (!search) return true
  const haystack = [
    model.root_model,
    ...model.providers.flatMap((provider) => [
      provider.provider_name,
      provider.provider_channel,
      provider.call_point,
    ]),
  ]
  return haystack.some((value) => value.toLowerCase().includes(search))
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
  const [search, setSearch] = useState('')

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

  const normalizedSearch = search.trim().toLowerCase()
  const visibleModels = useMemo(
    () =>
      catalog?.models.filter((model) =>
        matchesSearch(model, normalizedSearch)
      ) ?? [],
    [catalog, normalizedSearch]
  )

  const modeOptions = useMemo(
    () => [
      ...(isToken
        ? [{ value: 'inherit' as const, label: t('Inherit user setting') }]
        : []),
      { value: 'auto' as const, label: t('Automatic') },
      { value: 'fixed' as const, label: t('Fixed upstream') },
    ],
    [isToken, t]
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
    return <LoadingState message={t('Loading model routes')} />
  }

  if (query.isError) {
    return (
      <ErrorState
        title={t('Failed to load model routes')}
        description={
          query.error instanceof Error ? query.error.message : undefined
        }
        onRetry={() => void query.refetch()}
      />
    )
  }

  if (!catalog || catalog.models.length === 0) {
    return (
      <EmptyState
        title={t('No models available')}
        description={t('Only models available to your account are shown.')}
        bordered
      />
    )
  }

  const noSearchResults = normalizedSearch !== '' && visibleModels.length === 0

  return (
    <div
      className={cn('flex min-w-0 flex-col', props.compact ? 'gap-3' : 'gap-4')}
    >
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='relative w-full sm:max-w-sm'>
          <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2' />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Search models')}
            aria-label={t('Search models')}
            className='pl-9'
          />
        </div>
        <span className='text-muted-foreground text-xs tabular-nums'>
          {t('{{count}} models', { count: visibleModels.length })}
        </span>
      </div>

      {noSearchResults ? (
        <EmptyState
          title={t('No models match your search')}
          description={t('Try a different model or provider name.')}
          bordered
          className='min-h-[220px]'
        />
      ) : (
        <div className='overflow-hidden rounded-xl border'>
          <Table className='min-w-[720px] table-fixed'>
            <TableHeader className='bg-muted/40'>
              <TableRow>
                <TableHead className='w-[28%] px-3'>
                  {t('Root model')}
                </TableHead>
                <TableHead className='w-40 px-3'>{t('Mode')}</TableHead>
                <TableHead className='px-3'>{t('Upstream')}</TableHead>
                <TableHead className='w-24 px-3'>{t('Fallback')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleModels.map((model) => {
                const draft = drafts[model.root_model] ?? DEFAULT_DRAFT
                const selectedProvider = model.providers.find(
                  (provider) => provider.call_point === draft.callPoint
                )
                const stale =
                  draft.mode === 'fixed' &&
                  draft.callPoint !== '' &&
                  !selectedProvider
                return (
                  <TableRow key={model.root_model} className='h-[4.25rem]'>
                    <TableCell className='px-3 py-2'>
                      <div className='flex min-w-0 flex-col gap-1'>
                        <span className='truncate font-mono text-xs font-semibold'>
                          {model.root_model}
                        </span>
                        <Badge
                          variant='secondary'
                          className='w-fit font-normal'
                        >
                          {t('{{count}} upstreams', {
                            count: model.providers.length,
                          })}
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell className='px-3 py-2'>
                      <Select
                        value={draft.mode}
                        items={modeOptions}
                        onValueChange={(value) => {
                          if (!value) return
                          const mode = value as ModelRouteDraft['mode']
                          const firstProvider =
                            model.providers[0]?.call_point ?? ''
                          updateDraft(model.root_model, {
                            mode,
                            callPoint:
                              mode === 'fixed'
                                ? draft.callPoint || firstProvider
                                : '',
                          })
                        }}
                      >
                        <SelectTrigger
                          size='sm'
                          className='w-36'
                          aria-label={t('Mode')}
                        >
                          <SelectValue>
                            {modeOptions.find(
                              (option) => option.value === draft.mode
                            )?.label ?? t('Automatic')}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          {modeOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell className='px-3 py-2'>
                      {draft.mode === 'fixed' ? (
                        <div className='flex min-w-0 flex-col gap-1'>
                          <Select
                            value={draft.callPoint}
                            items={model.providers.map((provider) => ({
                              value: provider.call_point,
                              label: provider.provider_name
                                ? `${provider.provider_name} · ${provider.call_point}`
                                : provider.call_point,
                            }))}
                            onValueChange={(value) => {
                              if (!value) return
                              updateDraft(model.root_model, {
                                callPoint: value,
                              })
                            }}
                          >
                            <SelectTrigger
                              size='sm'
                              className='w-full max-w-[22rem]'
                              aria-label={t('Upstream')}
                            >
                              <SelectValue>
                                <span className='truncate'>
                                  {selectedProvider?.provider_name ||
                                    (stale ? t('Unavailable') : t('Upstream'))}
                                </span>
                              </SelectValue>
                            </SelectTrigger>
                            <SelectContent
                              align='start'
                              alignItemWithTrigger={false}
                              className='min-w-[18rem]'
                            >
                              {model.providers.map((provider) => (
                                <SelectItem
                                  key={provider.call_point}
                                  value={provider.call_point}
                                >
                                  <span className='flex min-w-0 flex-col gap-0.5 py-0.5'>
                                    <span className='truncate font-medium'>
                                      {provider.provider_name ||
                                        provider.provider_channel ||
                                        provider.call_point}
                                    </span>
                                    <span className='text-muted-foreground truncate font-mono text-[11px]'>
                                      {provider.call_point}
                                    </span>
                                  </span>
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          {selectedProvider ? (
                            <span className='text-muted-foreground truncate font-mono text-[11px]'>
                              {selectedProvider.call_point}
                            </span>
                          ) : null}
                          {stale ? (
                            <span className='text-destructive text-[11px]'>
                              {t(
                                'This upstream is no longer in the catalog; the request falls back to automatic.'
                              )}
                            </span>
                          ) : null}
                        </div>
                      ) : (
                        <div className='flex h-full items-center'>
                          <span className='text-muted-foreground text-sm'>
                            —
                          </span>
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='px-3 py-2'>
                      <Switch
                        checked={draft.fallback}
                        disabled={draft.mode !== 'fixed'}
                        onCheckedChange={(checked) =>
                          updateDraft(model.root_model, { fallback: checked })
                        }
                        aria-label={t('Fallback')}
                        title={t('Fallback')}
                      />
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <div className='flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
        <p className='text-muted-foreground max-w-2xl text-xs'>
          {t('Reference costs are informational; they do not change quota.')}
        </p>
        <Button
          onClick={() => void handleSave()}
          disabled={saving || visibleModels.length === 0}
        >
          {saving ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <Save className='size-4' />
          )}
          {t('Save')}
        </Button>
      </div>
    </div>
  )
}
