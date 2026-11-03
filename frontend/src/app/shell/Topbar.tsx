import { CircleHelp, Menu } from 'lucide-react'

import { AccountMenu } from '@/features/account/AccountMenu'

import { ResourceSearch } from './ResourceSearch'

export function Topbar({
  openNavigation,
  navigationOpen,
  openHelp,
  signOut,
}: {
  openNavigation: () => void
  navigationOpen: boolean
  openHelp: () => void
  signOut: () => void
}) {
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
        className="icon-button"
        aria-label="Workspace help"
        onClick={openHelp}
      >
        <CircleHelp size={19} />
      </button>
      <AccountMenu signOut={signOut} />
    </header>
  )
}
