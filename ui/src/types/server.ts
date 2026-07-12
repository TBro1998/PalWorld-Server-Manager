export interface Server {
  id: number
  name: string
  install_path: string
  port: number
  query_port: number
  rcon_port: number
  rcon_enabled: boolean
  status: 'stopped' | 'running' | 'installing' | 'error'
  pid: number
  created_at: string
  updated_at: string
}

export interface CreateServerData {
  name: string
  installPath?: string
}

export interface UpdateServerData {
  name?: string
  port?: number
  queryPort?: number
  rconPort?: number
  rconEnabled?: boolean
}
