import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { ModelRouteEditor } from '@/features/model-routes/components/model-route-editor'

import type { ApiKey } from '../types'

type ModelRouteOverridesDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  token: ApiKey | null
}

export function ModelRouteOverridesDialog(
  props: ModelRouteOverridesDialogProps
) {
  const { t } = useTranslation()
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Model routing')}
      description={
        props.token
          ? t('Key-level overrides for {{name}}', { name: props.token.name })
          : undefined
      }
      contentClassName='sm:max-w-4xl'
    >
      {props.token ? (
        <ModelRouteEditor
          tokenId={props.token.id}
          onSaved={() => props.onOpenChange(false)}
        />
      ) : null}
    </Dialog>
  )
}
