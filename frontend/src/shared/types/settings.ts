export interface WebhookRejection {
  at: string
  provider: string
  reason: string
}
export interface WebhookCertificate {
  subject: string
  names: string[]
  expires_at: string
}
export interface WebhookSettings {
  revision: number
  enabled: boolean
  endpoint: string
  host: string
  port: number
  path: string
  tls: boolean
  certificate?: WebhookCertificate
  secret_hint: string
  secret_at: string
  previous_until?: string
  rejection?: WebhookRejection
}
export interface WebhookSecret {
  secret: string
  settings: WebhookSettings
}
export interface WebhookDelivery {
  provider: string
  delivery_id: string
  received_at: string
  repository: string
  branch: string
  commit: string
  result: string
  detail: string
}
export interface RegistrySettings {
  revision: number
  retain_releases: number
  password_at: string
}
export interface AgentReporting {
  revision: number
  managed: boolean
  metrics_sec: number
  reconnect_sec: number
}
