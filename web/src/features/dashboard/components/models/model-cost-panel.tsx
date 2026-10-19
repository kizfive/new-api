import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getModelCostStats } from '@/features/model-routes/api'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { requireServerSuccess } from '@/lib/server-error-message'

import type { DashboardFilters } from '../../types'

type ModelCostPanelProps = {
  filters?: DashboardFilters
}

function unixSeconds(value?: Date): number | undefined {
  return value ? Math.floor(value.getTime() / 1000) : undefined
}

function formatTokens(value: number): string {
  return value.toLocaleString()
}

export function ModelCostPanel(props: ModelCostPanelProps) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: [
      'model-cost',
      props.filters?.start_timestamp?.toISOString(),
      props.filters?.end_timestamp?.toISOString(),
    ],
    queryFn: async () =>
      requireServerSuccess(
        await getModelCostStats({
          startTimestamp: unixSeconds(props.filters?.start_timestamp),
          endTimestamp: unixSeconds(props.filters?.end_timestamp),
        })
      ),
    staleTime: 30 * 1000,
  })

  const models = query.data?.data?.models ?? []
  if (query.isLoading) {
    return (
      <div className='text-muted-foreground flex items-center gap-2 rounded-lg border px-4 py-6 text-sm'>
        <Loader2 className='size-4 animate-spin' />
        {t('Loading model costs')}
      </div>
    )
  }
  if (models.length === 0) {
    return null
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center justify-between gap-2 px-4 py-2.5 sm:px-5 sm:py-3'>
        <div className='text-xs font-semibold'>{t('Model cost')}</div>
        <div className='text-muted-foreground text-[11px]'>
          {t('Reference cost only; quota is unchanged.')}
        </div>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Model / provider')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Tokens')}</TableHead>
            <TableHead className='text-right'>{t('Reference cost')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {models.map((model) => (
            <ModelRows key={model.model_name} model={model} />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function ModelRows(props: {
  model: {
    model_name: string
    requests: number
    total_tokens: number
    reference_cost_nanos: number
    pending: number
    missing: number
    providers: {
      provider: string
      requests: number
      total_tokens: number
      reference_cost_nanos: number
    }[]
  }
}) {
  const { t } = useTranslation()
  const model = props.model
  return (
    <>
      <TableRow className='bg-muted/20'>
        <TableCell className='font-mono text-xs font-semibold'>
          {model.model_name}
        </TableCell>
        <TableCell className='text-right text-xs tabular-nums'>
          {model.requests.toLocaleString()}
        </TableCell>
        <TableCell className='text-right text-xs tabular-nums'>
          {formatTokens(model.total_tokens)}
        </TableCell>
        <TableCell className='text-right text-xs tabular-nums'>
          {formatBillingCurrencyFromUSD(model.reference_cost_nanos / 1e9)}
          {model.pending + model.missing > 0 ? (
            <span className='text-muted-foreground ml-1'>
              ({model.pending + model.missing} {t('pending')})
            </span>
          ) : null}
        </TableCell>
      </TableRow>
      {model.providers.map((provider) => (
        <TableRow key={model.model_name + '|' + provider.provider}>
          <TableCell className='text-muted-foreground pl-6 font-mono text-xs'>
            {provider.provider || '—'}
          </TableCell>
          <TableCell className='text-right text-xs tabular-nums'>
            {provider.requests.toLocaleString()}
          </TableCell>
          <TableCell className='text-right text-xs tabular-nums'>
            {formatTokens(provider.total_tokens)}
          </TableCell>
          <TableCell className='text-right text-xs tabular-nums'>
            {formatBillingCurrencyFromUSD(provider.reference_cost_nanos / 1e9)}
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}
