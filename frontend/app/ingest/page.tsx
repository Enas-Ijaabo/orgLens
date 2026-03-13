import { getIngestStatus } from '../lib/api'
import IngestTab from '../components/IngestTab'

export const dynamic = 'force-dynamic'

export default async function IngestPage() {
  const status = await getIngestStatus()
  return <IngestTab initialStatus={status} />
}
