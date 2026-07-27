import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '../../test/utils'
import type { Storage, StorageRequest, StorageType } from '../../types'
import { StorageForm } from './StorageForm'

/**
 * The config keys `requestToStorage` (apps/api/internal/api/config_handlers.go)
 * actually reads, per type. Anything else the form sends is silently dropped by
 * the server — which is how SFTP shipped sending `user` while the API read
 * `username`, and how S3 shipped sending `endpoint` instead of `endpoint_url`.
 *
 * `path` is required for all three types: it is the local directory, the S3 key
 * prefix inside the bucket, and the SFTP remote base directory
 * (`config.Storage.Path`). #109 taught `requestToStorage` to read it for S3 and
 * SFTP, but the form kept omitting it for S3 — and since `UpdateStorage`
 * rewrites `config_json` wholesale, every edit moved a prefixed bucket back to
 * its root.
 */
const BACKEND_CONFIG_KEYS: Record<StorageType, string[]> = {
  local: ['path'],
  s3: ['bucket', 'region', 'access_key_id', 'endpoint_url', 'path'],
  sftp: [
    'host',
    'port',
    'username',
    'path',
    'known_hosts_path',
    'host_key_fingerprint',
    'private_key_path',
    'insecure_skip_host_key_verify',
  ],
}

const ALLOWED_EXTRA_CONFIG_KEYS: Record<StorageType, string[]> = {
  local: [],
  s3: [],
  sftp: [],
}

type User = ReturnType<typeof userEvent.setup>

const field = {
  name: () => screen.getByLabelText(/^name \*$/i),
  localPath: () => screen.getByLabelText(/^path \*$/i),
  bucket: () => screen.getByLabelText(/^bucket \*$/i),
  region: () => screen.getByLabelText(/^region \*$/i),
  accessKeyId: () => screen.getByLabelText(/^access key id \*$/i),
  secretAccessKey: () => screen.getByLabelText(/^secret access key/i),
  endpointUrl: () => screen.getByLabelText(/^endpoint url/i),
  host: () => screen.getByLabelText(/^host \*$/i),
  port: () => screen.getByLabelText(/^port \*$/i),
  username: () => screen.getByLabelText(/^username \*$/i),
  remotePath: () => screen.getByLabelText(/^remote path \*$/i),
  password: () => screen.getByLabelText(/^password$/i),
  privateKeyPath: () => screen.getByLabelText(/^private key path$/i),
  privateKeyPassphrase: () => screen.getByLabelText(/^private key passphrase$/i),
  knownHostsPath: () => screen.getByLabelText(/^known hosts path$/i),
  hostKeyFingerprint: () => screen.getByLabelText(/^host key fingerprint$/i),
  insecureSkip: () => screen.getByRole('checkbox', { name: /accept any host key/i }),
}

function renderForm(storage?: Storage) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  render(<StorageForm open onOpenChange={vi.fn()} storage={storage} onSubmit={onSubmit} />)
  return { onSubmit, user: userEvent.setup() }
}

async function selectType(user: User, optionLabel: string) {
  await user.click(screen.getByRole('combobox', { name: /^type \*$/i }))
  await user.click(screen.getByRole('option', { name: optionLabel }))
}

async function submit(user: User) {
  await user.click(screen.getByRole('button', { name: /^create$/i }))
}

const submitted = (onSubmit: ReturnType<typeof vi.fn>): StorageRequest => onSubmit.mock.calls[0][0]

async function fillSftpRequiredFields(user: User) {
  await user.type(field.name(), 'offsite')
  await selectType(user, 'SFTP')
  await user.type(field.host(), 'sftp.example.com')
  await user.clear(field.port())
  await user.type(field.port(), '2222')
  await user.type(field.username(), 'backup')
  await user.type(field.remotePath(), '/backups')
}

