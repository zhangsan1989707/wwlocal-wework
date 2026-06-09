import axios from 'axios'
import type {
  ApiResponse,
  LoginRequest,
  LoginResponse,
  HealthCheckResponse,
  LogQueryParams,
  LogQueryResponse,
  BehaviorQueryParams,
  BehaviorQueryResponse,
  SyncParams,
  SyncStatus,
  PaginatedResponse,
  ContactListParams,
  ContactMemberParams,
  Contact,
  Department,
  DeptMember,
  ContactSyncStatus,
  KeyVersion,
  SchedulerStatus,
  NightlyJobStatus,
  SyncHistoryParams,
  SyncHistory,
  SyncFeature,
  AdminOperLogParams,
  AdminOperLog,
  AdminOperLogSync,
  AdminOperLogStats,
  DashboardOverview,
  InactiveUser,
  InactiveUsersResponse,
  SystemStatus,
  TaskInfo,
  TaskSubmitParams,
  OperationLog,
  FieldPath,
  TimeRange,
  TrendResponse,
  TrendDeptResponse,
  DashboardV2Overview,
  DashboardV2Trend,
  DashboardV2DeptStat,
  DashboardV2UserItem,
  AdminUser,
} from '../types/api'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

const TOKEN_KEY = 'auth_token'
const REFRESH_TOKEN_KEY = 'auth_refresh_token'
const USERNAME_KEY = 'auth_username'
const ROLE_KEY = 'auth_role'

function getStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

function setStoredToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token)
  } catch {
    console.error('Failed to store token')
  }
}

function getStoredRefreshToken(): string | null {
  try {
    return localStorage.getItem(REFRESH_TOKEN_KEY)
  } catch {
    return null
  }
}

function setStoredRefreshToken(token: string): void {
  try {
    localStorage.setItem(REFRESH_TOKEN_KEY, token)
  } catch {
    console.error('Failed to store refresh token')
  }
}

function removeStoredToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    localStorage.removeItem(USERNAME_KEY)
    localStorage.removeItem(ROLE_KEY)
  } catch {
    console.error('Failed to remove stored token')
  }
}

function getStoredUsername(): string | null {
  try {
    return localStorage.getItem(USERNAME_KEY)
  } catch {
    return null
  }
}

function setStoredUsername(username: string): void {
  try {
    localStorage.setItem(USERNAME_KEY, username)
  } catch {
    console.error('Failed to store username')
  }
}

function getStoredRole(): string | null {
  try {
    return localStorage.getItem(ROLE_KEY)
  } catch {
    return null
  }
}

function setStoredRole(role: string): void {
  try {
    localStorage.setItem(ROLE_KEY, role)
  } catch {
    console.error('Failed to store role')
  }
}

// 拦截器已将 AxiosResponse 解包为 ApiResponse<T>，此类型标注实际返回值
type ApiResult<T> = Promise<ApiResponse<T>>
type BlobResult = Promise<Blob>

let reloading = false
let refreshPromise: Promise<string | null> | null = null

