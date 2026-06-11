import { useState } from 'react'
import { Cloud, Loader2, AlertTriangle, KeyRound, Copy, Check, ShieldCheck } from 'lucide-react'
import { Setup } from '../../wailsjs/go/main/App'

type Fields = { endpoint: string; bucket: string; accessKeyId: string; secretAccessKey: string }

export function SetupWizard({ onDone }: { onDone: () => void }) {
  const [f, setF] = useState<Fields>({ endpoint: '', bucket: '', accessKeyId: '', secretAccessKey: '' })
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [copied, setCopied] = useState(false)
  const [saved, setSaved] = useState(false)

  const set = (k: keyof Fields) => (e: React.ChangeEvent<HTMLInputElement>) => setF((s) => ({ ...s, [k]: e.target.value }))

  const connect = async () => {
    setErr(''); setBusy(true)
    try {
      const pass = await Setup(f as any)
      setPassphrase(pass)
    } catch (e: any) {
      setErr(String(e).replace(/^Error:\s*/, ''))
    } finally {
      setBusy(false)
    }
  }

  const copy = async () => {
    try { await navigator.clipboard.writeText(passphrase); setCopied(true); setTimeout(() => setCopied(false), 1500) } catch {}
  }

  // --- recovery screen ---
  if (passphrase) {
    return (
      <div className="titlebar-drag flex h-full flex-col items-center justify-center gap-4 px-8 pt-8 text-center">
        <KeyRound className="h-9 w-9 text-primary" />
        <div className="text-lg font-semibold">Save your recovery passphrase</div>
        <p className="max-w-sm text-sm text-fg-muted">
          This is the only key to your encrypted backups. If this Mac is lost or its keychain is wiped, this passphrase is
          the only way to get your files back. We do not keep a copy and cannot recover it for you.
        </p>
        <div className="no-drag flex w-full max-w-sm items-center gap-2 rounded-lg border border-border bg-muted px-3 py-2">
          <code className="flex-1 break-all text-left text-sm">{passphrase}</code>
          <button onClick={copy} className="shrink-0 rounded-md p-1.5 hover:bg-bg" title="Copy">
            {copied ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
          </button>
        </div>
        <label className="no-drag flex items-center gap-2 text-sm">
          <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} className="h-4 w-4 accent-primary" />
          I've saved this in my password manager
        </label>
        <button
          onClick={onDone} disabled={!saved}
          className="no-drag rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-fg hover:opacity-90 disabled:opacity-40"
        >
          Continue
        </button>
      </div>
    )
  }

  // --- connect form ---
  return (
    <div className="titlebar-drag flex h-full flex-col px-8 pt-10 pb-6">
      <div className="mb-4 flex items-center gap-2">
        <Cloud className="h-6 w-6 text-primary" />
        <div>
          <div className="text-base font-semibold">Connect dotbackup to your Cloudflare R2</div>
          <div className="text-xs text-fg-muted">Encrypted backups to a bucket you own.</div>
        </div>
      </div>

      <div className="no-drag mb-3 rounded-lg border border-border bg-muted/50 p-3 text-xs text-fg-muted">
        In the Cloudflare dashboard: <span className="font-medium text-fg">R2 → create a bucket</span>, then{' '}
        <span className="font-medium text-fg">R2 → API → Manage API Tokens → Create API Token</span> (Object Read &amp; Write,
        scoped to that one bucket). Paste the S3 endpoint, bucket name, Access Key ID and Secret Access Key it shows you.
      </div>

      {err && (
        <div className="no-drag mb-3 flex items-start gap-2 rounded-md bg-danger/5 px-3 py-2 text-xs text-danger">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" /><span className="break-words">{err}</span>
        </div>
      )}

      <div className="no-drag flex flex-1 flex-col gap-3 overflow-auto">
        <Field label="S3 endpoint" placeholder="https://<account-id>.r2.cloudflarestorage.com" value={f.endpoint} onChange={set('endpoint')} />
        <Field label="Bucket name" placeholder="dotbackup-jez" value={f.bucket} onChange={set('bucket')} />
        <Field label="Access Key ID" placeholder="from the R2 API token" value={f.accessKeyId} onChange={set('accessKeyId')} />
        <Field label="Secret Access Key" placeholder="shown once when you create the token" value={f.secretAccessKey} onChange={set('secretAccessKey')} type="password" />
      </div>

      <div className="no-drag mt-4 flex items-center gap-2">
        <div className="flex items-center gap-1 text-xs text-fg-muted"><ShieldCheck className="h-3.5 w-3.5 text-success" /> encrypted, keys stay in your Keychain</div>
        <div className="flex-1" />
        <button
          onClick={connect} disabled={busy}
          className="flex items-center gap-1.5 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-fg hover:opacity-90 disabled:opacity-50"
        >
          {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Cloud className="h-4 w-4" />} Connect
        </button>
      </div>
    </div>
  )
}

function Field({ label, ...props }: { label: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-medium text-fg-muted">{label}</span>
      <input {...props} className="w-full rounded-md border border-border bg-bg px-2.5 py-1.5 text-sm outline-none focus:border-primary" />
    </label>
  )
}
