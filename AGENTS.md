# dotbackup — agent guide

dotbackup is a small macOS desktop app that backs up a person's chosen folders to
**their own Cloudflare R2**, encrypted, restorable, and scheduled. It wraps
`restic`; the user owns the bucket and the keys. This file orients an AI coding
agent helping someone **use** or **develop** dotbackup.

## The one rule that overrides everything

**The repo passphrase is the only key to the data. If it is lost, the backups are
gone forever — nobody can recover them.** Setup shows it once. Always make sure the
user has saved it (password manager) before treating setup as done. Never invent,
guess, or "reset" a passphrase.

## Helping a user set it up

Two paths — prefer the in-app wizard.

1. **In-app wizard (preferred).** The user opens dotbackup; if it's not configured
   it shows a Connect screen. They need, from the Cloudflare dashboard:
   - **R2 → create a bucket** (e.g. `dotbackup-<name>`).
   - **R2 → API → Manage API Tokens → Create API Token**, permission **Object Read
     & Write**, scoped to that **one bucket** (least privilege).
   - Paste the **S3 endpoint**, **bucket name**, **Access Key ID**, and **Secret
     Access Key** into the wizard and click Connect. The app inits the encrypted
     repo, stores the secret + a generated passphrase in the macOS Keychain, and
     shows the recovery passphrase once.
2. **Agent-driven (`skills/setup-dotbackup`).** If the user would rather their agent
   wire it up, run that skill. Same result, scripted.

After setup: add folders, optionally turn on **Automatic backups** (the footer
toggle installs a launchd LaunchAgent so backups run with the app closed), and use
**Restore** to browse a snapshot and pull a file back.

## Building / installing from source

```bash
# prerequisites: Go 1.23+, Node 20+, pnpm, and the Wails v2 CLI + restic
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
brew install restic            # dev convenience; releases vendor the binary

wails build                    # -> build/bin/dotbackup.app
# or: wails dev                # hot-reload during development
```

The app shells out to `restic`; in dev it finds it on PATH / `DOTBACKUP_RESTIC`,
a release vendors a pinned binary in the bundle.

## Architecture (where things live)

| Area | Path | Note |
|---|---|---|
| Engine wrapper | `internal/restic/` | The only code that knows restic. Drives it over `--json`. Engine-agnostic boundary — swap engines here, nothing else. |
| Secrets | `internal/secret/` | Passphrase + S3 secret in the macOS Keychain. Never in config, plist, or argv. |
| Config | `internal/config/` | `~/Library/Application Support/dotbackup/config.json` — repo coords + folders + schedule. No secrets. |
| Scheduling | `internal/schedule/` | launchd LaunchAgent + the headless runner. |
| App surface | `app.go` | Wails-bound methods the UI calls. None returns a secret. |
| Entry + headless | `main.go` | GUI entry; also dispatches `--print-passphrase` (restic's password command) and `--run-scheduled` (launchd). |
| UI | `frontend/src/` | React 19 + Vite + Tailwind v4. `App.tsx` is the one screen. |

## Non-obvious gotchas (learned the hard way)

- **launchd `ProcessType` must be `Standard`, not `Background`.** Background QoS
  throttles network I/O so hard a backup never finishes. (`internal/schedule/launchd.go`)
- **All lock-taking restic commands carry `--retry-lock`** so the app (listing
  snapshots) and the scheduler (forget/prune) don't collide on the repo lock.
- **The passphrase reaches restic only via `RESTIC_PASSWORD_COMMAND`** (the app
  binary's `--print-passphrase` mode reading the Keychain) — never an env var.
- **Keychain ACL + ad-hoc signing:** a locally-built (ad-hoc signed) app may prompt
  on first Keychain read after each rebuild; a Developer-ID-signed/notarised release
  is stable. Click "Always Allow" when testing local builds.

## Status & scope

Phase 1 = backup on macOS (done; see README slice tracker). Phase 2 = live
multi-device sync (the goannad core), not yet built. Windows/Linux later — the
engine + UI are cross-platform; Keychain + launchd are the macOS-specific bits.
