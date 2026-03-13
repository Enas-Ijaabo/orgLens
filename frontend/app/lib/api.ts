// Server-side API helpers — uses INTERNAL_API_URL to reach backend within Docker network.
// Falls back to localhost for local development.

const BASE = process.env.INTERNAL_API_URL ?? 'http://localhost:8080'

export interface Fact {
  ID: string
  Text: string
  Type: string
  Source: string
}

export interface FileStatus {
  file: string
  ingested: boolean
  ingested_at?: string
  facts_count?: number
}

export async function getFacts(): Promise<Fact[]> {
  try {
    const res = await fetch(`${BASE}/api/facts`, { cache: 'no-store' })
    if (!res.ok) return []
    const data = await res.json()
    return data ?? []
  } catch {
    return []
  }
}

export async function getIngestStatus(): Promise<FileStatus[]> {
  try {
    const res = await fetch(`${BASE}/api/ingest/status`, { cache: 'no-store' })
    if (!res.ok) return []
    const data = await res.json()
    return data ?? []
  } catch {
    return []
  }
}
