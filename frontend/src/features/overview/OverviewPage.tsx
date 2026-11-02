import { Link, useSearchParams } from 'react-router-dom'
import { ArrowUpRight, Network } from 'lucide-react'
import { GraphView } from '@/features/infrastructure/GraphView'
import { AlertTable } from '@/features/alerts/AlertTable'
import { useWorkspace } from '@/shared/hooks'
import { Modal, PageHeader, Spinner } from '@/shared/ui'
import { AttentionQueue } from './AttentionQueue'
import { FleetSummary } from './FleetSummary'

export default function OverviewPage() {
  const { snapshot, canAdmin } = useWorkspace()
  const [params, setParams] = useSearchParams()
  if (!snapshot) return <Spinner />
  return (
    <div className="home-overview">
      <div className="home-heading">
        <PageHeader title="Overview" />
        <FleetSummary />
      </div>
      <AttentionQueue />
      {!snapshot.agents.length && !snapshot.projects.length && (
        <div className="onboarding">
          <Network size={28} aria-hidden="true" />
          <div>
            <h2>
              {canAdmin
                ? 'Connect your first machine'
                : 'Your workspace is ready'}
            </h2>
            <p>
              {canAdmin
                ? 'Enroll an agent, then create a project.'
                : 'Your administrator can connect machines and assign your projects.'}
            </p>
            {canAdmin && (
              <Link className="text-button" to="/agents">
                Enroll an agent
                <ArrowUpRight size={14} aria-hidden="true" />
              </Link>
            )}
          </div>
        </div>
      )}
      <GraphView graph={snapshot.fleet_graph} scope="fleet" />
      {params.get('alerts') === 'open' && (
        <Modal
          title="Active alerts"
          wide
          onClose={() =>
            setParams((current) => {
              const next = new URLSearchParams(current)
              next.delete('alerts')
              return next
            })
          }
        >
          <AlertTable alerts={snapshot.alerts} />
          <Link className="text-button" to="/activity?tab=Alerts">
            Alert history
            <ArrowUpRight size={14} aria-hidden="true" />
          </Link>
        </Modal>
      )}
    </div>
  )
}
