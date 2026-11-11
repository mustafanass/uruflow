import { useState } from 'react'

import { Pencil } from 'lucide-react'

import { useLoad } from '@/shared/hooks'
import type { AgentReporting as Settings } from '@/shared/types'
import { ErrorState, Facts, Panel, Spinner } from '@/shared/ui'

import { AgentReportingForm } from './AgentReportingForm'

export function AgentReporting() {
  const { data, error, reload, setData } = useLoad<Settings>(
    '/api/settings/agents',
  )
  const [editing, setEditing] = useState(false)
  if (error) return <ErrorState message={error} retry={reload} />
  if (!data) return <Spinner />
  return (
    <>
      <Panel
        title="Agent reporting"
        detail="How often agents report usage and how quickly they reconnect."
        actions={
          <button className="button small" onClick={() => setEditing(true)}>
            <Pencil size={14} />
            Edit
          </button>
        }
      >
        <Facts
          items={[
            [
              'Report interval',
              data.managed
                ? `Every ${data.metrics_sec} seconds`
                : `Each agent's own file (default every ${data.metrics_sec} seconds)`,
            ],
            [
              'Reconnect delay',
              data.managed
                ? `${data.reconnect_sec} seconds`
                : `Each agent's own file (default ${data.reconnect_sec} seconds)`,
            ],
            [
              'Applies to',
              `Connected agents on ${data.since} or later, immediately`,
            ],
          ]}
        />
      </Panel>
      {editing && (
        <AgentReportingForm
          settings={data}
          saved={setData}
          onClose={() => setEditing(false)}
        />
      )}
    </>
  )
}
