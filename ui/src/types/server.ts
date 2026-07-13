// LogKind selects which log stream to read/stream for a server.
// 'server'  -> the running Palworld process's stdout/stderr
// 'steamcmd' -> SteamCMD install/update output
export type LogKind = 'server' | 'steamcmd'

export interface Server {
  id: number
  name: string
  install_path: string
  status: 'stopped' | 'running' | 'installing' | 'error'
  last_error?: string
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
  queryPort?: number
  logFormat?: string
}

export interface UpdateServerData {
  name?: string
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

// --- Palworld REST API shapes ---
// These mirror the backend palapi structs (internal/palapi/client.go), which in
// turn mirror the official Palworld REST API responses. Field names match the
// JSON exactly so responses map 1:1.

// PalInfo mirrors GET /v1/api/info.
export interface PalInfo {
  version: string
  servername: string
  description: string
  worldguid: string
}

// PalMetrics mirrors GET /v1/api/metrics.
export interface PalMetrics {
  serverfps: number
  currentplayernum: number
  serverframetime: number
  maxplayernum: number
  uptime: number
  days: number
}

// PalPlayer mirrors one entry of GET /v1/api/players.
export interface PalPlayer {
  name: string
  accountName: string
  playerId: string
  userId: string
  ip: string
  ping: number
  location_x: number
  location_y: number
  level: number
  building_count: number
}

// PalPlayers mirrors GET /v1/api/players.
export interface PalPlayers {
  players: PalPlayer[]
}

// RestStatus mirrors GET /api/servers/:id/rest/status. The AdminPassword is
// never included (surfaced by the backend on purpose).
export type RestReason =
  | ''
  | 'not_found'
  | 'not_running'
  | 'restapi_disabled'
  | 'admin_password_empty'
  | 'unreachable'

export interface RestStatus {
  enabled: boolean
  running: boolean
  reachable: boolean
  port: number
  reason: RestReason
}
