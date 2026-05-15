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

export interface LogQueryParams {
  feature_ids?: number[]
  start_time?: number
  end_time?: number
  mobile?: string
  conditions?: { key: string; operator: string; value: string }[]
  page?: number
  page_size?: number
  realtime?: boolean
}

export interface SyncParams {
  sync_all: boolean
  feature_ids?: number[]
  start_time?: number
  end_time?: number
}

export interface PaginatedParams {
  page?: number
  page_size?: number
}

export interface ContactListParams extends PaginatedParams {
  department_id?: number
  keyword?: string
  name?: string
  mobile?: string
}

export interface ContactMemberParams extends PaginatedParams {
  keyword?: string
}

export interface SyncHistoryParams extends PaginatedParams {
  sync_type?: string
}

export interface SyncFeatureItem {
  feature_id: number
  enabled: boolean
}

export interface AdminOperLogParams extends PaginatedParams {
  start_time?: number
  end_time?: number
  oper_type?: string
  oper_userid?: string
}

export interface AdminOperLogSync {
  start_time?: number
  end_time?: number
}

export const authAPI = {
  login: (data: { username: string; password: string }) => api.post('/auth/login', data),
}

export const healthAPI = {
  check: () => api.get('/health'),
}

export const logAPI = {
  query: (data: LogQueryParams) => api.post('/logs/query', data),
  getFeatures: () => api.get('/logs/features'),
  getTimeRange: () => api.get('/logs/time-range'),
  getFieldPaths: () => api.get('/logs/field-paths'),
}

export const syncAPI = {
  sync: (data: SyncParams) => api.post('/logs/sync', data),
  cancel: () => api.post('/logs/sync/cancel'),
  status: () => api.get('/logs/sync/status'),
}

export const keyAPI = {
  list: () => api.get('/keys'),
  add: (data: { version: string; pem_content?: string }) => api.post('/keys', data),
  activate: (data: { version: string }) => api.put('/keys/activate', data),
  test: (version: string) => api.get('/keys/test', { params: { version } }),
}

export const schedulerAPI = {
  start: (data?: { start_delay?: string }) => api.post('/scheduler/start', data),
  stop: () => api.post('/scheduler/stop'),
  status: () => api.get('/scheduler/status'),
  incrementalSync: (data: SyncParams) => api.post('/scheduler/sync', data),
  setInterval: (data: { interval: string }) => api.put('/scheduler/interval', data),
}

export const contactAPI = {
  list: (params: ContactListParams) => api.get('/contacts', { params }),
  getDepartments: () => api.get('/contacts/departments'),
  getDeptTree: () => api.get('/contacts/tree'),
  getDeptMembers: (deptId: number, params: ContactMemberParams) => api.get(`/contacts/departments/${deptId}/members`, { params }),
  getContact: (userId: string) => api.get(`/contacts/${userId}`),
  getNames: (user_ids: string[]) => api.post('/contacts/names', { user_ids }),
  sync: () => api.post('/contacts/sync'),
  syncIncremental: () => api.post('/contacts/sync/incremental'),
  cancel: () => api.post('/contacts/sync/cancel'),
  status: () => api.get('/contacts/sync/status'),
  syncAsyncExport: (data?: { department_id?: number; fetch_child?: number }) => api.post('/contacts/sync/async-export', data),
  syncIncrementalAsync: () => api.post('/contacts/sync/incremental-async'),
  asyncSyncStatus: () => api.get('/contacts/sync/async-status'),
}

export const dashboardAPI = {
  getOverview: () => api.get('/dashboard/overview'),
  getInactiveUsers: (params?: { range?: string; dept_id?: number; min_inactive_days?: number }) => api.get('/dashboard/inactive-users', { params }),
}

export const systemAPI = {
  getStatus: () => api.get('/system/status'),
}

export const syncHistoryAPI = {
  list: (params: SyncHistoryParams) => api.get('/sync-history', { params }),
}

export const syncFeatureAPI = {
  list: () => api.get('/sync-features'),
  update: (data: { features: SyncFeatureItem[] }) => api.put('/sync-features', data),
}

export const adminOperLogAPI = {
  query: (params: AdminOperLogParams) => api.get('/admin-oper-logs', { params }),
  sync: (data: AdminOperLogSync) => api.post('/admin-oper-logs/sync', data),
  syncStatus: () => api.get('/admin-oper-logs/sync/status'),
  getTypes: () => api.get('/admin-oper-logs/types'),
  getUsers: () => api.get('/admin-oper-logs/users'),
}

export default api
