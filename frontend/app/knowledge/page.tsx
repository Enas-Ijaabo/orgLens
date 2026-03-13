import { getFacts } from '../lib/api'
import KnowledgeTab from '../components/KnowledgeTab'

export const dynamic = 'force-dynamic'

export default async function KnowledgePage() {
  const facts = await getFacts()
  return <KnowledgeTab initialFacts={facts} />
}
