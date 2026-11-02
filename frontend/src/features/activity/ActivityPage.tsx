import { withTab } from '@/shared/utils/url'
import { Navigate, useSearchParams } from 'react-router-dom'
import { AlertTable } from '@/features/alerts/AlertTable'
import { useAlertHistory } from '@/features/alerts/useAlertHistory'
import { useWorkspace } from '@/shared/hooks'
import {
  ErrorState,
  PageHeader,
  Panel,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'
import { ActivityTimeline } from './ActivityTimeline'

export default function ActivityPage() {
  const { snapshot } = useWorkspace()
  const history = useAlertHistory()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'Alerts' ? 'Alerts' : 'Timeline'
  const state = params.get('state') || ''
  if (params.get('tab') === 'Audit')
    return <Navigate replace to="/system?tab=Audit%20log" />
  return (
    <>
      <PageHeader title="Activity" />
      <Tabs
        items={['Timeline', 'Alerts']}
        value={tab}
        onChange={(value) =>
          setParams((current) => withTab(current, value, 'Timeline'))
        }
      />
      <TabPanel value={tab}>
        {history.error && (
          <ErrorState message={history.error} retry={history.reload} />
        )}
        {!snapshot ? (
          <Spinner />
        ) : (
          <Panel>
            {tab === 'Timeline' ? (
              <ActivityTimeline alerts={history.rows} />
            ) : (
              <>
                <div className="context-filters">
                  <label>
                    Status
                    <select
                      aria-label="Alert status"
                      name="state"
                      value={state}
                      onChange={(event) =>
                        setParams(
                          (current) => {
                            const next = withTab(current, 'Alerts')
                            if (event.target.value)
                              next.set('state', event.target.value)
                            else next.delete('state')
                            next.delete('alerts-page.page')
                            return next
                          },
                          { replace: true },
                        )
                      }
                    >
                      <option value="">All alerts</option>
                      <option value="active">Active</option>
                      <option value="resolved">Resolved</option>
                    </select>
                  </label>
                </div>
                <AlertTable
                  alerts={history.rows.filter(
                    (alert) =>
                      !state || (state === 'resolved') === !!alert.resolved,
                  )}
                  onResolved={history.reload}
                />
              </>
            )}
          </Panel>
        )}
      </TabPanel>
    </>
  )
}
