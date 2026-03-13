'use client'

import { useState, useRef, useEffect } from 'react'

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

interface Props {
  hasFacts: boolean
}

interface Message {
  role: 'user' | 'assistant'
  content: string
  sources?: string[]
  error?: boolean
}

export default function AskTab({ hasFacts }: Props) {
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const submit = async () => {
    const q = input.trim()
    if (!q || loading || !hasFacts) return
    setMessages(prev => [...prev, { role: 'user', content: q }])
    setInput('')
    setLoading(true)
    try {
      const res = await fetch(`${API}/api/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ q }),
      })
      if (!res.ok) throw new Error()
      const data: { answer: string; sources: { text: string; source: string }[] } = await res.json()
      const sources = data.sources?.length
        ? [...new Set(data.sources.map(s => s.source.split('/').pop()!))]
        : []
      if (!sources.length || !data.answer?.trim()) {
        setMessages(prev => [...prev, { role: 'assistant', content: 'No relevant knowledge found for this query.', error: true }])
      } else {
        setMessages(prev => [...prev, { role: 'assistant', content: data.answer, sources }])
      }
    } catch {
      setMessages(prev => [...prev, { role: 'assistant', content: 'Something went wrong. Please try again.', error: true }])
    } finally {
      setLoading(false)
      setTimeout(() => textareaRef.current?.focus(), 0)
    }
  }

  return (
    <div className="h-full flex flex-col">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        <div className="max-w-2xl mx-auto space-y-6">

          {messages.length === 0 && (
            <div className="flex flex-col items-center justify-center min-h-[40vh] text-center gap-3">
              {hasFacts ? (
                <>
                  <div className="w-12 h-12 rounded-full bg-zinc-900 border border-zinc-800 flex items-center justify-center text-xl">💬</div>
                  <p className="text-zinc-300 font-medium">Ask anything about this codebase</p>
                  <p className="text-zinc-600 text-sm max-w-xs">Answers are grounded in extracted knowledge with source citations.</p>
                  <div className="flex flex-wrap gap-2 mt-2 justify-center">
                    {['How does authentication work?', 'What are the payment limits?', 'How is traffic routed?'].map(s => (
                      <button key={s} onClick={() => { setInput(s); textareaRef.current?.focus() }}
                        className="text-xs text-zinc-500 border border-zinc-800 hover:border-zinc-700 hover:text-zinc-300 px-3 py-1.5 rounded-full transition-colors">
                        {s}
                      </button>
                    ))}
                  </div>
                </>
              ) : (
                <>
                  <div className="w-12 h-12 rounded-full bg-zinc-900 border border-zinc-800 flex items-center justify-center text-xl">📁</div>
                  <p className="text-zinc-300 font-medium">No knowledge base yet</p>
                  <p className="text-zinc-600 text-sm max-w-xs">
                    Go to the <a href="/ingest" className="text-blue-400 hover:text-blue-300">Ingest tab</a> to analyze your codebase first.
                  </p>
                </>
              )}
            </div>
          )}

          {messages.map((msg, i) => (
            <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              {msg.role === 'user' ? (
                <div className="max-w-lg bg-zinc-800 border border-zinc-700 text-zinc-100 text-sm px-4 py-3 rounded-2xl rounded-tr-md">
                  {msg.content}
                </div>
              ) : (
                <div className="flex-1 max-w-xl">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="w-5 h-5 rounded-full bg-blue-600 flex items-center justify-center text-[10px] font-bold text-white shrink-0">O</span>
                    <span className="text-xs font-medium text-zinc-500">OrgLens</span>
                  </div>
                  <div className={`text-sm leading-relaxed whitespace-pre-wrap ${msg.error ? 'text-zinc-500 italic' : 'text-zinc-200'}`}>
                    {msg.content}
                  </div>
                  {msg.sources && msg.sources.length > 0 && (
                    <div className="flex items-center gap-2 mt-3 flex-wrap">
                      <span className="text-[11px] text-zinc-600 font-medium">Sources</span>
                      {msg.sources.map(s => (
                        <span key={s} className="text-[11px] font-mono text-zinc-500 bg-zinc-900 border border-zinc-800 px-2 py-0.5 rounded">
                          {s}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}

          {loading && (
            <div className="flex justify-start">
              <div className="flex-1 max-w-xl">
                <div className="flex items-center gap-2 mb-2">
                  <span className="w-5 h-5 rounded-full bg-blue-600 flex items-center justify-center text-[10px] font-bold text-white shrink-0">O</span>
                  <span className="text-xs font-medium text-zinc-500">OrgLens</span>
                </div>
                <div className="flex gap-1.5 items-center h-5">
                  {[0, 150, 300].map(d => (
                    <span key={d} className="w-1.5 h-1.5 bg-zinc-600 rounded-full animate-bounce" style={{ animationDelay: `${d}ms` }} />
                  ))}
                </div>
              </div>
            </div>
          )}

          <div ref={bottomRef} />
        </div>
      </div>

      {/* Input */}
      <div className="shrink-0 px-6 pb-6 pt-3 border-t border-zinc-800 bg-zinc-950/80 backdrop-blur-sm">
        <div className="max-w-2xl mx-auto flex gap-3 items-end">
          <textarea
            ref={textareaRef}
            rows={1}
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit() } }}
            disabled={!hasFacts || loading}
            placeholder={hasFacts ? 'Ask a question… (Enter to send)' : 'Ingest a codebase first…'}
            className="flex-1 bg-zinc-900 border border-zinc-800 focus:border-zinc-600 text-zinc-200 text-sm rounded-xl px-4 py-3 placeholder-zinc-600 focus:outline-none resize-none disabled:opacity-40 transition-colors"
            style={{ minHeight: 48, maxHeight: 160 }}
          />
          <button
            onClick={submit}
            disabled={!hasFacts || loading || !input.trim()}
            className="px-4 py-3 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-xl transition-colors disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  )
}
