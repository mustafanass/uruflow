import { lazy, Suspense } from 'react'
import { Link, Route, Routes } from 'react-router-dom'

import { PageLoading } from './shell/PageLoading'
import { LegacyRedirect } from './LegacyRedirect'

const Overview = lazy(() => import('@/features/overview/OverviewPage'))
const Projects = lazy(() => import('@/features/projects/ProjectsPage'))
const ProjectDetail = lazy(
  () => import('@/features/projects/ProjectDetailPage'),
)
const Agents = lazy(() => import('@/features/agents/AgentsPage'))
const AgentDetail = lazy(() => import('@/features/agents/AgentDetailPage'))
const ContainerDetail = lazy(
  () => import('@/features/containers/ContainerDetailPage'),
)
const ReleaseDetail = lazy(
  () => import('@/features/releases/ReleaseDetailPage'),
)
const Activity = lazy(() => import('@/features/activity/ActivityPage'))
const System = lazy(() => import('@/features/system/SystemPage'))
const Settings = lazy(() => import('@/features/settings/SettingsPage'))
export function AppRoutes() {
  return (
    <Suspense fallback={<PageLoading />}>
      <div className="page-view">
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/infrastructure" element={<LegacyRedirect to="/" />} />
          <Route path="/projects" element={<Projects />} />
          <Route path="/projects/:name" element={<ProjectDetail />} />
          <Route path="/agents" element={<Agents />} />
          <Route path="/agents/:id" element={<AgentDetail />} />
          <Route
            path="/containers"
            element={<LegacyRedirect to="/agents" tab="Containers" />}
          />
          <Route path="/containers/:agent/:id" element={<ContainerDetail />} />
          <Route
            path="/releases"
            element={<LegacyRedirect to="/projects" tab="Releases" />}
          />
          <Route path="/releases/:id" element={<ReleaseDetail />} />
          <Route
            path="/registry"
            element={
              <LegacyRedirect to="/system" tab="Server" anchor="#registry" />
            }
          />
          <Route
            path="/alerts"
            element={<LegacyRedirect to="/activity" tab="Alerts" />}
          />
          <Route path="/activity" element={<Activity />} />
          <Route path="/system" element={<System />} />
          <Route path="/settings/:section?" element={<Settings />} />
          <Route
            path="*"
            element={
              <div className="empty">
                <h1>Page not found</h1>
                <Link to="/">Back to overview</Link>
              </div>
            }
          />
        </Routes>
      </div>
    </Suspense>
  )
}
