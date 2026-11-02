import { RelativeTime } from '@/shared/ui/RelativeTime'
import { Cpu, HardDrive, MemoryStick } from 'lucide-react'

import { type Metrics } from '@/shared/types'
import { available, bytes } from '@/shared/utils'

export function HostResources({
  metrics,
  compact = false,
  label = 'Host resources',
}: {
  metrics?: Metrics
  compact?: boolean
  label?: string
}) {
  const items = [
    {
      name: 'CPU',
      icon: Cpu,
      valid: available(metrics, 'cpu'),
      value: metrics?.cpu_percent || 0,
      text: metrics ? `${metrics.cpu_percent.toFixed(1)}%` : 'Unavailable',
    },
    {
      name: 'Memory',
      icon: MemoryStick,
      valid: available(metrics, 'memory'),
      value: metrics?.memory_percent || 0,
      text: metrics
        ? `${bytes(metrics.memory_used)} / ${bytes(metrics.memory_total)}`
        : 'Unavailable',
    },
    {
      name: 'Root disk',
      icon: HardDrive,
      valid: available(metrics, 'disk'),
      value: metrics?.disk_percent || 0,
      text: metrics
        ? `${bytes(metrics.disk_used)} / ${bytes(metrics.disk_total)}`
        : 'Unavailable',
    },
  ]
  return (
    <div className={'host-resources ' + (compact ? 'compact' : '')}>
      <div className="resource-heading">
        <h3>{label}</h3>
        <span>
          <RelativeTime value={metrics?.collected_at} />
        </span>
      </div>
      <div className="resource-meters">
        {items.map((item) => (
          <div className="resource-meter" key={item.name}>
            <div className="meter-label">
              <span>
                <item.icon size={15} />
                {item.name}
              </span>
              <strong>{item.valid ? item.text : 'Unavailable'}</strong>
            </div>
            <div className="meter-track">
              <span
                className={item.value > 90 ? 'pressure' : ''}
                style={{
                  width: item.valid
                    ? `${Math.max(0, Math.min(100, item.value))}%`
                    : '0%',
                }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
