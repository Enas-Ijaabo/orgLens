'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'

const TABS = [
  { href: '/ingest',    label: 'Ingest' },
  { href: '/knowledge', label: 'Knowledge' },
  { href: '/ask',       label: 'Ask' },
]

export default function NavBar() {
  const path = usePathname()

  return (
    <header className="shrink-0 flex items-center gap-6 px-6 border-b border-zinc-800 bg-zinc-950/90 backdrop-blur-sm">
      <Link href="/ingest" className="flex items-center gap-2 py-4">
        <span className="text-white font-semibold text-sm tracking-tight">OrgLens</span>
        <span className="text-[10px] font-medium text-zinc-600 bg-zinc-800 px-1.5 py-0.5 rounded uppercase tracking-wider">Nova</span>
      </Link>

      <nav className="flex h-full">
        {TABS.map(t => {
          const active = path === t.href || path.startsWith(t.href + '/')
          return (
            <Link
              key={t.href}
              href={t.href}
              className={`flex items-center px-4 py-4 text-sm font-medium border-b-2 transition-colors ${
                active
                  ? 'border-blue-500 text-white'
                  : 'border-transparent text-zinc-500 hover:text-zinc-300'
              }`}
            >
              {t.label}
            </Link>
          )
        })}
      </nav>

      <div className="ml-auto text-[11px] text-zinc-600 hidden sm:block">
        Powered by Amazon Nova
      </div>
    </header>
  )
}
