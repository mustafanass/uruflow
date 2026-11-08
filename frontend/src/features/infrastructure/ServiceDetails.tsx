import type { Project, Service } from '@/shared/types'
import { Code, Facts } from '@/shared/ui'
import { bytes, ports } from '@/shared/utils'

export function ServiceDetails({
  service,
  project,
}: {
  service: Service
  project: Project
}) {
  const limits = [
    service.resources?.cpus ? `${service.resources.cpus} cores` : '',
    service.resources?.memory_bytes
      ? bytes(service.resources.memory_bytes)
      : '',
    service.resources?.pids ? `${service.resources.pids} PIDs` : '',
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <>
      <h4>Service definition</h4>
      <Facts
        items={[
          ['Mode', service.mode || 'service'],
          ['Ports', ports(service.ports)],
          [
            'Source',
            <Code
              value={service.image || service.git_url || project.git_url || ''}
            />,
          ],
          ['Limits', limits || 'Not configured'],
          ['Restart', service.restart || 'unless-stopped'],
          ['Healthcheck', service.healthcheck?.type || 'Not configured'],
        ]}
      />
    </>
  )
}
