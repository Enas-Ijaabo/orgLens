'use client'

import { useState, useRef } from 'react'
import { useRouter } from 'next/navigation'
import { FileStatus } from '../lib/api'

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

interface Props {
  initialStatus: FileStatus[]
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function groupByFolder(files: FileStatus[]): [string, FileStatus[]][] {
  const map = new Map<string, FileStatus[]>()
  for (const f of files) {
    const parts = f.file.split('/')
    const key = (parts.length > 2 ? parts.slice(0, 2).join('/') : parts[0]) + '/'
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(f)
  }
  return Array.from(map.entries())
}

export default function IngestTab({ initialStatus }: Props) {
  const [status, setStatus] = useState<FileStatus[]>(initialStatus)
  const [isIngesting, setIsIngesting] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const router = useRouter()

  const total = status.length
  const done = status.filter(f => f.ingested).length
  const totalFacts = status.reduce((s, f) => s + (f.facts_count ?? 0), 0)
  const allDone = total > 0 && done === total
  const lastDates = status.filter(f => f.ingested && f.ingested_at).map(f => new Date(f.ingested_at!).getTime())
  const lastAnalysis = lastDates.length ? timeAgo(new Date(Math.max(...lastDates)).toISOString()) : 'never'

  const fetchStatus = async () => {
    try {
      const res = await fetch(`${API}/api/ingest/status`)
      if (res.ok) setStatus((await res.json()) ?? [])
    } catch {}
  }

  const handleIngest = async () => {
    setIsIngesting(true)
    pollRef.current = setInterval(fetchStatus, 2000)
    try {
      await fetch(`${API}/api/ingest`, { method: 'POST' })
    } finally {
      if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
      setIsIngesting(false)
      await fetchStatus()
      router.refresh()
    }
  }

  const firstPendingIdx = isIngesting ? status.findIndex(f => !f.ingested) : -1
  const folders = groupByFolder(status)

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-3xl mx-auto px-6 py-8 space-y-6">

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4">
          {[
            { label: 'Files analyzed', value: total ? `${done} / ${total}` : '— / —' },
            { label: 'Facts discovered', value: totalFacts || '—' },
            { label: 'Last analysis',   value: lastAnalysis },
          ].map(stat => (
            <div key={stat.label} className="bg-zinc-900 border border-zinc-800 rounded-xl px-5 py-4">
              <div className="text-xs text-zinc-500 mb-1.5">{stat.label}</div>
              <div className="text-xl font-semibold text-white">{stat.value}</div>
            </div>
          ))}
        </div>

        {/* Dataset card */}
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl overflow-hidden">
          <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-800">
            <span className="text-sm font-medium text-zinc-300">Dataset files</span>
            {allDone ? (
              <button onClick={handleIngest} disabled={isIngesting}
                className="text-xs px-3 py-1.5 rounded-lg border border-zinc-700 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-colors disabled:opacity-40">
                Re-analyze
              </button>
            ) : (
              <button onClick={handleIngest} disabled={isIngesting}
                className="text-xs px-4 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-medium transition-colors disabled:opacity-40 flex items-center gap-2">
                {isIngesting && <span className="w-3 h-3 border border-white/40 border-t-white rounded-full animate-spin" />}
                {isIngesting ? 'Analyzing…' : 'Analyze Codebase'}
              </button>
            )}
          </div>

          <div className="px-5 py-4">
            {isIngesting ? (
              <div className="font-mono text-xs space-y-2">
                {status.map((f, i) => {
                  const isDone = f.ingested
                  const isCurrent = i === firstPendingIdx
                  return (
                    <div key={f.file} className="flex items-center gap-3">
                      <span className={`w-4 shrink-0 ${isDone ? 'text-emerald-400' : isCurrent ? 'text-yellow-400 animate-pulse' : 'text-zinc-700'}`}>
                        {isDone ? '✓' : isCurrent ? '⟳' : '—'}
                      </span>
                      <span className={`flex-1 truncate ${isDone ? 'text-zinc-400' : isCurrent ? 'text-yellow-200' : 'text-zinc-600'}`}>{f.file}</span>
                      <span className="text-zinc-600 shrink-0">
                        {isDone ? `${f.facts_count} facts` : isCurrent ? 'extracting…' : 'waiting'}
                      </span>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="space-y-5">
                {folders.map(([folder, files]) => (
                  <div key={folder}>
                    <div className="text-[10px] font-semibold text-zinc-600 uppercase tracking-widest mb-2">{folder}</div>
                    <div className="space-y-0.5">
                      {files.map(f => (
                        <div key={f.file} className="flex items-center gap-3 px-2 py-1 rounded-lg hover:bg-zinc-800/50 transition-colors">
                          <span className={`w-3.5 shrink-0 text-[10px] ${f.ingested ? 'text-emerald-500' : 'text-zinc-700'}`}>
                            {f.ingested ? '✓' : '—'}
                          </span>
                          <span className={`font-mono text-xs flex-1 truncate ${f.ingested ? 'text-zinc-300' : 'text-zinc-600'}`}>
                            {f.file.split('/').pop()}
                          </span>
                          <span className="text-zinc-600 text-xs w-20 text-right shrink-0">
                            {f.ingested && f.ingested_at ? timeAgo(f.ingested_at) : 'never'}
                          </span>
                          {f.ingested && f.facts_count !== undefined && (
                            <span className="text-zinc-700 text-xs font-mono w-14 text-right shrink-0">[{f.facts_count}]</span>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
                {total === 0 && <p className="text-zinc-600 text-sm text-center py-6">No dataset files found.</p>}
              </div>
            )}
          </div>
        </div>

      </div>
    </div>
  )
}
