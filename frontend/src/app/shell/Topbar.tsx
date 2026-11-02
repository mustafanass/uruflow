import { useNavigate } from 'react-router-dom'

import { CircleHelp, Menu } from 'lucide-react'

import { useWorkspace } from '@/shared/hooks'

import { ResourceSearch } from './ResourceSearch'

export function Topbar({
  openNavigation,
  navigationOpen,
  openHelp,
}: {
  openNavigation: () => void
  navigationOpen: boolean
  openHelp: () => void
}) {
  const { stale, connection } = useWorkspace()
  const navigate = useNavigate()
  const state = stale
    ? 'Data stale'
    : connection === 'live'
      ? 'Live'
      : connection === 'reconnecting'
        ? 'Reconnecting'
        : 'Connecting'
  return (
    <header className="topbar">
      <button
        className="icon-button mobile-only"
        aria-label="Open navigation"
        aria-controls="workspace-navigation"
        aria-expanded={navigationOpen}
        onClick={openNavigation}
      >
        <Menu size={21} />
      </button>
      <ResourceSearch />
      <button
        className={
          'live-indicator ' + (stale || connection !== 'live' ? 'stale' : '')
        }
        title={state}
        aria-label={state + ' updates'}
        onClick={() => navigate('/system')}
      >
        <span className="status-dot" />
        {state}
      </button>
      <button
        className="icon-button"
        aria-label="Workspace help"
        onClick={openHelp}
      >
        <CircleHelp size={19} />
      </button>
    </header>
  )
}
