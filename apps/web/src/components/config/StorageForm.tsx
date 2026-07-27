import { useState } from 'react'
import { ShieldAlert } from 'lucide-react'
import { Button } from '../ui/button'
import { Checkbox } from '../ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/dialog'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { PasswordInput } from './PasswordInput'
import type { Storage, StorageConfigRequest, StorageRequest, StorageType } from '../../types'

interface StorageFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  storage?: Storage
  onSubmit: (storage: StorageRequest) => Promise<void>
}

/**
 * Everything the form collects, flat. `config` is assembled per type on submit
 * so the payload only ever carries the keys the API actually reads.
 */
interface StorageFormState {
  name: string
  type: StorageType
  keep: number
  // local + sftp
  path: string
  // s3
  bucket: string
  region: string
  accessKeyId: string
  endpointUrl: string
  secretAccessKey: string
  // sftp
  host: string
  /** Kept as the raw input string; parsed to a number when the payload is built. */
  port: string
  username: string
  knownHostsPath: string
  hostKeyFingerprint: string
  insecureSkipHostKeyVerify: boolean
  privateKeyPath: string
  privateKeyPassphrase: string
  password: string
}

const DEFAULT_SFTP_PORT = 22

/**
 * Secrets come back from the API as `***REDACTED***`; they must never be
 * pre-filled into an input or they would be sent back as a literal password.
 */
function initialState(storage?: Storage): StorageFormState {
  const config = storage?.config ?? {}
  return {
    name: storage?.name ?? '',
    type: storage?.type ?? 'local',
    keep: storage?.keep ?? 7,
    path: config.path ?? '',
    bucket: config.bucket ?? '',
    region: config.region ?? '',
    accessKeyId: config.access_key_id ?? '',
    endpointUrl: config.endpoint_url ?? '',
    secretAccessKey: '',
    host: config.host ?? '',
    port: String(config.port ?? DEFAULT_SFTP_PORT),
    username: config.username ?? '',
    knownHostsPath: config.known_hosts_path ?? '',
    hostKeyFingerprint: config.host_key_fingerprint ?? '',
    insecureSkipHostKeyVerify: config.insecure_skip_host_key_verify ?? false,
    privateKeyPath: config.private_key_path ?? '',
    privateKeyPassphrase: '',
    password: '',
  }
}

/**
 * Mirrors `requestToStorage` in apps/api/internal/api/config_handlers.go. Any
 * key not listed there is dropped by the server, so the shapes must match.
 */
function buildConfig(state: StorageFormState): StorageConfigRequest {
  switch (state.type) {
    case 'local':
      return { path: state.path }
    case 's3':
      return {
        bucket: state.bucket,
        region: state.region,
        access_key_id: state.accessKeyId,
        endpoint_url: state.endpointUrl,
      }
    case 'sftp':
      return {
        host: state.host,
        // The API decodes this as a JSON number (`.(float64)`) and drops a string.
        port: parseInt(state.port, 10) || DEFAULT_SFTP_PORT,
        username: state.username,
        path: state.path,
        known_hosts_path: state.knownHostsPath,
        // The API rejects a fingerprint combined with the insecure skip, so the
        // skip wins and the pin is dropped rather than 400-ing the user.
        host_key_fingerprint: state.insecureSkipHostKeyVerify ? '' : state.hostKeyFingerprint,
        private_key_path: state.privateKeyPath,
        insecure_skip_host_key_verify: state.insecureSkipHostKeyVerify,
      }
  }
}

function buildPayload(state: StorageFormState): StorageRequest {
  const payload: StorageRequest = {
    name: state.name,
    type: state.type,
    keep: state.keep,
    config: buildConfig(state),
  }

  // Secrets travel top-level and only when the user typed one, so editing a
  // backend without retyping them does not overwrite them with blanks.
  if (state.type === 's3' && state.secretAccessKey) {
    payload.secret_access_key = state.secretAccessKey
  }
  if (state.type === 'sftp') {
    if (state.password) {
      payload.password = state.password
    }
    if (state.privateKeyPassphrase) {
      payload.private_key_passphrase = state.privateKeyPassphrase
    }
  }

  return payload
}

