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
  launch_args: string
  installed: boolean
  created_at: string
  updated_at: string
}

export interface CreateServerData {
  name: string
  installPath?: string
}

// LaunchArgs mirrors the backend palconfig.LaunchArgs JSON shape.
export interface LaunchArgs {
  port?: number
  players?: number
  usePerfThreads?: boolean
  noAsyncLoadingThread?: boolean
  useMultithreadForDS?: boolean
  numberOfWorkerThreadsServer?: number
  publicLobby?: boolean
  publicIP?: string
  publicPort?: number
  logFormat?: string
}

export interface UpdateServerData {
  name?: string
  port?: number
  queryPort?: number
  rconPort?: number
  rconEnabled?: boolean
  installPath?: string
  launchArgs?: LaunchArgs
}

export type ConfigParamType = 'bool' | 'int' | 'float' | 'string' | 'enum' | 'raw'

export interface ConfigParamDef {
  key: string
  type: ConfigParamType
  default: string
  category: string
  options?: string[]
}

export interface ServerConfig {
  settings: Record<string, string>
  launchArgs: LaunchArgs
  raw: string
  installed: boolean
}

export interface UpdateServerConfigData {
  settings?: Record<string, string>
  raw?: string
  launchArgs?: LaunchArgs
}
