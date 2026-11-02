import { useSearchParams } from 'react-router-dom'

import { GitBranch } from 'lucide-react'

import { GraphView } from '@/features/infrastructure/GraphView'
import { useWorkspace } from '@/shared/hooks'
import type { Project } from '@/shared/types'
import { Spinner } from '@/shared/ui'

export function ProjectInfrastructure({ p }: { p: Project }) {
  const { snapshot } = useWorkspace()
  const [params, setParams] = useSearchParams()
  const view = params.get('view') === 'delivery' ? 'delivery' : 'runtime'
  function setView(value: string) {
    if (view === value) {
      return
    }
    const next = new URLSearchParams(params)
    if (value === 'runtime') {
      next.delete('view')
    } else {
      next.set('view', value)
    }
    setParams(next)
  }
  const graph = snapshot?.graphs[p.name]?.[view]
  return (
    <>
      <div className="section-heading">
        <div>
          <h2>Project infrastructure</h2>
        </div>
        <div className="segmented">
          <button
            className={view === 'runtime' ? 'selected' : ''}
            aria-pressed={view === 'runtime'}
            onClick={() => setView('runtime')}
          >
            Runtime
          </button>
          <button
            className={view === 'delivery' ? 'selected' : ''}
            aria-pressed={view === 'delivery'}
            onClick={() => setView('delivery')}
          >
            <GitBranch size={14} />
            Delivery
          </button>
        </div>
      </div>
      {graph ? (
        <GraphView graph={graph} scope={p.name + '/' + view} project={p} />
      ) : (
        <Spinner label="Building the infrastructure view…" />
      )}
    </>
  )
}
