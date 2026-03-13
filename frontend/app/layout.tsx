import type { Metadata } from 'next'
import { Geist, Geist_Mono } from 'next/font/google'
import './globals.css'
import NavBar from './components/NavBar'

const geistSans = Geist({ variable: '--font-geist-sans', subsets: ['latin'] })
const geistMono = Geist_Mono({ variable: '--font-geist-mono', subsets: ['latin'] })

export const metadata: Metadata = {
  title: 'OrgLens — Codebase knowledge, instantly queryable',
  description: 'Extract, embed, and query knowledge from your codebase using Amazon Nova AI.',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased bg-zinc-950 text-zinc-100 h-screen flex flex-col overflow-hidden`}>
        <NavBar />
        <main className="flex-1 min-h-0">
          {children}
        </main>
      </body>
    </html>
  )
}
