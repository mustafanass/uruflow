import { withTab } from '@/shared/utils/url'
import { useEffect } from 'react'
import { Navigate, useLocation, useSearchParams } from 'react-router-dom'
import { AccessManagement } from '@/features/system/access/AccessManagement'
import { RegistryCatalog } from '@/features/registry/RegistryCatalog'
import { useWorkspace } from '@/shared/hooks'
import { PageHeader, Tabs, TabPanel } from '@/shared/ui'
import { AuditLog } from './AuditLog'
import { SystemOverview } from './SystemOverview'

export default function SystemPage() {
  const { canAdmin, fleetAccess } = useWorkspace()
  const [params, setParams] = useSearchParams()
  const { hash } = useLocation()
  const tabs = canAdmin ? ['Server', 'Access', 'Audit log'] : ['Server']
  const tab = tabs.includes(params.get('tab') || '')
    ? params.get('tab')!
    : 'Server'
  useEffect(() => {
    if (tab !== 'Server' || hash !== '#registry') return
    const frame = requestAnimationFrame(() =>
      document.getElementById('registry')?.scrollIntoView(),
    )
    return () => cancelAnimationFrame(frame)
  }, [hash, tab])
  if (params.get('tab') === 'Settings')
    return <Navigate replace to="/settings" />
  const content =
    tab === 'Access' ? (
      <AccessManagement />
    ) : tab === 'Audit log' ? (
      <AuditLog />
    ) : (
      <>
        <SystemOverview />
        {fleetAccess && (
          <section id="registry">
            <RegistryCatalog />
          </section>
        )}
      </>
    )
  return (
    <>
      <PageHeader title="System" />
      {tabs.length > 1 ? (
        <>
          <Tabs
            items={tabs}
            value={tab}
            onChange={(value) =>
              setParams((current) => withTab(current, value, 'Server'))
            }
          />
          <TabPanel value={tab}>{content}</TabPanel>
        </>
      ) : (
        content
      )}
    </>
  )
}
