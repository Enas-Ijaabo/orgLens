import { redirect } from 'next/navigation'
import { getFacts } from './lib/api'

export const dynamic = 'force-dynamic'

export default async function Home() {
  const facts = await getFacts()
  if (facts.length > 0) redirect('/knowledge')
  redirect('/ingest')
}
