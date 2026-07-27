# Security Policy

BareD moves database dumps offsite. It handles database passwords, S3 access keys,
SFTP credentials, and an encryption key. Treat it as security-sensitive software and
read the [Known limitations](#known-limitations) before you deploy it.

## Supported versions

BareD is pre-1.0. Only the latest release receives security fixes; there are no
backported patches for older minor versions.

| Version | Supported |
| ------- | --------- |
| Latest release (`0.4.x`) | Yes |
| Any earlier release | No — upgrade |
| `main` | Best effort |

Because the project is pre-1.0, a security fix may ship in a minor version bump that
also carries other changes. Pin a version and read the release notes before upgrading.

## Reporting a vulnerability

**Do not open a public issue for a security problem.**

Report privately through GitHub's private vulnerability reporting:

1. Go to <https://github.com/etowett/bared/security/advisories/new>
2. Or: the repository's **Security** tab → **Report a vulnerability**

If that is unavailable to you, open a public issue containing only a request for a
private contact channel — no details, no reproducer.

Please include what you have:

- The affected version or commit, and the component (storage backend, database
  engine, HTTP API, web UI, config handling)
- What an attacker gains, and what access they need to start
- Reproduction steps or a proof of concept
- Any suggested fix

**Never include real credentials, real backup data, or a live host in a report.**
Redact them.

### What to expect

BareD is maintained by one person, so these are honest targets rather than a
contractual SLA:

| Stage | Target |
| ----- | ------ |
| Acknowledgement of your report | 3 business days |
| Initial assessment and severity | 7 business days |
| Fix or documented mitigation for high severity | 30 days |
| Fix or documented mitigation for lower severity | Next scheduled release |

You will be credited in the advisory and release notes unless you ask not to be.
Please give the project a reasonable window to ship a fix before disclosing publicly.
If you do not hear back within the acknowledgement window, escalate by opening a
public issue asking for contact — again, with no details.

## Known limitations

These are real, currently unfixed, and tracked in public. They are listed here so you
can make an informed decision rather than discovering them after the fact.

### The encryption key is stored alongside the data it protects

**An accepted design trade-off, not a bug** — documented under
[#87](https://github.com/etowett/bared/issues/87). It is here so you can decide
whether it is acceptable for your deployment.

When `BARED_ENCRYPTION_KEY` is not set, BareD generates a key and stores it
base64-encoded, in plaintext, in the **same SQLite database** as the encrypted
credentials (`apps/api/internal/daemon/daemon.go`). Anyone who can read that database
file gets both the ciphertext and the key.

This is a deliberate trade-off for a single-binary daemon with nowhere else to put a
key, but it means database-backed secret encryption protects against *casual* reads
of individual rows — not against someone who has the file.

*What actually protects you:*

- Set `BARED_ENCRYPTION_KEY` in the environment from a real secret store, so the key
  never lands in the database. Generate it with `openssl rand -base64 32` — the value
  is **base64**, 32 bytes decoded.
- Restrict filesystem permissions on the SQLite database and its directory.
- Back up the key separately from the database.

> **Ordering trap:** the environment variable takes precedence over the stored key.
> If a key already exists in the database and you *then* set `BARED_ENCRYPTION_KEY`
> to a different value, previously-encrypted credentials become undecryptable. Set it
> before first start, or re-enter your secrets after changing it.

### Not audited

BareD has not had an independent security audit. Test coverage is roughly 36% against
a CI-enforced ratchet of 34% (`make coverage-check`), which only ever moves up.
`cmd/brd` has no tests, and `internal/notify` — which handles webhook and SMTP
credentials — is still in the 20s.

## Deployment hardening

If you are running BareD against production data:

- **Do not expose the HTTP interface publicly.** Bind it to localhost or a private
  interface and front it with a VPN or authenticating proxy. Use
  `--http-secure-cookies` and `--http-allowed-origin` when behind a TLS-terminating
  proxy.
- **Least privilege for database credentials.** A backup user needs read access, not
  `SUPER`. A restore target is separate and inherently destructive — scope it
  narrowly.
- **Least privilege for storage credentials.** Scope S3 keys to a single bucket and
  prefix. Prefer write-only where your provider supports it.
- **Never set `insecure_skip_host_key_verify` on an SFTP storage.** Host key
  verification is on by default — `known_hosts_path` (default `~/.ssh/known_hosts`),
  or pin one key with `host_key_fingerprint`. The insecure flag accepts any host key,
  which hands an on-path attacker both the SFTP credentials and the backup stream. It
  exists for throwaway local testing and warns loudly every time it is used.
- **Prefer key auth for SFTP.** `private_key_path` (with an optional
  `private_key_passphrase`) avoids putting a reusable password in the config at all.
- **Keep secrets out of config files.** Use `${ENV_VAR}` expansion in YAML and supply
  values from the environment or a secret store.
- **Protect the state directory.** The SQLite database holds job history, encrypted
  credentials, and possibly the encryption key. Restrict it to the daemon's user.
- **Restore is destructive.** It overwrites the target database. Verify restores on a
  scratch database on a schedule — an untested backup is not a backup.
- **Watch what you paste.** Logs and API responses are written with secret redaction
  in mind, but redact anything you share in an issue anyway.

## Scope

**In scope:** authentication and session handling, secret storage and redaction,
path traversal in storage keys or restore targets, command injection through config
values, the encryption implementation, dependency vulnerabilities that BareD actually
reaches, and anything that lets a backup be read or altered by a party who should not
be able to.

**Out of scope:** the [known limitations](#known-limitations) above (already
tracked — please comment on the existing issue rather than filing a report),
vulnerabilities in `mysqldump`/`pg_dump`/`redis-cli` themselves, findings that need
an already-compromised host or root on the BareD machine, missing hardening headers
with no demonstrated impact, and automated scanner output with no working
exploitation path.
