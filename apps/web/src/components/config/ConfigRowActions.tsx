import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Pencil, Trash2 } from 'lucide-react'

interface ConfigRowActionsProps {
  /** Singular, lower-case name of the row's resource, e.g. "target". */
  resource: string
  /** The row's name, used to make the accessible labels unambiguous. */
  name: string
  /** True when the config is YAML-sourced and therefore not writable here. */
  readOnly: boolean
  /** True while a delete is in flight. */
  deletePending?: boolean
  onEdit: () => void
  onDelete: () => void
}

/**
 * The Edit/Delete cell shared by the four config list pages.
 *
 * The buttons are icon-only, so they carry an `aria-label`, and each is wrapped
 * in a tooltip that states why it is disabled. A disabled button fires no
 * pointer events, so the tooltip trigger has to be the `span` around it rather
 * than the button itself.
 */
export function ConfigRowActions({
  resource,
  name,
  readOnly,
  deletePending = false,
  onEdit,
  onDelete,
}: ConfigRowActionsProps) {
  const readOnlyHint = `This ${resource} is defined in YAML — migrate the configuration to the database to change it here`

  return (
    <div className="flex items-center justify-end gap-2">
      <Tooltip>
        <TooltipTrigger asChild>
          <span tabIndex={readOnly ? 0 : undefined}>
            <Button
              variant="ghost"
              size="sm"
              onClick={onEdit}
              disabled={readOnly}
              aria-label={`Edit ${resource} ${name}`}
            >
              <Pencil className="h-4 w-4" />
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent>{readOnly ? readOnlyHint : `Edit ${name}`}</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <span tabIndex={readOnly ? 0 : undefined}>
            <Button
              variant="ghost"
              size="sm"
              onClick={onDelete}
              disabled={readOnly || deletePending}
              aria-label={`Delete ${resource} ${name}`}
            >
              <Trash2 className="h-4 w-4 text-red-500" />
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent>{readOnly ? readOnlyHint : `Delete ${name}`}</TooltipContent>
      </Tooltip>
    </div>
  )
}
