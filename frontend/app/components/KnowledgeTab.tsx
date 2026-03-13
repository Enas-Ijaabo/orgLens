'use client'

import { useState, useMemo } from 'react'
import { Fact } from '../lib/api'

interface Props {
  initialFacts: Fact[]
}

const TYPE_GROUPS = [
  { key: 'business_rule', label: 'Business Rules',  color: 'text-violet-400', dot: 'bg-violet-500' },
  { key: 'architecture',  label: 'Architecture',    color: 'text-blue-400',   dot: 'bg-blue-500' },
  { key: 'constraint',    label: 'Constraints',     color: 'text-amber-400',  dot: 'bg-amber-500' },
  { key: 'behavior',      label: 'Behavior',        color: 'text-emerald-400',dot: 'bg-emerald-500' },
  { key: 'data_rule',     label: 'Data Rules',      color: 'text-teal-400',   dot: 'bg-teal-500' },
  { key: 'decision',      label: 'Decisions',       color: 'text-rose-400',   dot: 'bg-rose-500' },
]

export default function KnowledgeTab({ initialFacts }: Props) {
  const facts = initialFacts
  const [expanded, setExpanded] = useState<Set<string>>(new Set(['business_rule']))
  const [typeFilter, setTypeFilter] = useState('all')
  const [search, setSearch] = useState('')

  const filtered = useMemo(() => facts.filter(f => {
    if (typeFilter !== 'all' && f.Type !== typeFilter) return false
    if (search && !f.Text.toLowerCase().includes(search.toLowerCase())) return false
    return true
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }), [facts, typeFilter, search])

  const toggle = (key: string) =>
    setExpanded(prev => { const n = new Set(prev); n.has(key) ? n.delete(key) : n.add(key); return n })

  const isOpen = (key: string) =>
    search ? filtered.some(f => f.Type === key) : expanded.has(key)

  if (facts.length === 0) {
    return (
      <div className="h-full flex flex-col items-center justify-center gap-3 text-center px-6">
        <div className="w-12 h-12 rounded-full bg-zinc-900 border border-zinc-800 flex items-center justify-center text-xl">🔍</div>
        <p className="text-zinc-300 font-medium">No knowledge extracted yet</p>
        <p className="text-zinc-600 text-sm max-w-xs">Go to the <a href="/ingest" className="text-blue-400 hover:text-blue-300">Ingest tab</a> to analyze your codebase.</p>
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col">
      {/* Top bar */}
      <div className="shrink-0 px-6 py-4 border-b border-zinc-800 bg-zinc-950/80 backdrop-blur-sm">
        <div className="max-w-3xl mx-auto flex items-center gap-3">
          <span className="text-sm text-zinc-500 shrink-0">
            <span className="text-white font-semibold">{facts.length}</span> statements
          </span>
          <select
            value={typeFilter}
            onChange={e => setTypeFilter(e.target.value)}
            className="bg-zinc-900 border border-zinc-800 text-zinc-400 text-xs rounded-lg px-3 py-2 focus:outline-none focus:border-zinc-600 cursor-pointer"
          >
            <option value="all">All types</option>
            {TYPE_GROUPS.map(g => (
              <option key={g.key} value={g.key}>{g.label}</option>
            ))}
          </select>
          <input
            type="text"
            placeholder="Search…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="flex-1 bg-zinc-900 border border-zinc-800 text-zinc-300 text-xs rounded-lg px-3 py-2 placeholder-zinc-600 focus:outline-none focus:border-zinc-600"
          />
          {search && (
            <button onClick={() => setSearch('')} className="text-zinc-600 hover:text-zinc-400 text-xs shrink-0">✕ clear</button>
          )}
        </div>
      </div>

      {/* Groups */}
      <div className="flex-1 overflow-y-auto px-6 py-4">
        <div className="max-w-3xl mx-auto space-y-2">
          {TYPE_GROUPS.map(group => {
            if (typeFilter !== 'all' && typeFilter !== group.key) return null
            const groupFacts = filtered.filter(f => f.Type === group.key)
            const open = isOpen(group.key)

            return (
              <div key={group.key} className="border border-zinc-800 rounded-xl overflow-hidden">
                <button
                  onClick={() => toggle(group.key)}
                  className="w-full flex items-center gap-3 px-4 py-3 bg-zinc-900 hover:bg-zinc-800/80 transition-colors text-left"
                >
                  <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${group.dot}`} />
                  <span className={`text-xs font-semibold ${group.color} flex-1`}>{group.label}</span>
                  <span className="text-xs text-zinc-600 tabular-nums">{groupFacts.length}</span>
                  <span className="text-zinc-700 text-[10px] ml-1">{open ? '▲' : '▼'}</span>
                </button>

                {open && (
                  <div className="divide-y divide-zinc-800/40">
                    {groupFacts.length === 0 ? (
                      <div className="px-4 py-3 text-xs text-zinc-600 bg-zinc-950">No matches</div>
                    ) : (
                      groupFacts.map(f => (
                        <div key={f.ID} className="flex items-start justify-between gap-4 px-4 py-3 bg-zinc-950 hover:bg-zinc-900/40 transition-colors">
                          <div className="flex items-start gap-3 min-w-0">
                            <span className="text-zinc-700 mt-0.5 shrink-0 text-xs">•</span>
                            <span className="text-sm text-zinc-300 leading-relaxed">{f.Text}</span>
                          </div>
                          <span className="text-[11px] font-mono text-zinc-600 shrink-0 bg-zinc-900 border border-zinc-800 px-2 py-0.5 rounded mt-0.5">
                            {f.Source.split('/').pop()}
                          </span>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
