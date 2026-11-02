export interface User {
  projects: string[]
  username: string
  role: 'admin' | 'operator' | 'viewer'
  created_at: string
}
export interface Session {
  authenticated: boolean
  setup_required?: boolean
  user?: User
  csrf_token?: string
}
