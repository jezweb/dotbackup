---
name: setup-dotbackup
description: Set up dotbackup for a user — point it at their own Cloudflare R2 bucket, create the encrypted restic repository, store the secrets in their macOS Keychain, and write the config the app reads. Use when a user is installing dotbackup, says "set up my backups", connects dotbackup to Cloudflare, or the app shows the "no config" setup screen. macOS, Phase 1 (backup).
---

# Set up dotbackup

You are wiring dotbackup to the user's **own** Cloudflare R2 so their folders back
up encrypted. Your job is the Cloudflare + repo wiring; the app does the rest.

The one thing that matters more than everything else here: **the repo passphrase is
the only way back. Lose it and the data is gone forever.** Step 8 is the most
important step in this whole skill. Do not rush it.

## What you produce

- An R2 bucket in the user's Cloudflare account.
- A **bucket-scoped** S3 credential (least privilege — can touch only this bucket).
- An encrypted restic repository in that bucket.
- Two secrets in the macOS Keychain: the S3 secret and the repo passphrase.
- `~/Library/Application Support/dotbackup/config.json` (no secrets in it).
- The passphrase, recorded by the user somewhere safe.

## Before you start

- macOS, `restic` available (the app vendors it; for setup, `which restic` or use
  the app's copy), `wrangler` logged in to the user's Cloudflare account
  (`wrangler whoami`), and `openssl` (built in).
- Pick a short `USER` tag for keychain entries (e.g. their macOS short name).

## Steps

### 1. Find the account

```bash
wrangler whoami
```
Capture the **Account ID** for the account they want to use. The R2 endpoint is
`https://<ACCOUNT_ID>.r2.cloudflarestorage.com`.

### 2. Create the bucket

```bash
wrangler r2 bucket create dotbackup-<USER>
```
If wrangler's token lacks R2 scope (error about permissions), have the user create
the bucket in the dashboard instead: **Cloudflare dashboard → R2 → Create bucket**,
name it `dotbackup-<USER>`. Either way you just need the bucket to exist.

### 3. Get a bucket-scoped S3 credential (user does this, you guide)

This is the **paste** path — the user creates a least-privilege token themselves, so
no token-minting credential is ever handled by the agent.

Direct the user to: **Cloudflare dashboard → R2 → API → Manage API Tokens →
Create API Token**, then:
- **Permissions:** Object Read & Write
- **Specify bucket:** scope it to `dotbackup-<USER>` only (not "all buckets")
- Create it.

The dashboard then shows, once:
- **Access Key ID**
- **Secret Access Key**

Have them paste both to you. (These are the S3 credentials directly — no hashing or
derivation. The "S3 endpoint" the dashboard shows should match step 1.)

### 4. Generate the repo passphrase

```bash
openssl rand -base64 32
```
Keep this value. It becomes the encryption key for the whole repository.

### 5. Store both secrets in the Keychain

```bash
security add-generic-password -s dotbackup-s3-secret  -a <USER> -w '<SECRET_ACCESS_KEY>' -T /Applications/dotbackup.app -U
security add-generic-password -s dotbackup-passphrase -a <USER> -w '<PASSPHRASE>'         -T /Applications/dotbackup.app -U
```
`-T /Applications/dotbackup.app` lets the app read them without a prompt; `-U`
updates if they already exist. If the app isn't installed yet, the user may get one
keychain permission prompt on first read — that's expected. Never put these values
in `config.json`, the plist, or any file.

### 6. Initialise the encrypted repository

With the env pointing at the bucket (endpoint + bucket + access key id from above,
secret + passphrase from the keychain), run `restic init`. The app's
`--print-passphrase` mode feeds the passphrase via `RESTIC_PASSWORD_COMMAND`, so it
never sits in the environment. A fresh repo is created and encrypted from byte one.

### 7. Write the config

Write `~/Library/Application Support/dotbackup/config.json`:

```jsonc
{
  "version": 1,
  "user": "<USER>",
  "repo": {
    "endpoint": "https://<ACCOUNT_ID>.r2.cloudflarestorage.com",
    "bucket": "dotbackup-<USER>",
    "accessKeyId": "<ACCESS_KEY_ID>"   // the SECRET is in the keychain, never here
  },
  "folders": [
    { "path": "/Users/<USER>/Documents/.jez", "backup": true, "sync": false, "excludes": ["secrets/**"] }
  ],
  "schedule": { "everyHours": 6, "onlyOnWifi": false }
}
```
Pre-tick `~/Documents/.jez` and ask which workspace folder(s) to add. If a chosen
folder contains a `secrets/` subdir, default to **excluding** it (offer to include —
it's always encrypted, so it's safe either way). Never log secret file paths.

### 8. Record the passphrase — the safety-critical moment

Show the passphrase to the user **once**, plainly, with this framing:

> This passphrase is the only way to restore your backups if this Mac is lost or
> the keychain is wiped. Save it now in your password manager. We do not keep a
> copy and cannot recover it for you.

Offer to stash it in their password manager. Confirm they've saved it before you
consider setup done. This is the single most important step — treat it that way.

## Done

The app now reads `config.json` on launch, lists snapshots, and backs up on
schedule. The user can verify with the app's "Back up now".

## Notes for the agent

- **Bucket-scoped, least privilege.** Step 3's token must be scoped to the one
  bucket. A whole-account token would work but violates least privilege — don't.
- **Two secrets, keychain only.** S3 secret and passphrase. Both in the keychain,
  neither in any file.
- **Idempotent-ish.** Re-running is safe: bucket create handles "already exists",
  `restic init` on an existing repo is a no-op, keychain `-U` updates in place.
