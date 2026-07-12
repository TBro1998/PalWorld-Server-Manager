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
}

export default apiClient;
