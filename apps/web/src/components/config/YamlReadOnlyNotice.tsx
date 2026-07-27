import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Link } from '@tanstack/react-router'
import { FileWarning } from 'lucide-react'

interface YamlReadOnlyNoticeProps {
  /** Plural, lower-case name of what the page manages, e.g. "targets". */
  resource: string
}

/**
 * Explains why Edit and Delete are disabled on a config page.
 *
 * When the daemon reads its configuration from YAML, the file is the source of
 * truth and the UI cannot write to it. Without this the buttons just look
 * broken — a `title` attribute is not a discoverable explanation.
 */
export function YamlReadOnlyNotice({ resource }: YamlReadOnlyNoticeProps) {
  return (
    <Alert>
      <FileWarning className="h-4 w-4" />
      <AlertTitle>These {resource} come from YAML and are read-only here</AlertTitle>
      <AlertDescription>
        <p>
          The daemon is running with a YAML configuration file as its source of truth, so {resource}{' '}
          cannot be edited or deleted from the dashboard. Edit the YAML file and restart the daemon,
          or move the configuration into the database to manage it here.
        </p>
        <p className="mt-2 flex flex-wrap gap-x-4 gap-y-1">
          <Link to="/config" className="font-medium underline underline-offset-4">
            Migrate this configuration to the database
          </Link>
          <Link to="/config/import" className="font-medium underline underline-offset-4">
            Import YAML into the database
          </Link>
        </p>
      </AlertDescription>
    </Alert>
  )
}
