import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { Agent, Project } from '@/shared/types'
import { Badge, DataTable, Panel } from '@/shared/ui'
import { services } from '@/shared/utils'

export function AgentProjects({
  projects,
  a,
}: {
  projects: Project[]
  a: Agent
}) {
  const { snapshot } = useWorkspace()
  if (!snapshot) {
    return null
  }
  return (
    <Panel>
      <DataTable
        scope="agent-projects"
        rows={projects}
        rowKey={(p) => p.name}
        rowHref={(p) => '/projects/' + segment(p.name)}
        searchText={(p) => p.name + ' ' + p.env}
        empty="No project assignments"
        columns={[
          {
            key: 'project',
            title: 'Project',
            render: (p) => <strong>{p.name}</strong>,
            sort: (p) => p.name,
          },
          {
            key: 'assignment',
            title: 'Assigned role',
            render: (p) =>
              [
                p.builder === a.id ? 'builder' : '',
                p.runners?.includes(a.id) ? 'runner' : '',
              ]
                .filter(Boolean)
                .join(' + '),
          },
          {
            key: 'runtime',
            title: 'Runtime',
            render: (p) => <Badge value={snapshot.project_states[p.name]} />,
          },
          {
            key: 'services',
            title: 'Services',
            render: (p) => services(p).length,
          },
        ]}
      />
    </Panel>
  )
}