export function StorageForm({ open, onOpenChange, storage, onSubmit }: StorageFormProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        {/*
          The page keeps this dialog mounted and swaps `storage` underneath it,
          so the fields live in a child that is mounted fresh on every open.
          Otherwise "Edit" would show whatever the previous session left behind.
        */}
        {open && (
          <StorageFormFields
            key={storage?.name ?? '__new__'}
            storage={storage}
            onOpenChange={onOpenChange}
            onSubmit={onSubmit}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function StorageFormFields({ storage, onOpenChange, onSubmit }: Omit<StorageFormProps, 'open'>) {
  const isEdit = !!storage

  const [formData, setFormData] = useState<StorageFormState>(() => initialState(storage))
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const update = <K extends keyof StorageFormState>(key: K, value: StorageFormState[K]) =>
    setFormData((prev) => ({ ...prev, [key]: value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)

    try {
      await onSubmit(buildPayload(formData))
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save storage')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{isEdit ? 'Edit Storage' : 'Create Storage'}</DialogTitle>
        <DialogDescription>
          {isEdit
            ? 'Update storage backend configuration'
            : 'Configure a new storage backend for backups'}
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 p-3 rounded-md text-sm">
            {error}
          </div>
        )}

        <div className="space-y-2">
          <Label htmlFor="name">
            Name <span className="text-red-500">*</span>
          </Label>
          <Input
            id="name"
            value={formData.name}
            onChange={(e) => update('name', e.target.value)}
            placeholder="my-storage"
            required
            disabled={isEdit}
          />
          {isEdit && <p className="text-xs text-gray-500">Storage name cannot be changed</p>}
        </div>

        <div className="space-y-2">
          <Label htmlFor="type">
            Type <span className="text-red-500">*</span>
          </Label>
          <Select
            value={formData.type}
            onValueChange={(value) => update('type', value as StorageType)}
            disabled={isEdit}
          >
            <SelectTrigger id="type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="local">Local</SelectItem>
              <SelectItem value="s3">S3</SelectItem>
              <SelectItem value="sftp">SFTP</SelectItem>
            </SelectContent>
          </Select>
          {isEdit && <p className="text-xs text-gray-500">Storage type cannot be changed</p>}
        </div>

        <div className="space-y-2">
          <Label htmlFor="keep">
            Retention (days) <span className="text-red-500">*</span>
          </Label>
          <Input
            id="keep"
            type="number"
            min="1"
            value={formData.keep}
            onChange={(e) => update('keep', parseInt(e.target.value))}
            required
          />
          <p className="text-xs text-gray-500">Number of days to keep backups</p>
        </div>

        {formData.type === 'local' && (
          <div className="space-y-2">
            <Label htmlFor="local_path">
              Path <span className="text-red-500">*</span>
            </Label>
            <Input
              id="local_path"
              value={formData.path}
              onChange={(e) => update('path', e.target.value)}
              placeholder="/var/backups"
              required
            />
          </div>
        )}

        {formData.type === 's3' && (
          <>
            <div className="space-y-2">
              <Label htmlFor="s3_bucket">
                Bucket <span className="text-red-500">*</span>
              </Label>
              <Input
                id="s3_bucket"
                value={formData.bucket}
                onChange={(e) => update('bucket', e.target.value)}
                placeholder="my-backup-bucket"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="s3_region">
                Region <span className="text-red-500">*</span>
              </Label>
              <Input
                id="s3_region"
                value={formData.region}
                onChange={(e) => update('region', e.target.value)}
                placeholder="us-east-1"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="s3_access_key_id">
                Access Key ID <span className="text-red-500">*</span>
              </Label>
              <Input
                id="s3_access_key_id"
                value={formData.accessKeyId}
                onChange={(e) => update('accessKeyId', e.target.value)}
                placeholder="AKIAIOSFODNN7EXAMPLE"
                required
              />
            </div>

            <PasswordInput
              label="Secret Access Key"
              value={formData.secretAccessKey}
              onChange={(value) => update('secretAccessKey', value)}
              placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
              required={!isEdit}
              isEdit={isEdit}
            />

            <div className="space-y-2">
              <Label htmlFor="s3_endpoint_url">Endpoint URL (optional)</Label>
              <Input
                id="s3_endpoint_url"
                value={formData.endpointUrl}
                onChange={(e) => update('endpointUrl', e.target.value)}
                placeholder="https://s3.amazonaws.com"
              />
              <p className="text-xs text-gray-500">Leave blank for AWS S3</p>
            </div>
          </>
        )}

        {formData.type === 'sftp' && (
          <>
            <div className="space-y-2">
              <Label htmlFor="sftp_host">
                Host <span className="text-red-500">*</span>
              </Label>
              <Input
                id="sftp_host"
                value={formData.host}
                onChange={(e) => update('host', e.target.value)}
                placeholder="sftp.example.com"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="sftp_port">
                Port <span className="text-red-500">*</span>
              </Label>
              <Input
                id="sftp_port"
                type="number"
                min="1"
                value={formData.port}
                onChange={(e) => update('port', e.target.value)}
                placeholder="22"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="sftp_username">
                Username <span className="text-red-500">*</span>
              </Label>
              <Input
                id="sftp_username"
                value={formData.username}
                onChange={(e) => update('username', e.target.value)}
                placeholder="backup"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="sftp_path">
                Remote Path <span className="text-red-500">*</span>
              </Label>
              <Input
                id="sftp_path"
                value={formData.path}
                onChange={(e) => update('path', e.target.value)}
                placeholder="/backups"
                required
              />
            </div>

            <fieldset className="space-y-4 rounded-md border p-4">
              <legend className="px-1 text-sm font-medium">Authentication</legend>
              <p className="text-xs text-gray-500">
                Provide a password, a private key, or both — the server offers whichever is
                configured.
              </p>

              <PasswordInput
                label="Password"
                value={formData.password}
                onChange={(value) => update('password', value)}
                placeholder="••••••••"
                isEdit={isEdit}
              />

              <div className="space-y-2">
                <Label htmlFor="sftp_private_key_path">Private Key Path</Label>
                <Input
                  id="sftp_private_key_path"
                  value={formData.privateKeyPath}
                  onChange={(e) => update('privateKeyPath', e.target.value)}
                  placeholder="/etc/bared/id_ed25519"
                />
                <p className="text-xs text-gray-500">
                  OpenSSH private key file, readable by the daemon
                </p>
              </div>

              <PasswordInput
                label="Private Key Passphrase"
                value={formData.privateKeyPassphrase}
                onChange={(value) => update('privateKeyPassphrase', value)}
                placeholder="••••••••"
                isEdit={isEdit}
              />
            </fieldset>

            <fieldset className="space-y-4 rounded-md border p-4">
              <legend className="px-1 text-sm font-medium">Host key verification</legend>
              <p className="text-xs text-gray-500">
                SFTP fails closed: an unknown host key aborts the transfer. Pin a fingerprint or
                point at a known_hosts file the daemon can read.
              </p>

              <div className="space-y-2">
                <Label htmlFor="sftp_known_hosts_path">Known Hosts Path</Label>
                <Input
                  id="sftp_known_hosts_path"
                  value={formData.knownHostsPath}
                  onChange={(e) => update('knownHostsPath', e.target.value)}
                  placeholder="~/.ssh/known_hosts"
                  disabled={formData.insecureSkipHostKeyVerify}
                />
                <p className="text-xs text-gray-500">
                  Leave blank to use <code>~/.ssh/known_hosts</code>
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="sftp_host_key_fingerprint">Host Key Fingerprint</Label>
                <Input
                  id="sftp_host_key_fingerprint"
                  value={formData.hostKeyFingerprint}
                  onChange={(e) => update('hostKeyFingerprint', e.target.value)}
                  placeholder="SHA256:n3s1Xb…"
                  disabled={formData.insecureSkipHostKeyVerify}
                />
                <p className="text-xs text-gray-500">
                  Pins one key instead of consulting known_hosts. Useful in containers with no
                  known_hosts file. Run <code>ssh-keyscan host | ssh-keygen -lf -</code> to obtain
                  it.
                </p>
              </div>

              <div
                className={
                  formData.insecureSkipHostKeyVerify
                    ? 'rounded-md border-2 border-red-500 bg-red-50 dark:bg-red-900/20 p-3'
                    : 'rounded-md border border-red-300 dark:border-red-900 p-3'
                }
              >
                <div className="flex items-start gap-3">
                  <Checkbox
                    id="sftp_insecure_skip_host_key_verify"
                    checked={formData.insecureSkipHostKeyVerify}
                    onCheckedChange={(checked) =>
                      update('insecureSkipHostKeyVerify', checked === true)
                    }
                    className="mt-0.5 border-red-500 data-[state=checked]:bg-red-600 data-[state=checked]:text-white"
                  />
                  <div className="space-y-1">
                    <Label
                      htmlFor="sftp_insecure_skip_host_key_verify"
                      className="flex items-center gap-1.5 font-semibold text-red-600 dark:text-red-400"
                    >
                      <ShieldAlert className="h-4 w-4" aria-hidden="true" />
                      Danger: accept any host key
                    </Label>
                    <p className="text-xs text-red-600 dark:text-red-400">
                      Disables MITM protection. Anything on the network path can impersonate the
                      server and capture both these credentials and every backup you upload. Use
                      only against a host you control on a trusted network.
                    </p>
                    {formData.insecureSkipHostKeyVerify && (
                      <p
                        role="alert"
                        className="text-xs font-semibold text-red-700 dark:text-red-300"
                      >
                        Host key verification is off for this backend. Any pinned fingerprint or
                        known_hosts path will be ignored.
                      </p>
                    )}
                  </div>
                </div>
              </div>
            </fieldset>
          </>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </Button>
        </DialogFooter>
      </form>
    </>
  )
}