describe('StorageForm', () => {
  // Regression for #95: the form sent `user` while the API reads `username`, so
  // every SFTP backend created from the dashboard failed validation with an
  // empty username — and none of the host-key fields could be set at all.
  it('sends an SFTP payload matching the backend StorageRequest contract', async () => {
    const { onSubmit, user } = renderForm()

    await fillSftpRequiredFields(user)
    await user.type(field.password(), 's3cret')
    await user.type(field.privateKeyPath(), '/etc/bared/id_ed25519')
    await user.type(field.privateKeyPassphrase(), 'unlock-me')
    await user.type(field.knownHostsPath(), '/etc/bared/known_hosts')
    await user.type(field.hostKeyFingerprint(), 'SHA256:abc123')

    await submit(user)

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(submitted(onSubmit)).toEqual({
      name: 'offsite',
      type: 'sftp',
      keep: 7,
      config: {
        host: 'sftp.example.com',
        // A JSON number: the handler reads `req.Config["port"].(float64)`.
        port: 2222,
        username: 'backup',
        path: '/backups',
        known_hosts_path: '/etc/bared/known_hosts',
        host_key_fingerprint: 'SHA256:abc123',
        private_key_path: '/etc/bared/id_ed25519',
        insecure_skip_host_key_verify: false,
      },
      // Secrets ride top-level so the API never echoes them back in a response.
      password: 's3cret',
      private_key_passphrase: 'unlock-me',
    })
  })

  it.each([
    { type: 'local' as const, option: 'Local' },
    { type: 's3' as const, option: 'S3' },
    { type: 'sftp' as const, option: 'SFTP' },
  ])('sends only config keys the API reads for $type storage', async ({ type, option }) => {
    const { onSubmit, user } = renderForm()

    if (type === 'sftp') {
      await fillSftpRequiredFields(user)
    } else {
      await user.type(field.name(), 'store')
      await selectType(user, option)
      if (type === 'local') {
        await user.type(field.localPath(), '/var/backups')
      } else {
        await user.type(field.bucket(), 'my-bucket')
        await user.type(field.region(), 'us-east-1')
        await user.type(field.accessKeyId(), 'AKIA')
        await user.type(field.secretAccessKey(), 'shhh')
      }
    }

    await submit(user)

    const sent = Object.keys(submitted(onSubmit).config)
    const known = [...BACKEND_CONFIG_KEYS[type], ...ALLOWED_EXTRA_CONFIG_KEYS[type]]
    expect(sent.filter((key) => !known.includes(key))).toEqual([])
    for (const key of BACKEND_CONFIG_KEYS[type]) {
      expect(sent).toContain(key)
    }
  })

  // Same class of drift as #95, found while checking the other backends: the S3
  // branch sent `endpoint`, which the API ignores in favour of `endpoint_url`,
  // so custom endpoints (MinIO, R2) were dropped on save and lost on edit.
  it('sends the S3 custom endpoint as endpoint_url', async () => {
    const { onSubmit, user } = renderForm()

    await user.type(field.name(), 'minio')
    await selectType(user, 'S3')
    await user.type(field.bucket(), 'my-bucket')
    await user.type(field.region(), 'us-east-1')
    await user.type(field.accessKeyId(), 'AKIA')
    await user.type(field.secretAccessKey(), 'shhh')
    await user.type(field.endpointUrl(), 'https://minio.internal')

    await submit(user)

    const payload = submitted(onSubmit)
    expect(payload.config).toEqual({
      bucket: 'my-bucket',
      region: 'us-east-1',
      access_key_id: 'AKIA',
      endpoint_url: 'https://minio.internal',
      path: '',
    })
    expect(payload.secret_access_key).toBe('shhh')
  })

  // #109 taught requestToStorage to read `path` for S3 (the key prefix inside
  // the bucket), but the form kept omitting it. Because UpdateStorage rewrites
  // config_json wholesale, editing anything on a prefixed bucket moved every
  // later backup to the bucket root.
  it('sends the S3 key prefix as path', async () => {
    const { onSubmit, user } = renderForm()

    await user.type(field.name(), 's3')
    await selectType(user, 'S3')
    await user.type(field.bucket(), 'my-bucket')
    await user.type(field.region(), 'us-east-1')
    await user.type(field.accessKeyId(), 'AKIA')
    await user.type(field.secretAccessKey(), 'shhh')
    await user.type(screen.getByLabelText(/^path prefix/i), 'backups/')

    await submit(user)

    expect(submitted(onSubmit).config).toMatchObject({ path: 'backups/' })
  })

  // The API rejects a pinned fingerprint combined with the insecure skip, so
  // the form must not send both.
  it('drops a pinned fingerprint when host key verification is disabled', async () => {
    const { onSubmit, user } = renderForm()

    await fillSftpRequiredFields(user)
    await user.type(field.hostKeyFingerprint(), 'SHA256:abc123')
    await user.click(field.insecureSkip())

    expect(screen.getByRole('alert')).toHaveTextContent(/host key verification is off/i)

    await submit(user)

    expect(submitted(onSubmit).config).toMatchObject({
      insecure_skip_host_key_verify: true,
      host_key_fingerprint: '',
    })
  })

  it('omits blank secrets so saving does not wipe the stored ones', async () => {
    const { onSubmit, user } = renderForm()

    await fillSftpRequiredFields(user)
    await submit(user)

    const payload = submitted(onSubmit)
    expect(payload).not.toHaveProperty('password')
    expect(payload).not.toHaveProperty('private_key_passphrase')
  })

  it('pre-fills an existing SFTP backend without echoing redacted secrets', () => {
    renderForm({
      name: 'offsite',
      type: 'sftp',
      keep: 14,
      config: {
        host: 'sftp.example.com',
        port: 2222,
        username: 'backup',
        known_hosts_path: '/etc/bared/known_hosts',
        host_key_fingerprint: 'SHA256:abc123',
        private_key_path: '/etc/bared/id_ed25519',
        insecure_skip_host_key_verify: false,
        password: '***REDACTED***',
        private_key_passphrase: '***REDACTED***',
      },
      enabled: true,
      created_at: '',
      updated_at: '',
    })

    expect(field.name()).toHaveValue('offsite')
    expect(field.host()).toHaveValue('sftp.example.com')
    expect(field.username()).toHaveValue('backup')
    expect(field.knownHostsPath()).toHaveValue('/etc/bared/known_hosts')
    expect(field.hostKeyFingerprint()).toHaveValue('SHA256:abc123')
    expect(field.privateKeyPath()).toHaveValue('/etc/bared/id_ed25519')
    // The API returns `***REDACTED***`; echoing it back would save it verbatim.
    expect(field.password()).toHaveValue('')
    expect(field.privateKeyPassphrase()).toHaveValue('')
  })
})
