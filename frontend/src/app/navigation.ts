import {
  Activity,
  Boxes,
  LayoutDashboard,
  Server,
  Settings,
} from 'lucide-react'

export const navigation = [
  { to: '/', label: 'Overview', icon: LayoutDashboard },
  { to: '/projects', label: 'Projects', icon: Boxes },
  { to: '/agents', label: 'Agents', icon: Server },
  { to: '/activity', label: 'Activity', icon: Activity },
  { to: '/system', label: 'System', icon: Settings },
]
export function navigationSection(path: string) {
  if (/^\/(projects|releases)(\/|$)/.test(path)) return '/projects'
  if (/^\/(agents|containers)(\/|$)/.test(path)) return '/agents'
  if (/^\/(system|registry)(\/|$)/.test(path)) return '/system'
  if (/^\/(activity|alerts)(\/|$)/.test(path)) return '/activity'
  return '/'
}
export function pageTitle(path: string) {
  return navigation.find((item) => item.to === navigationSection(path))!.label
}
