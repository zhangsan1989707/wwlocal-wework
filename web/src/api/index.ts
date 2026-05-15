import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

let reloading = false
api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401 && !reloading) {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      reloading = true
      window.location.reload()
    }
    return Promise.reject(error)
  }
)

export const authAPI = {
  login: (data: { username: string; password: string }) => api.post('/auth/login', data),
}

export const healthAPI = {
  check: () => api.get('/health'),
}

export const logAPI = {
  query: (data: any) => api.post('/logs/query', data),
  getFeatures: () => api.get('/logs/features'),
  getTimeRange: () => api.get('/logs/time-range'),
  getFieldPaths: () => api.get('/logs/field-paths'),
}

export const syncAPI = {
  sync: (data: any) => api.post('/logs/sync', data),
  cancel: () => api.post('/logs/sync/cancel'),
  status: () => api.get('/logs/sync/status'),
}

export const keyAPI = {
  list: () => api.get('/keys'),
  add: (data: any) => api.post('/keys', data),
  activate: (data: any) => api.put('/keys/activate', data),
  test: (version: string) => api.get('/keys/test', { params: { version } }),
}

export const schedulerAPI = {
  start: (data?: any) => api.post('/scheduler/start', data),
  stop: () => api.post('/scheduler/stop'),
  status: () => api.get('/scheduler/status'),
  incrementalSync: (data: any) => api.post('/scheduler/sync', data),
  setInterval: (data: { interval: string }) => api.put('/scheduler/interval', data),
}

export const contactAPI = {
  list: (params: any) => api.get('/contacts', { params }),
  getDepartments: () => api.get('/contacts/departments'),
  getDeptTree: () => api.get('/contacts/tree'),
  getDeptMembers: (deptId: number, params: any) => api.get(`/contacts/departments/${deptId}/members`, { params }),
  getContact: (userId: string) => api.get(`/contacts/${userId}`),
  getNames: (user_ids: string[]) => api.post('/contacts/names', { user_ids }),
  sync: () => api.post('/contacts/sync'),
  syncIncremental: () => api.post('/contacts/sync/incremental'),
  cancel: () => api.post('/contacts/sync/cancel'),
  status: () => api.get('/contacts/sync/status'),
}

export const dashboardAPI = {
  getOverview: () => api.get('/dashboard/overview'),
  getInactiveUsers: (params?: { range?: string; dept_id?: number; min_inactive_days?: number }) => api.get('/dashboard/inactive-users', { params }),
}

export const systemAPI = {
  getStatus: () => api.get('/system/status'),
}

export const syncHistoryAPI = {
  list: (params: any) => api.get('/sync-history', { params }),
}

export const syncFeatureAPI = {
  list: () => api.get('/sync-features'),
  update: (data: any) => api.put('/sync-features', data),
}

export const adminOperLogAPI = {
  query: (params: any) => api.get('/admin-oper-logs', { params }),
  sync: (data: any) => api.post('/admin-oper-logs/sync', data),
  syncStatus: () => api.get('/admin-oper-logs/sync/status'),
}

export default api
