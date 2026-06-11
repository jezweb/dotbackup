import { useEffect, useState } from 'react'
import { X, FolderOpen, File, Loader2, Check, AlertTriangle, Undo2 } from 'lucide-react'
import { ListSnapshots, SnapshotTree, RestoreFile, PickRestoreTarget } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

function shortPath(p: string) { return p.replace(/^\/Users\/[^/]+/, '~') }
function fmtTime(iso: string) {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

export function RestoreDialog({ onClose }: { onClose: () => void }) {
  const [snaps, setSnaps] = useState<main.SnapshotView[]>([])
  const [selSnap, setSelSnap] = useState<string>('')
  const [tree, setTree] = useState<main.NodeView[]>([])
  const [loadingTree, setLoadingTree] = useState(false)
  const [selFile, setSelFile] = useState<string>('') // '' = whole snapshot
  const [target, setTarget] = useState<string>('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => { ListSnapshots().then(setSnaps).catch((e) => setErr(String(e))) }, [])

  const openSnap = async (id: string) => {
    setSelSnap(id); setSelFile(''); setDone(false); setLoadingTree(true); setErr('')
    try { setTree(await SnapshotTree(id)) } catch (e: any) { setErr(String(e)) } finally { setLoadingTree(false) }
  }
  const pickTarget = async () => { const t = await PickRestoreTarget(); if (t) setTarget(t) }
  const doRestore = async () => {
    if (!selSnap || !target) return
    setBusy(true); setErr(''); setDone(false)
    try { await RestoreFile(selSnap, selFile, target); setDone(true) }
    catch (e: any) { setErr(String(e)) } finally { setBusy(false) }
  }

  const files = tree.filter((n) => n.type === 'file')

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-6" onClick={onClose}>
      <div className="flex max-h-full w-full max-w-md flex-col rounded-xl bg-card shadow-xl" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b border-border px-4 py-3">
          <div className="flex items-center gap-2 text-sm font-semibold"><Undo2 className="h-4 w-4 text-primary" /> Restore</div>
          <button onClick={onClose} className="rounded-md p-1 text-fg-muted hover:bg-muted"><X className="h-4 w-4" /></button>
        </div>

        {err && (
          <div className="mx-4 mt-3 flex items-start gap-2 rounded-md bg-danger/5 px-3 py-2 text-xs text-danger">
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" /><span className="break-words">{err}</span>
          </div>
        )}

        <div className="flex-1 overflow-auto p-4">
          {/* 1. snapshot */}
          <label className="mb-1 block text-xs font-medium text-fg-muted">Snapshot</label>
          <select
            value={selSnap} onChange={(e) => openSnap(e.target.value)}
            className="mb-4 w-full rounded-md border border-border bg-bg px-2 py-1.5 text-sm"
          >
            <option value="" disabled>Choose a point in time…</option>
            {snaps.map((s) => (
              <option key={s.id} value={s.id}>{fmtTime(s.time)} — {s.shortId}</option>
            ))}
          </select>

          {/* 2. what to restore */}
          {selSnap && (
            <>
              <label className="mb-1 block text-xs font-medium text-fg-muted">What to restore</label>
              {loadingTree ? (
                <div className="flex items-center gap-2 py-3 text-xs text-fg-muted"><Loader2 className="h-3.5 w-3.5 animate-spin" /> reading snapshot…</div>
              ) : (
                <div className="mb-4 max-h-44 overflow-auto rounded-md border border-border">
                  <button
                    onClick={() => setSelFile('')}
                    className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm ${selFile === '' ? 'bg-primary/10 text-primary' : 'hover:bg-muted'}`}
                  >
                    <FolderOpen className="h-3.5 w-3.5" /> Everything in this snapshot
                  </button>
                  {files.map((n) => (
                    <button
                      key={n.path} onClick={() => setSelFile(n.path)}
                      className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm ${selFile === n.path ? 'bg-primary/10 text-primary' : 'hover:bg-muted'}`}
                      title={n.path}
                    >
                      <File className="h-3.5 w-3.5 shrink-0" /><span className="truncate">{shortPath(n.path)}</span>
                    </button>
                  ))}
                </div>
              )}

              {/* 3. target */}
              <label className="mb-1 block text-xs font-medium text-fg-muted">Restore into</label>
              <button onClick={pickTarget} className="mb-2 flex w-full items-center gap-2 rounded-md border border-border px-3 py-1.5 text-left text-sm hover:bg-muted">
                <FolderOpen className="h-3.5 w-3.5" />
                {target ? <span className="truncate">{shortPath(target)}</span> : <span className="text-fg-muted">Choose a folder…</span>}
              </button>
            </>
          )}
        </div>

        <div className="flex items-center gap-2 border-t border-border px-4 py-3">
          {done && <div className="flex items-center gap-1 text-xs text-success"><Check className="h-3.5 w-3.5" /> Restored</div>}
          <div className="flex-1" />
          <button onClick={onClose} className="rounded-md px-3 py-1.5 text-sm hover:bg-muted">Close</button>
          <button
            onClick={doRestore} disabled={!selSnap || !target || busy}
            className="flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-fg hover:opacity-90 disabled:opacity-40"
          >
            {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Undo2 className="h-4 w-4" />} Restore
          </button>
        </div>
      </div>
    </div>
  )
}
