import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    console.error('API Error:', error)
    return Promise.reject(error)
  }
)

export interface ApiResponse<T = any> {
  code: number
  msg: string
  data: T
}

export const healthAPI = {
  check: () => api.get('/health'),
}

export const logAPI = {
  query: (data: any) => api.post('/logs/query', data),
  getFeatures: () => api.get('/logs/features'),
  getTimeRange: () => api.get('/logs/time-range'),
}

export const syncAPI = {
  sync: (data: any) => api.post('/logs/sync', data),
  status: () => api.get('/logs/sync/status'),
}

export const keyAPI = {
  list: () => api.get('/keys'),
  add: (data: any) => api.post('/keys', data),
  activate: (data: any) => api.put('/keys/activate', data),
}

export default api