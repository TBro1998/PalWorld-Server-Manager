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

// Mod mirrors models.Mod — the global mod library entry.
// package_name, mod_name, version and tags are backfilled from Info.json after
// a successful download. tags may arrive as null — read via `tags ?? []`.
export interface Mod {
  id: number
  workshop_id: string
  name: string
  downloaded: boolean
  download_path: string
  package_name: string
  mod_name: string
  version: string
  tags: string[] | null
  created_at: string
  updated_at: string
  // server_count is included when fetched via GET /api/mods
  server_count?: number
  // downloading is runtime state included by GET /api/mods: true while a
  // SteamCMD download for this mod is in progress. Lets the UI re-derive an
  // in-flight download after a page refresh and reattach its log stream.
  downloading?: boolean
}

// ServerMod mirrors models.ServerMod — a server's reference to a global Mod.
// The flat global-mod fields (workshop_id, name, …) are joined server-side by
// ListServerMods so the UI never needs a second round-trip.
export interface ServerMod {
  id: number
  server_id: number
  mod_id: number
  enabled: boolean
  deployed_version: string
  // Fields joined from the global Mod:
  workshop_id: string
  name: string
  mod_name: string
  package_name: string
  version: string
  tags: string[] | null
  downloaded: boolean
  // True when global library has a newer version than deployed_version.
  version_mismatch: boolean
  created_at: string
  updated_at: string
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
  // Server info from the reachability probe; present only when reachable. The
  // Overview reuses this instead of issuing a separate /rest/info request.
  info?: PalInfo
}

// --- Workshop browser (GET /api/steam/workshop/search + /dependencies) ---

// WorkshopItem is a single search result from IPublishedFileService/QueryFiles.
export interface WorkshopItem {
  workshop_id: string
  title: string
  description: string   // short description; may be empty
  preview_url: string   // thumbnail image URL
  author: string        // Steam display name / steam ID of creator
  subscriptions: number
  views: number
  time_updated: number  // Unix timestamp
  tags: string[]
}

// WorkshopDep is a lightweight entry in the dependency list returned by
// ResolveDependencies. Only the fields needed for the confirmation dialog.
export interface WorkshopDep {
  workshop_id: string
  title: string
  preview_url: string
}
// These mirror the DTOs in internal/api/save_handlers.go and describe parsed
// Level.sav / Players/*.sav data (offline players included), independent of the
// live REST API.

export interface SavePlayer {
  uid: string
  instanceId: string
  name: string
  level: number
  exp: number
  guildId: string
  guildName?: string
}

export interface SavePlayers {
  players: SavePlayer[]
}

export interface SavePalTalent {
  hp: number
  melee: number
  shot: number
  defense: number
}

export interface SavePal {
  instanceId: string
  ownerUid: string
  species: string
  name: string
  level: number
  exp: number
  gender: string
  rank: number
  talent: SavePalTalent
  passives: string[]
}

export interface SavePals {
  pals: SavePal[]
}

export interface SaveGuildMember {
  uid: string
  name: string
  role: number
  lastOnline: number
}

export interface SaveGuild {
  guildId: string
  name: string
  baseCampLevel: number
  adminUid: string
  members: SaveGuildMember[]
}

export interface SaveGuilds {
  guilds: SaveGuild[]
}

export interface SaveItem {
  container: string
  slot: number
  count: number
  staticId: string
  itemType?: string
  durability?: number
  passives?: string[]
}

// Inventory is keyed by container role (e.g. "CommonContainerId").
export interface SaveInventory {
  inventory: Record<string, SaveItem[]>
}
