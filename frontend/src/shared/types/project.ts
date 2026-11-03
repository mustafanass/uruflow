interface Port {
  host_ip?: string
  host: number
  container: number
  protocol?: string
}
export interface Mount {
  type?: string
  source: string
  target: string
  read_only?: boolean
}
export interface Service {
  name: string
  image?: string
  git_url?: string
  branch?: string
  dockerfile?: string
  context?: string
  watch?: string[]
  mode?: string
  ports?: Port[]
  volumes?: Mount[]
  networks?: {
    name: string
    aliases?: string[]
  }[]
  network?: string
  depends_on?: {
    service: string
    condition: string
  }[]
  env?: Record<string, string>
  restart?: string
  resources?: {
    memory_bytes?: number
    cpus?: number
    pids?: number
  }
  healthcheck?: {
    type: string
    path?: string
    port?: number
    interval?: number
    timeout?: number
    command?: string[]
  }
}
export interface Project {
  name: string
  env: string
  builder: string
  runners: string[]
  services?: Service[]
  workflow?: string
  timeout?: number
  source: string
  git_url?: string
  branch?: string
  dockerfile?: string
  context?: string
  runtime?: {
    ports?: Port[]
    env?: Record<string, string>
    volumes?: Mount[]
    network?: string
  }
  networks?: Record<string, unknown>
  volumes?: Record<string, unknown>
  created_at: string
}
