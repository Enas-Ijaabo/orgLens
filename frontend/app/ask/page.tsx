import { getFacts } from '../lib/api'
import AskTab from '../components/AskTab'

export const dynamic = 'force-dynamic'

export default async function AskPage() {
  const facts = await getFacts()
  return <AskTab hasFacts={facts.length > 0} />
}
