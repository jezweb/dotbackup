# dotbackup

A dead-simple desktop backup app: back up your chosen folders (workspace, `.jez`,
anything) to **your own Cloudflare R2**, encrypted by default, restorable, and set
up by your agent so you barely configure anything.

One screen: a list of folders, each with **Backup** and (later) **Sync** toggles,
pointed at your Cloudflare. Encryption is always on. Your stuff is safe.

Part of the [dotjez](https://github.com/jezweb/dotjez) family. Phase 2 folds in
live multi-device sync (the goannad core).

## Status

Phase 1 (backup, macOS first) — in progress.

| Slice | What | State |
|---|---|---|
| 1 | Engine wrapper (`restic` over `--json`) + keychain indirection | ✅ passing its gate against real R2 |
| 2 | Setup skill + config (`restic init`, app reads config) | ✅ config→engine path proven; user-paste R2 token (`skills/setup-dotbackup`) |
| 3 | Wails UI — the one-screen folder list + backup-now | |
| 4 | launchd scheduling (headless runner) | |
| 5 | Restore browser (snapshot → tree → restore) | |
| 6 | Excludes / retention / secrets handling + encryption proof | |

## Architecture

- **Shell** — Wails (Go + React/Vite/Tailwind) tray app.
- **Engine** — vendored `restic` binary, driven over its stable `--json` contract.
  All restic specifics are isolated in `internal/restic` (engine-agnostic boundary).
- **Storage** — the user's own Cloudflare R2 bucket (path-style S3).
- **Secrets** — repo passphrase + S3 secret live in the macOS Keychain, never in
  config, plist, or argv. restic reads the passphrase via `RESTIC_PASSWORD_COMMAND`.
- **Schedule** — a launchd LaunchAgent runs the binary headless; backups happen
  with the app closed.

## The one rule that matters

**Lose the passphrase, lose the data.** Encryption is real (the repo on R2 is
ciphertext). The setup flow surfaces the passphrase once, with a clear "save this"
moment, and offers to stash it in a password manager.

## Build

```bash
go build ./...
```

Design docs (plan + build spec) live in `.jez/plans/` (local only).

## Licence

Copyright (c) 2026 Jezweb Pty Ltd.
