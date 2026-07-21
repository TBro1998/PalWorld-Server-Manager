import axios from 'axios';

export const apiClient = axios.create({
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor to add JWT token
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for error handling.
// Only redirect to /login on 401 for protected API calls — never for the auth
// endpoints themselves, so login/setup can surface their own error messages.
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      const url: string = error.config?.url ?? '';
      const isAuthEndpoint = url.includes('/api/auth/');
      if (!isAuthEndpoint) {
        localStorage.removeItem('token');
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);

// Returns the stored JWT token (or empty string for SSE URL params).
export function getToken(): string {
  return typeof window !== 'undefined' ? (localStorage.getItem('token') ?? '') : '';
}

// --- Authentication ---
export const authApi = {
  // Returns { configured: boolean } — whether a password has been set.
  status: () => apiClient.get<{ configured: boolean }>('/api/auth/status'),
  // Sets the initial password; only works when configured=false. Returns { token }.
  setup: (password: string) =>
    apiClient.post<{ token: string }>('/api/auth/setup', { password }),
  // Validates the password and returns { token }.
  login: (password: string) =>
    apiClient.post<{ token: string }>('/api/auth/login', { password }),
};

// Server API functions
import type {
  Server,
  CreateServerData,
  UpdateServerData,
  ServerConfig,
  UpdateServerConfigData,
  ConfigParamDef,
  LogKind,
  RestStatus,
  PalInfo,
  PalMetrics,
  PalPlayers,
  SavePlayers,
  SaveGuilds,
  SavePals,
  SaveInventory,
  Mod,
  ServerMod,
  WorkshopItem,
  WorkshopDep,
  Backup,
  BackupScope,
  BackupSchedule,
} from '@/types/server'

export const serversApi = {
  list: () => apiClient.get<Server[]>('/api/servers'),
  create: (data: CreateServerData) => apiClient.post<Server>('/api/servers', data),
  get: (id: number) => apiClient.get<Server>(`/api/servers/${id}`),
  update: (id: number, data: UpdateServerData) => apiClient.put<Server>(`/api/servers/${id}`, data),
  delete: (id: number) => apiClient.delete(`/api/servers/${id}`),
  install: (id: number) => apiClient.post(`/api/servers/${id}/install`),
  start: (id: number) => apiClient.post(`/api/servers/${id}/start`),
  stop: (id: number) => apiClient.post(`/api/servers/${id}/stop`),
  restart: (id: number) => apiClient.post(`/api/servers/${id}/restart`),
  getConfig: (id: number) => apiClient.get<ServerConfig>(`/api/servers/${id}/config`),
  updateConfig: (id: number, data: UpdateServerConfigData) =>
    apiClient.put(`/api/servers/${id}/config`, data),
  configSchema: () => apiClient.get<{ params: ConfigParamDef[] }>('/api/config/schema'),
  // kind selects which log stream to read: 'server' (the running game process)
  // or 'steamcmd' (install/update output). Defaults to 'server'.
  getLogs: (id: number, kind: LogKind = 'server', lines = 200) =>
    apiClient.get<{ serverId: number; kind: LogKind; logs: string[] }>(`/api/servers/${id}/logs`, {
      params: { kind, lines },
    }),
  // Relative URL for EventSource. EventSource cannot set custom headers, so the
  // JWT is passed as a ?token= query parameter which the backend middleware accepts.
  logStreamUrl: (id: number, kind: LogKind = 'server') =>
    `/api/servers/${id}/logs/stream?kind=${kind}&token=${encodeURIComponent(getToken())}`,

  // --- Palworld REST API proxy ---
  // The backend forwards these to the game server's official REST API after
  // reading its INI for port/AdminPassword. status is always structured; the
  // rest require the server to be running with REST enabled (else 4xx).
  restStatus: (id: number) => apiClient.get<RestStatus>(`/api/servers/${id}/rest/status`),
  restInfo: (id: number) => apiClient.get<PalInfo>(`/api/servers/${id}/rest/info`),
  restMetrics: (id: number) => apiClient.get<PalMetrics>(`/api/servers/${id}/rest/metrics`),
  restPlayers: (id: number) => apiClient.get<PalPlayers>(`/api/servers/${id}/rest/players`),
  restSettings: (id: number) =>
    apiClient.get<Record<string, unknown>>(`/api/servers/${id}/rest/settings`),
  restAnnounce: (id: number, data: { message: string }) =>
    apiClient.post(`/api/servers/${id}/rest/announce`, data),
  restKick: (id: number, data: { userid: string; message: string }) =>
    apiClient.post(`/api/servers/${id}/rest/kick`, data),
  restBan: (id: number, data: { userid: string; message: string }) =>
    apiClient.post(`/api/servers/${id}/rest/ban`, data),
  restUnban: (id: number, data: { userid: string }) =>
    apiClient.post(`/api/servers/${id}/rest/unban`, data),
  restSave: (id: number) => apiClient.post(`/api/servers/${id}/rest/save`),
  restShutdown: (id: number, data: { waittime: number; message: string }) =>
    apiClient.post(`/api/servers/${id}/rest/shutdown`, data),
  restStop: (id: number) => apiClient.post(`/api/servers/${id}/rest/stop`),

  // --- Save-file inspection ---
  // Parses the co-located Level.sav / Players saves (read-only). Independent of
  // the live REST API: works even when the server is stopped, as long as a save
  // exists on disk. 404 when no save is found.
  savePlayers: (id: number) =>
    apiClient.get<SavePlayers>(`/api/servers/${id}/save/players`),
  saveGuilds: (id: number) => apiClient.get<SaveGuilds>(`/api/servers/${id}/save/guilds`),
  savePals: (id: number, uid: string) =>
    apiClient.get<SavePals>(`/api/servers/${id}/save/players/${uid}/pals`),
  saveInventory: (id: number, uid: string) =>
    apiClient.get<SaveInventory>(`/api/servers/${id}/save/players/${uid}/inventory`),

  // --- Whitelist management ---
  // Reads and writes the server's whitelist.txt. The file is managed by the
  // backend (inside the server's install path) and does not require the server
  // to be running.
  getWhitelist: (id: number) =>
    apiClient.get<{ entries: string[] }>(`/api/servers/${id}/whitelist`),
  addWhitelist: (id: number, uid: string) =>
    apiClient.post<{ entries: string[] }>(`/api/servers/${id}/whitelist`, { uid }),
  removeWhitelist: (id: number, uid: string) =>
    apiClient.delete<{ entries: string[] }>(`/api/servers/${id}/whitelist`, { data: { uid } }),

  // --- Process resource stats ---
  // CPU / memory of the server's process tree. When the server is not running
  // the backend returns 200 with { running: false, reason: 'not_running' }.
  stats: (id: number) => apiClient.get<ProcessStats>(`/api/servers/${id}/stats`),
}

// --- Backup management ---
// Create/list/download/delete/restore backups of a server's save and/or config,
// plus the per-server automatic-backup schedule. Restore requires the server to
// be stopped (backend returns 409 otherwise).
export const backupsApi = {
  list: (id: number) =>
    apiClient.get<{ backups: Backup[] }>(`/api/servers/${id}/backups`),
  create: (id: number, scope: BackupScope) =>
    apiClient.post<Backup>(`/api/servers/${id}/backups`, { scope }),
  remove: (id: number, backupId: number) =>
    apiClient.delete(`/api/servers/${id}/backups/${backupId}`),
  restore: (id: number, backupId: number) =>
    apiClient.post(`/api/servers/${id}/backups/${backupId}/restore`),
  // Downloads the zip as a blob so the browser can save it via an object URL.
  download: (id: number, backupId: number) =>
    apiClient.get<Blob>(`/api/servers/${id}/backups/${backupId}/download`, {
      responseType: 'blob',
    }),
  getSchedule: (id: number) =>
    apiClient.get<BackupSchedule>(`/api/servers/${id}/backups/schedule`),
  updateSchedule: (id: number, data: Omit<BackupSchedule, 'server_id'>) =>
    apiClient.put<BackupSchedule>(`/api/servers/${id}/backups/schedule`, data),
}

// --- Global mod library ---
// CRUD for the shared mod library. Downloads are triggered separately.
export const globalModsApi = {
  list: () => apiClient.get<{ mods: Mod[] }>('/api/mods'),
  add: (data: { workshopId: string; name?: string }) =>
    apiClient.post<Mod>('/api/mods', data),
  remove: (modId: number) => apiClient.delete(`/api/mods/${modId}`),
  download: (modId: number) => apiClient.post(`/api/mods/${modId}/download`),
  // Historical download log lines for a mod, used to backfill the log panel
  // when (re)attaching to an in-progress download after a page refresh.
  getLogs: (modId: number, lines = 200) =>
    apiClient.get<{ modId: number; logs: string[] }>(`/api/mods/${modId}/logs`, {
      params: { lines },
    }),
  // Relative URL for EventSource: per-mod download progress stream.
  // Each mod uses its own ID as the stream key so concurrent downloads stay independent.
  logStreamUrl: (modId: number) => `/api/mods/${modId}/logs/stream?token=${encodeURIComponent(getToken())}`,
}

// --- Server mod references ---
// Links between servers and the global library. deploy copies staged files into
// the server's Mods/Workshop directory. Progress observed via steamcmd log stream.
export const modsApi = {
  list: (serverId: number) =>
    apiClient.get<{ mods: ServerMod[] }>(`/api/servers/${serverId}/mods`),
  link: (serverId: number, data: { modId?: number; workshopId?: string }) =>
    apiClient.post<ServerMod>(`/api/servers/${serverId}/mods`, data),
  unlink: (serverId: number, serverModId: number) =>
    apiClient.delete(`/api/servers/${serverId}/mods/${serverModId}`),
  toggle: (serverId: number, serverModId: number) =>
    apiClient.put<ServerMod>(`/api/servers/${serverId}/mods/${serverModId}/toggle`),
  deploy: (serverId: number) =>
    apiClient.post(`/api/servers/${serverId}/mods/deploy`),
}

// --- Steam account (global) ---
// status reports the configured username and whether a cached SteamCMD session
// is ready; login runs `steamcmd +login` server-side and classifies the result
// (returned synchronously). The password is only used for the login request and
// is never stored by the backend (not in the DB, logs, or responses).
// The live steamcmd output is delivered separately over SSE via
// loginStreamUrl() — open that EventSource before calling login() to catch the
// first lines.
export const steamApi = {
  status: () =>
    apiClient.get<{ username: string; sessionReady: boolean; webApiKeyConfigured: boolean }>('/api/steam/status'),
  login: (data: { username: string; password: string; guardCode?: string }) =>
    apiClient.post<{
      result: 'success' | 'needGuard' | 'badCredentials' | 'error'
      message?: string
    }>('/api/steam/login', data),
  // Relative URL for EventSource. Emits named `log` events, one per steamcmd
  // output line, on the global login stream (no server ID).
  loginStreamUrl: () => `/api/steam/logs/stream?token=${encodeURIComponent(getToken())}`,

  // --- Workshop search (proxies Steam Web API; key stays server-side) ---
  // search queries IPublishedFileService/QueryFiles for Palworld mods.
  // cursor: omit or "*" for the first page; pass nextCursor to paginate.
  workshopSearch: (params: { q?: string; cursor?: string; num?: number }) =>
    apiClient.get<{ items: WorkshopItem[]; next_cursor: string; total: number }>(
      '/api/steam/workshop/search',
      { params: { q: params.q ?? '', cursor: params.cursor ?? '*', num: params.num ?? 20 } },
    ),
  // workshopDependencies resolves all transitive Steam Workshop deps of a mod.
  // Returns a flat deduplicated list (not including the mod itself).
  workshopDependencies: (workshopId: string) =>
    apiClient.get<{ dependencies: WorkshopDep[] }>(
      `/api/steam/workshop/mods/${workshopId}/dependencies`,
    ),
  // setWebApiKey persists (or clears, when key="") the Steam Web API key.
  // Returns {configured: bool}. The key is never echoed back.
  setWebApiKey: (key: string) =>
    apiClient.post<{ configured: boolean }>('/api/steam/webapi-key', { key }),
}

import type { VersionInfo, CheckResult, SystemSettings, UpdateStatus, HostStats, ProcessStats } from '@/types/system'

// --- System version & self-update ---
export const systemApi = {
  // Returns the running binary's build metadata (version / buildTime / gitCommit).
  version: () => apiClient.get<VersionInfo>('/api/system/version'),
  // Queries GitHub for the latest release. Pass cached=true to read the
  // in-memory cache without hitting GitHub.
  checkUpdate: (cached = false) =>
    apiClient.get<CheckResult>('/api/system/update/check', {
      params: cached ? { cached: '1' } : undefined,
    }),
  // Triggers async download + replace + restart. Subscribe to updateStreamUrl()
  // before calling this so you don't miss early progress events.
  applyUpdate: () => apiClient.post<{ message: string }>('/api/system/update/apply'),
  // Returns the current update phase (idle / downloading / restarting / error).
  // Poll on page mount to restore progress UI after a navigation or refresh.
  updateStatus: () => apiClient.get<UpdateStatus>('/api/system/update/status'),
  // Relative URL for EventSource.  Open before calling applyUpdate().
  updateStreamUrl: () => `/api/system/update/stream?token=${encodeURIComponent(getToken())}`,
  // Returns persisted system settings (download_mirror).
  getSettings: () => apiClient.get<SystemSettings>('/api/system/settings'),
  // Persists system settings.
  setSettings: (data: Partial<SystemSettings>) =>
    apiClient.put<SystemSettings>('/api/system/settings', data),
  // Returns whole-host resource usage (CPU / memory / data disk). Poll ~5s.
  stats: () => apiClient.get<HostStats>('/api/system/stats'),
}

export default apiClient;