api.interceptors.request.use((config) => {
  const token = getStoredToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

async function tryRefreshToken(): Promise<string | null> {
  const refreshToken = getStoredRefreshToken()
  if (!refreshToken) return null

  try {
    const res = await axios.post('/api/v1/auth/refresh', {
      refresh_token: refreshToken,
    })
    const data = res.data as ApiResponse<{ token: string }>
    if (data.code === 0 && data.data?.token) {
      setStoredToken(data.data.token)
      return data.data.token
    }
  } catch {
    // refresh failed
  }
  return null
}

api.interceptors.response.use(
  (response) => response.data,
  async (error) => {
    const status = error.response?.status
    const data = error.response?.data as ApiResponse<unknown> | undefined
    const originalRequest = error.config

    if (status === 401 && !originalRequest._retry && !reloading) {
      originalRequest._retry = true
      // 避免并发请求重复 refresh
      if (!refreshPromise) {
        refreshPromise = tryRefreshToken().finally(() => {
          refreshPromise = null
        })
      }
      const newToken = await refreshPromise
      if (newToken) {
        originalRequest.headers.Authorization = `Bearer ${newToken}`
        return api(originalRequest)
      }
      // refresh 失败，清除 token 并刷新页面显示登录界面
      reloading = true
      removeStoredToken()
      window.location.reload()
      return Promise.reject(new Error('登录已过期，请重新登录'))
    }

    if (status === 403) {
      return Promise.reject(new Error('没有权限执行此操作'))
    }

    if (status === 429) {
      return Promise.reject(new Error('请求过于频繁，请稍后再试'))
    }

    if (status && status >= 500) {
      return Promise.reject(new Error(data?.msg || '服务器错误，请稍后再试'))
    }

    const message = data?.msg || error.message || '请求失败'
    return Promise.reject(new Error(message))
  }
)

export const authAPI = {
  login: (data: LoginRequest): ApiResult<LoginResponse> =>
    (api.post<ApiResponse<LoginResponse>>('/auth/login', data) as unknown as ApiResult<LoginResponse>).then((res) => {
      if (res.code === 0 && res.data) {
        setStoredToken(res.data.token)
        if (res.data.refresh_token) {
          setStoredRefreshToken(res.data.refresh_token)
        }
        setStoredUsername(res.data.username)
        setStoredRole(res.data.role)
      }
      return res
    }),
  changePassword: (data: { old_password: string; new_password: string }) =>
    api.put<ApiResponse<{ message: string }>>('/auth/password', data) as unknown as ApiResult<{ message: string }>,
  getToken: () => getStoredToken(),
  getUsername: () => getStoredUsername(),
  getRole: () => getStoredRole(),
  logout: () => removeStoredToken(),
  isAuthenticated: () => !!getStoredToken(),
}

export const userAPI = {
  list: () =>
    api.get<ApiResponse<AdminUser[]>>('/users') as unknown as ApiResult<AdminUser[]>,
  create: (data: { username: string; password: string; role: string; enabled?: boolean; dept_ids: number[] }) =>
    api.post<ApiResponse<AdminUser>>('/users', data) as unknown as ApiResult<AdminUser>,
  update: (id: number, data: { role: string; enabled: boolean; dept_ids: number[] }) =>
    api.put<ApiResponse<AdminUser>>(`/users/${id}`, data) as unknown as ApiResult<AdminUser>,
  resetPassword: (id: number, password: string) =>
    api.put<ApiResponse<{ message: string }>>(`/users/${id}/password`, { password }) as unknown as ApiResult<{ message: string }>,
}

export const healthAPI = {
  check: () =>
    axios.get<ApiResponse<HealthCheckResponse>>('/health').then((res) => res.data),
}

export const logAPI = {
  query: (data: LogQueryParams) =>
    api.post<ApiResponse<LogQueryResponse>>('/logs/query', data) as unknown as ApiResult<LogQueryResponse>,
  behaviorQuery: (data: BehaviorQueryParams) =>
    api.post<ApiResponse<BehaviorQueryResponse>>('/logs/behavior-query', data) as unknown as ApiResult<BehaviorQueryResponse>,
  queryByCursor: (data: LogQueryParams & { cursor?: string }) =>
    api.post<ApiResponse<LogQueryResponse>>('/logs/query/cursor', data) as unknown as ApiResult<LogQueryResponse>,
  getFeatures: () =>
    api.get<ApiResponse<SyncFeature[]>>('/logs/features') as unknown as ApiResult<SyncFeature[]>,
  getTimeRange: () =>
    api.get<ApiResponse<TimeRange>>('/logs/time-range') as unknown as ApiResult<TimeRange>,
  getFieldPaths: () =>
    api.get<ApiResponse<FieldPath[]>>('/logs/field-paths') as unknown as ApiResult<FieldPath[]>,
  exportCSVURL: () => {
    return '/api/v1/logs/export'
  },
  behaviorExportCSVURL: () => {
    return '/api/v1/logs/behavior-export'
  },
}

export const syncAPI = {
  sync: (data: SyncParams) =>
    api.post<ApiResponse<{ message: string }>>('/logs/sync', data) as unknown as ApiResult<{ message: string }>,
  cancel: () =>
    api.post<ApiResponse<{ message: string }>>('/logs/sync/cancel') as unknown as ApiResult<{ message: string }>,
  status: () =>
    api.get<ApiResponse<SyncStatus>>('/logs/sync/status') as unknown as ApiResult<SyncStatus>,
}

export const keyAPI = {
  list: () =>
    api.get<ApiResponse<KeyVersion[]>>('/keys') as unknown as ApiResult<KeyVersion[]>,
  add: (data: { version: string; private_key_pem: string }) =>
    api.post<ApiResponse<KeyVersion>>('/keys', data) as unknown as ApiResult<KeyVersion>,
  activate: (data: { version: string }) =>
    api.put<ApiResponse<KeyVersion>>('/keys/activate', data) as unknown as ApiResult<KeyVersion>,
  test: (version: string) =>
    api.get<ApiResponse<{ success: boolean; message: string }>>('/keys/test', {
      params: { version },
    }) as unknown as ApiResult<{ success: boolean; message: string }>,
}

export const schedulerAPI = {
  start: (data?: { start_delay?: string }) =>
    api.post<ApiResponse<{ message: string }>>('/scheduler/start', data) as unknown as ApiResult<{ message: string }>,
  stop: () =>
    api.post<ApiResponse<{ message: string }>>('/scheduler/stop') as unknown as ApiResult<{ message: string }>,
  status: () =>
    api.get<ApiResponse<SchedulerStatus>>('/scheduler/status') as unknown as ApiResult<SchedulerStatus>,
  incrementalSync: (data: SyncParams) =>
    api.post<ApiResponse<{ message: string }>>('/scheduler/sync', data) as unknown as ApiResult<{ message: string }>,
  setInterval: (data: { interval: string }) =>
    api.put<ApiResponse<{ message: string }>>('/scheduler/interval', data) as unknown as ApiResult<{ message: string }>,
}

export const nightlyAPI = {
  status: () =>
    api.get<ApiResponse<NightlyJobStatus>>('/nightly/status') as unknown as ApiResult<NightlyJobStatus>,
  run: (date?: string) =>
    api.post<ApiResponse<{ message: string; date: string }>>('/nightly/run', null, {
      params: date ? { date } : undefined,
    }) as unknown as ApiResult<{ message: string; date: string }>,
}

export const contactAPI = {
  list: (params: ContactListParams) =>
    api.get<ApiResponse<PaginatedResponse<Contact>>>('/contacts', { params }) as unknown as ApiResult<PaginatedResponse<Contact>>,
  getDepartments: () =>
    api.get<ApiResponse<Department[]>>('/contacts/departments') as unknown as ApiResult<Department[]>,
  getDeptTree: () =>
    api.get<ApiResponse<{ tree: Department[]; total: number }>>('/contacts/tree') as unknown as ApiResult<{ tree: Department[]; total: number }>,
  getDeptMembers: (deptId: number, params: ContactMemberParams) =>
    api.get<ApiResponse<PaginatedResponse<DeptMember>>>(
      `/contacts/departments/${deptId}/members`,
      { params }
    ) as unknown as ApiResult<PaginatedResponse<DeptMember>>,
  getContact: (userId: string) =>
    api.get<ApiResponse<Contact>>(`/contacts/${userId}`) as unknown as ApiResult<Contact>,
  getNames: (user_ids: string[]) =>
    api.post<ApiResponse<Record<string, string>>>('/contacts/names', {
      user_ids,
    }) as unknown as ApiResult<Record<string, string>>,
  sync: () =>
    api.post<ApiResponse<{ message: string }>>('/contacts/sync') as unknown as ApiResult<{ message: string }>,
  syncIncremental: () =>
    api.post<ApiResponse<{ message: string }>>('/contacts/sync/incremental') as unknown as ApiResult<{ message: string }>,
  cancel: () =>
    api.post<ApiResponse<{ message: string }>>('/contacts/sync/cancel') as unknown as ApiResult<{ message: string }>,
  status: () =>
    api.get<ApiResponse<ContactSyncStatus>>('/contacts/sync/status') as unknown as ApiResult<ContactSyncStatus>,
}

// Dashboard V2 API
export const dashboardV2Api = {
  getOverview: (date?: string) =>
    api.get<ApiResponse<DashboardV2Overview>>('/dashboard/v2/overview', { params: { date } }) as unknown as ApiResult<DashboardV2Overview>,
  getTrend: (params: { metric_type: string; start_date?: string; end_date?: string; granularity?: string; dimension_key?: string }) =>
    api.get<ApiResponse<DashboardV2Trend>>('/dashboard/v2/trend', { params }) as unknown as ApiResult<DashboardV2Trend>,
  getMultiTrend: (params: { metric_types: string; start_date?: string; end_date?: string; granularity?: string }) =>
    api.get<ApiResponse<DashboardV2Trend>>('/dashboard/v2/multi-trend', { params }) as unknown as ApiResult<DashboardV2Trend>,
  getDepartments: (date?: string) =>
    api.get<ApiResponse<DashboardV2DeptStat[]>>('/dashboard/v2/departments', { params: { date } }) as unknown as ApiResult<DashboardV2DeptStat[]>,
  getDevices: (date?: string) =>
    api.get<ApiResponse<DashboardV2Overview['devices']>>('/dashboard/v2/devices', { params: { date } }) as unknown as ApiResult<DashboardV2Overview['devices']>,
  getUsers: (params: { date?: string; list_type?: string; page?: number; page_size?: number }) =>
    api.get<ApiResponse<{ total: number; users: DashboardV2UserItem[] }>>('/dashboard/v2/users', { params }) as unknown as ApiResult<{ total: number; users: DashboardV2UserItem[] }>,
  exportOverview: (date?: string) =>
    api.get('/dashboard/v2/export/overview', { params: { date }, responseType: 'blob' }) as unknown as BlobResult,
  exportTrend: (params: { metric_types?: string; metric_type?: string; start_date?: string; end_date?: string; granularity?: string }) =>
    api.get('/dashboard/v2/export/trend', { params, responseType: 'blob' }) as unknown as BlobResult,
  exportDepartments: (date?: string) =>
    api.get('/dashboard/v2/export/departments', { params: { date }, responseType: 'blob' }) as unknown as BlobResult,
  exportDevices: (date?: string) =>
    api.get('/dashboard/v2/export/devices', { params: { date }, responseType: 'blob' }) as unknown as BlobResult,
  exportUsers: (params: { date?: string; list_type?: string }) =>
    api.get('/dashboard/v2/export/users', { params, responseType: 'blob' }) as unknown as BlobResult,
}

export const dashboardAPI = {
  getOverview: () =>
    api.get<ApiResponse<DashboardOverview>>('/dashboard/overview') as unknown as ApiResult<DashboardOverview>,
  getInactiveUsers: (params?: {
    range?: string
    dept_id?: number
    min_inactive_days?: number
    page?: number
    page_size?: number
  }) =>
    api.get<ApiResponse<InactiveUsersResponse>>(
      '/dashboard/inactive-users',
      { params }
    ) as unknown as ApiResult<InactiveUsersResponse>,
  exportInactiveUsersURL: (params: { range?: string; dept_id?: number; min_inactive_days?: number }) => {
    const qs = new URLSearchParams()
    if (params.range) qs.set('range', params.range)
    if (params.dept_id) qs.set('dept_id', String(params.dept_id))
    if (params.min_inactive_days) qs.set('min_inactive_days', String(params.min_inactive_days))
    return `/api/v1/dashboard/inactive-users/export?${qs.toString()}`
  },
  getTrend: (params: { granularity?: string; range?: string; dept_id?: number; feature_ids?: string }) =>
    api.get<ApiResponse<TrendResponse>>('/dashboard/trend', { params }) as unknown as ApiResult<TrendResponse>,
  getTrendByDept: (params: { range?: string; feature_id?: number }) =>
    api.get<ApiResponse<TrendDeptResponse>>('/dashboard/trend/dept', { params }) as unknown as ApiResult<TrendDeptResponse>,
  exportTrendURL: (params: { granularity?: string; range?: string; dept_id?: number; feature_ids?: string }) => {
    const qs = new URLSearchParams()
    if (params.granularity) qs.set('granularity', params.granularity)
    if (params.range) qs.set('range', params.range)
    if (params.dept_id) qs.set('dept_id', String(params.dept_id))
    if (params.feature_ids) qs.set('feature_ids', params.feature_ids)
    return `/api/v1/dashboard/trend/export?${qs.toString()}`
  },
}

export const systemAPI = {
  getStatus: () =>
    api.get<ApiResponse<SystemStatus>>('/system/status') as unknown as ApiResult<SystemStatus>,
}

export const syncHistoryAPI = {
  list: (params: SyncHistoryParams) =>
    api.get<ApiResponse<PaginatedResponse<SyncHistory>>>('/sync-history', {
      params,
    }) as unknown as ApiResult<PaginatedResponse<SyncHistory>>,
}

export const syncFeatureAPI = {
  list: () =>
    api.get<ApiResponse<SyncFeature[]>>('/sync-features') as unknown as ApiResult<SyncFeature[]>,
  update: (data: { features: Array<{ feature_id: number; enabled: boolean }> }) =>
    api.put<ApiResponse<{ message: string }>>('/sync-features', data) as unknown as ApiResult<{ message: string }>,
}

export const adminOperLogAPI = {
  query: (params: AdminOperLogParams) =>
    api.get<ApiResponse<PagedResponse<AdminOperLog>>>(
      '/admin-oper-logs',
      { params }
    ) as unknown as ApiResult<PagedResponse<AdminOperLog>>,
  sync: (data: AdminOperLogSync) =>
    api.post<ApiResponse<{ running: boolean; message: string }>>(
      '/admin-oper-logs/sync',
      data
    ) as unknown as ApiResult<{ running: boolean; message: string }>,
  syncStatus: () =>
    api.get<ApiResponse<AdminOperLogStats>>(
      '/admin-oper-logs/sync/status'
    ) as unknown as ApiResult<AdminOperLogStats>,
  getStats: (params?: { start_time?: number; end_time?: number }) =>
    api.get<ApiResponse<AdminOperLogStats>>('/admin-oper-logs/stats', {
      params,
    }) as unknown as ApiResult<AdminOperLogStats>,
  getTypes: () =>
    api.get<ApiResponse<string[]>>('/admin-oper-logs/types') as unknown as ApiResult<string[]>,
  getUsers: () =>
    api.get<ApiResponse<string[]>>('/admin-oper-logs/users') as unknown as ApiResult<string[]>,
}

export const operationLogAPI = {
  list: (params?: { page?: number; page_size?: number; action?: string; status_code?: number }) =>
    api.get<ApiResponse<PaginatedResponse<OperationLog>>>('/operation-logs', { params }) as unknown as ApiResult<PaginatedResponse<OperationLog>>,
  getActions: () =>
    api.get<ApiResponse<string[]>>('/operation-logs/actions') as unknown as ApiResult<string[]>,
}

export const taskAPI = {
  submit: (data: TaskSubmitParams) =>
    api.post<ApiResponse<{ task_id: string }>>('/tasks', data) as unknown as ApiResult<{ task_id: string }>,
  list: () =>
    api.get<ApiResponse<TaskInfo[]>>('/tasks') as unknown as ApiResult<TaskInfo[]>,
  get: (id: string) =>
    api.get<ApiResponse<TaskInfo>>(`/tasks/${id}`) as unknown as ApiResult<TaskInfo>,
  cancel: (id: string) =>
    api.post<ApiResponse<{ message: string }>>(`/tasks/${id}/cancel`) as unknown as ApiResult<{ message: string }>,
  retry: (id: string) =>
    api.post<ApiResponse<{ message: string }>>(`/tasks/${id}/retry`) as unknown as ApiResult<{ message: string }>,
}

type PagedResponse<T> = {
  data: T[]
  total: number
  page: number
  page_size: number
}

export type {
  ApiResponse,
  LoginRequest,
  LoginResponse,
  HealthCheckResponse,
  LogQueryParams,
  LogQueryResponse,
  SyncParams,
  SyncStatus,
  PaginatedResponse,
  ContactListParams,
  Contact,
  Department,
  DeptMember,
  ContactSyncStatus,
  KeyVersion,
  SchedulerStatus,
  SyncHistoryParams,
  SyncHistory,
  SyncFeature,
  AdminOperLogParams,
  AdminOperLog,
  AdminOperLogSync,
  AdminOperLogStats,
  DashboardOverview,
  InactiveUser,
  InactiveUsersResponse,
  SystemStatus,
  TaskInfo,
  TaskSubmitParams,
  OperationLog,
  FieldPath,
  TimeRange,
  TrendResponse,
  TrendDeptResponse,
  DashboardV2Overview,
  DashboardV2Trend,
  DashboardV2DeptStat,
  DashboardV2UserItem,
}

export default api
