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

// Response interceptor for error handling
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

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
  // Relative URL for EventSource. EventSource cannot set Authorization headers;
  // these endpoints currently require no JWT. If auth is enabled later, pass the
  // token via a query param here instead (not implemented for now).
  logStreamUrl: (id: number, kind: LogKind = 'server') =>
    `/api/servers/${id}/logs/stream?kind=${kind}`,

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
}

// --- Mod management ---
// list/add/remove/toggle are synchronous CRUD; update triggers the async
// SteamCMD download + deploy + config write (progress observed via the existing
// steamcmd log stream, serversApi.logStreamUrl(id, 'steamcmd')).
export const modsApi = {
  list: (id: number) => apiClient.get<{ mods: Mod[] }>(`/api/servers/${id}/mods`),
  add: (id: number, data: { workshopId: string; name?: string }) =>
    apiClient.post<Mod>(`/api/servers/${id}/mods`, data),
  remove: (id: number, modId: number) =>
    apiClient.delete(`/api/servers/${id}/mods/${modId}`),
  toggle: (id: number, modId: number) =>
    apiClient.put<Mod>(`/api/servers/${id}/mods/${modId}/toggle`),
  update: (id: number) => apiClient.post(`/api/servers/${id}/mods/update`),
}

// --- Steam account (global) ---
// status reports the configured username and whether a cached SteamCMD session
// is ready; login runs `steamcmd +login` server-side and classifies the result.
// The password is only used for the login request and is never stored by the
// backend (not in the DB, logs, or responses).
export const steamApi = {
  status: () =>
    apiClient.get<{ username: string; sessionReady: boolean }>('/api/steam/status'),
  login: (data: { username: string; password: string; guardCode?: string }) =>
    apiClient.post<{
      result: 'success' | 'needGuard' | 'badCredentials' | 'error'
      message?: string
      log?: string
    }>('/api/steam/login', data),
}

export default apiClient;
