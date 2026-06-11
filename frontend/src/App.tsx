import { useEffect, useState, useCallback } from 'react'
import {
  Cloud, FolderPlus, HardDriveUpload, RefreshCw, Trash2,
  Check, Loader2, ShieldCheck, AlertTriangle, Undo2, CalendarClock,
} from 'lucide-react'
import { RestoreDialog } from './components/RestoreDialog'
import { SetupWizard } from './components/SetupWizard'
import {
  GetStatus, ListSnapshots, BackupNow, BackupAll,
  PickFolder, AddFolder, RemoveFolder, SetFolderBackup,
  GetSchedule, SetSchedule,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { main } from '../wailsjs/go/models'

type Progress = { percent: number; filesDone: number; totalFiles: number }

function relativeTime(iso: string): string {
  if (!iso) return 'never backed up'
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return 'never backed up'
  const secs = Math.max(0, Math.floor((Date.now() - then) / 1000))
  if (secs < 60) return 'backed up just now'
  if (secs < 3600) return `backed up ${Math.floor(secs / 60)} min ago`
  if (secs < 86400) return `backed up ${Math.floor(secs / 3600)} h ago`
  return `backed up ${Math.floor(secs / 86400)} d ago`
}

function shortPath(p: string): string {
  return p.replace(/^\/Users\/[^/]+/, '~')
}

export default function App() {
  const [status, setStatus] = useState<main.StatusView | null>(null)
  const [snapshots, setSnapshots] = useState<main.SnapshotView[]>([])
  const [progress, setProgress] = useState<Record<string, Progress>>({})
  const [error, setError] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [showRestore, setShowRestore] = useState(false)
  const [sched, setSched] = useState<main.ScheduleView | null>(null)

  const refresh = useCallback(async () => {
    const s = await GetStatus()
    setStatus(s)
    if (s.configured) {
      try {
        setSnapshots(await ListSnapshots())
        setSched(await GetSchedule())
      } catch (e: any) {
        setError(String(e))
      }
    }
    setLoading(false)
  }, [])

  useEffect(() => {
    refresh()
    const offProg = EventsOn('backup:progress', (d: any) => {
      setProgress((p) => ({ ...p, [d.path]: { percent: d.percent, filesDone: d.filesDone, totalFiles: d.totalFiles } }))
    })
    const offDone = EventsOn('backup:done', (d: any) => {
      setProgress((p) => { const n = { ...p }; delete n[d.path]; return n })
      refresh()
    })
    const offErr = EventsOn('backup:error', (d: any) => {
      setProgress((p) => { const n = { ...p }; delete n[d.path]; return n })
      setError(d.error || 'backup failed')
    })
    return () => { offProg(); offDone(); offErr() }
  }, [refresh])

  const onAdd = async () => {
    const path = await PickFolder()
    if (path) { await AddFolder(path); refresh() }
  }
  const onBackup = async (path: string) => {
    setError('')
    setProgress((p) => ({ ...p, [path]: { percent: 0, filesDone: 0, totalFiles: 0 } }))
    try { await BackupNow(path) } catch (e: any) { setError(String(e)) }
  }
  const onBackupAll = async () => {
    setError('')
    try { await BackupAll() } catch (e: any) { setError(String(e)) }
  }
  const onToggle = async (path: string, on: boolean) => { await SetFolderBackup(path, on); refresh() }
  const onRemove = async (path: string) => { await RemoveFolder(path); refresh() }

  if (loading) {
    return <div className="flex h-full items-center justify-center text-fg-muted"><Loader2 className="animate-spin" /></div>
  }

  const toggleSchedule = async () => {
    if (!sched) return
    const next = !sched.enabled
    setSched({ ...sched, enabled: next })
    try { await SetSchedule(next, sched.everyHours) } catch (e: any) { setError(String(e)); setSched(await GetSchedule()) }
  }

  if (!status?.configured) return <SetupWizard onDone={refresh} />

  const anyRunning = Object.keys(progress).length > 0

  return (
    <div className="flex h-full flex-col bg-bg text-fg">
      <header className="titlebar-drag flex items-center gap-2 border-b border-border px-4 pt-10 pb-3">
        <Cloud className="h-5 w-5 text-primary" />
        <div className="flex-1">
          <div className="text-sm font-semibold leading-none">dotbackup</div>
          <div className="mt-1 flex items-center gap-1 text-xs text-fg-muted">
            <ShieldCheck className="h-3 w-3 text-success" />
            encrypted to {status.bucket}
          </div>
        </div>
        <button onClick={refresh} className="no-drag rounded-md p-1.5 text-fg-muted hover:bg-muted" title="Refresh">
          <RefreshCw className="h-4 w-4" />
        </button>
      </header>

      {error && (
        <div className="flex items-start gap-2 border-b border-border bg-danger/5 px-4 py-2 text-xs text-danger">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span className="break-words">{error}</span>
        </div>
      )}

      <main className="flex-1 overflow-auto">
        {status.folders.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-fg-muted">
            <FolderPlus className="h-8 w-8" />
            <div className="text-sm">No folders yet</div>
            <button onClick={onAdd} className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-fg hover:opacity-90">
              Add a folder
            </button>
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {status.folders.map((f) => {
              const p = progress[f.path]
              return (
                <li key={f.path} className="flex items-center gap-3 px-4 py-3">
                  <input
                    type="checkbox" checked={f.backup}
                    onChange={(e) => onToggle(f.path, e.target.checked)}
                    className="h-4 w-4 accent-primary"
                    title="Back up this folder"
                  />
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium" title={f.path}>{shortPath(f.path)}</div>
                    {p ? (
                      <div className="mt-1">
                        <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                          <div className="h-full bg-primary transition-all" style={{ width: `${Math.round(p.percent * 100)}%` }} />
                        </div>
                        <div className="mt-1 text-xs text-fg-muted">
                          backing up… {Math.round(p.percent * 100)}%{p.totalFiles ? ` (${p.filesDone}/${p.totalFiles})` : ''}
                        </div>
                      </div>
                    ) : (
                      <div className="mt-0.5 flex items-center gap-1 text-xs text-fg-muted">
                        {f.lastBackupAt && <Check className="h-3 w-3 text-success" />}
                        {relativeTime(f.lastBackupAt)}
                      </div>
                    )}
                  </div>
                  <button
                    onClick={() => onBackup(f.path)} disabled={!!p}
                    className="no-drag rounded-md border border-border px-2.5 py-1 text-xs font-medium hover:bg-muted disabled:opacity-40"
                  >
                    {p ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : 'Back up now'}
                  </button>
                  <button onClick={() => onRemove(f.path)} className="no-drag rounded-md p-1 text-fg-muted hover:bg-muted hover:text-danger" title="Remove">
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </main>

      {sched && (
        <div className="flex items-center gap-2 border-t border-border bg-muted/30 px-4 py-2 text-xs">
          <CalendarClock className="h-3.5 w-3.5 text-fg-muted" />
          <span className="text-fg-muted">Automatic backups{sched.enabled ? ` every ${sched.everyHours}h` : ' off'}</span>
          <div className="flex-1" />
          <button
            onClick={toggleSchedule} aria-label="Toggle automatic backups"
            className={`relative h-5 w-9 shrink-0 rounded-full transition-colors ${sched.enabled ? 'bg-primary' : 'bg-border'}`}
          >
            <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-all ${sched.enabled ? 'left-[18px]' : 'left-0.5'}`} />
          </button>
        </div>
      )}

      <footer className="flex items-center gap-2 border-t border-border px-4 py-3">
        <button onClick={onAdd} className="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1.5 text-sm hover:bg-muted">
          <FolderPlus className="h-4 w-4" /> Add folder
        </button>
        <button
          onClick={onBackupAll} disabled={anyRunning || status.folders.every((f) => !f.backup)}
          className="flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-fg hover:opacity-90 disabled:opacity-40"
        >
          <HardDriveUpload className="h-4 w-4" /> Back up all
        </button>
        <button
          onClick={() => setShowRestore(true)} disabled={snapshots.length === 0}
          className="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1.5 text-sm hover:bg-muted disabled:opacity-40"
        >
          <Undo2 className="h-4 w-4" /> Restore
        </button>
        <div className="flex-1" />
        <div className="text-xs text-fg-muted">{snapshots.length} snapshot{snapshots.length === 1 ? '' : 's'}</div>
      </footer>

      {showRestore && <RestoreDialog onClose={() => setShowRestore(false)} />}
    </div>
  )
}

