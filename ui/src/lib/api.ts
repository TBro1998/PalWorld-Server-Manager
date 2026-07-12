import axios from 'axios';

const API_BASE_URL = '/api';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
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
import type { Server, CreateServerData, UpdateServerData } from '@/types/server'

export const serversApi = {
  list: () => apiClient.get<Server[]>('/servers'),
  create: (data: CreateServerData) => apiClient.post<Server>('/servers', data),
  get: (id: number) => apiClient.get<Server>(`/servers/${id}`),
  update: (id: number, data: UpdateServerData) => apiClient.put<Server>(`/servers/${id}`, data),
  delete: (id: number) => apiClient.delete(`/servers/${id}`),
  install: (id: number) => apiClient.post(`/servers/${id}/install`),
  start: (id: number) => apiClient.post(`/servers/${id}/start`),
  stop: (id: number) => apiClient.post(`/servers/${id}/stop`),
  restart: (id: number) => apiClient.post(`/servers/${id}/restart`),
}

export default apiClient;
