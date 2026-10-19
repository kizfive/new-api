import { useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

import { applyModelRoutesToChannel, syncModelRoutes } from './api'
import { ModelRouteEditor } from './components/model-route-editor'

export function ModelRoutes() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = Boolean(user?.role && user.role >= 10)
  const [syncing, setSyncing] = useState(false)
  const [applying, setApplying] = useState(false)
  const [channelId, setChannelId] = useState('')

  const handleSync = async () => {
    setSyncing(true)
    try {
      const result = await syncModelRoutes()
      if (!result.success) {
        handleServerError(result, t('Failed to sync the routing catalog'))
        return
      }
      toast.success(
        t('Catalog synced: {{count}} entries', {
          count: result.data?.count ?? 0,
        })
      )
      await queryClient.invalidateQueries({ queryKey: ['model-routes'] })
    } catch (error) {
      handleServerError(error, t('Failed to sync the routing catalog'))
    } finally {
      setSyncing(false)
    }
  }

  const handleApplyChannel = async () => {
    const id = Number(channelId)
    if (!Number.isInteger(id) || id <= 0) {
      toast.error(t('Enter a valid channel ID'))
      return
    }
    setApplying(true)
    try {
      const result = await applyModelRoutesToChannel(id)
      if (!result.success) {
        handleServerError(
          result,
          t('Failed to apply the catalog to the channel')
        )
        return
      }
      toast.success(
        t('Applied {{count}} root models to channel {{channel}}', {
          count: result.data?.count ?? 0,
          channel: id,
        })
      )
      await queryClient.invalidateQueries({ queryKey: ['model-routes'] })
    } catch (error) {
      handleServerError(error, t('Failed to apply the catalog to the channel'))
    } finally {
      setApplying(false)
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Model routing')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        {isAdmin ? (
          <div className='flex flex-wrap items-center gap-2'>
            <Input
              className='w-28'
              type='number'
              min={1}
              placeholder={t('Channel ID')}
              value={channelId}
              onChange={(event) => setChannelId(event.target.value)}
            />
            <Button
              variant='outline'
              onClick={handleApplyChannel}
              disabled={applying || !channelId}
            >
              {t('Apply roots to channel')}
            </Button>
            <Button variant='outline' onClick={handleSync} disabled={syncing}>
              <RefreshCw
                className={syncing ? 'size-4 animate-spin' : 'size-4'}
              />
              {t('Sync catalog')}
            </Button>
          </div>
        ) : null}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Choose a fixed upstream for each root model, or leave it automatic. Key-level overrides take precedence over these user defaults.'
            )}
          </p>
          <ModelRouteEditor />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
