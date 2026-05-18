export interface ApiResponse<T = unknown> {
  code: number
  msg?: string
  data?: T
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  refresh_token: string
  username: string
}

export interface HealthCheckResponse {
  status: string
  checks: Record<string, string>
}

export interface LogQueryParams {
  feature_ids: number[]
  start_time: number
  end_time: number
  mobile?: string
  conditions?: QueryCondition[]
  page?: number
  page_size?: number
  realtime?: boolean
}

export interface QueryCondition {
  key: string
  operator: string
  value: string
}

export interface LogQueryResponse {
  data: LogEntry[]
  total: number
  cursor?: string
}

export interface LogEntry {
  id: number
  feature_id: number
  log_time: number
  openid: string
  parsed_json?: Record<string, unknown>
  raw_data?: string
  sender?: {
    openid?: string
    name?: string
  }
  content?: string
}

export interface SyncParams {
  sync_all: boolean
  feature_ids?: number[]
  start_time?: number
  end_time?: number
}

export interface SyncStatus {
  running: boolean
  progress: number
  total: number
  results: Record<string, SyncFeatureResult>
}

export interface SyncFeatureResult {
  synced: number
  failed: number
  duration: number
}

export interface PaginatedParams {
  page?: number
  page_size?: number
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
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

export interface Contact {
  user_id: string
  name: string
  mobile: string
  department: number | number[]
  position?: string
  avatar?: string
  email?: string
  is_active: number
  created_at: string
  updated_at: string
}

export interface Department {
  id: number
  name: string
  parent_id: number
  order: number
  childrens?: Department[]
}

export interface DeptMember {
  userid: string
  name: string
  department: number[]
  position?: string
  mobile?: string
  email?: string
  avatar?: string
  status: number
}

export interface ContactSyncStatus {
  running: boolean
  progress: number
  total: number
  synced: number
  failed: number
  started_at?: string
}

export interface KeyVersion {
  version: string
  is_active: boolean
  created_at: string
  expires_at?: string
  test_status?: 'pending' | 'success' | 'failure'
}

export interface SchedulerStatus {
  running: boolean
  interval: string
  last_run?: string
  next_run?: string
}

export interface SyncHistoryParams extends PaginatedParams {
  sync_type?: string
}

export interface SyncHistory {
  id: number
  feature_id: number
  feature_name: string
  sync_type: string
  start_time: string
  end_time: string
  status: string
  total: number
  synced: number
  failed: number
  duration: number
  created_at: string
}

export interface SyncFeature {
  feature_id: number
  name: string
  description?: string
  enabled: boolean
  last_sync_time?: string
  total_records: number
  time_range?: {
    start?: string
    end?: string
  }
}

export interface AdminOperLogParams extends PaginatedParams {
  start_time?: number
  end_time?: number
  oper_type?: string
  oper_userid?: string
}

export interface AdminOperLog {
  id: number
  time: number
  oper_type_id: number
  oper_type: string
  oper_userid: string
  oper_name: string
  oper_data: string
  oper_desc?: string
  app_id?: string
  created_at: string
}

export interface AdminOperLogSync {
  start_time?: number
  end_time?: number
}

export interface AdminOperLogStats {
  running: boolean
  total: number
  last_time?: string
  by_type?: Record<string, number>
  by_user?: Record<string, number>
  daily?: Array<{ date: string; count: number }>
}

export interface DashboardOverview {
  kpi: {
    latest_sync_time?: string
    recent_sync_count: number
    failed_types_count: number
    active_keys_count: number
    contacts_count: number
    inactive_percentage: number
  }
  recent_syncs: Array<{
    feature_id: number
    feature_name: string
    synced: number
    failed: number
    duration: number
    created_at: string
  }>
  alerts: Array<{
    type: string
    level: 'critical' | 'warning' | 'info'
    message: string
  }>
}

export interface InactiveUser {
  name: string
  mobile: string
  position?: string
  department?: string
  user_id: string
  inactive_days: number
}

export interface SystemStatus {
  db_connected: boolean
  uptime: string
  key_status: {
    active_version?: string
    total_versions: number
  }
  contact_status: {
    last_sync_time?: string
    members_count: number
  }
  sync_coverage: Array<{
    feature_id: number
    feature_name: string
    last_sync_time?: string
    total_records: number
    time_range?: {
      start?: string
      end?: string
    }
  }>
  storage: Array<{
    table_name: string
    rows: number
    data_size: string
    index_size: string
    total_size: string
  }>
}

export interface TaskInfo {
  id: string
  task_type: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  progress?: number
  total?: number
  result?: Record<string, unknown>
  error?: string
  created_at: string
  started_at?: string
  completed_at?: string
}

export interface TaskSubmitParams {
  task_type: string
  params?: Record<string, unknown>
}

export interface OperationLog {
  id: number
  username: string
  action: string
  method: string
  path: string
  status_code: number
  duration: number
  ip: string
  created_at: string
}

export interface FieldPath {
  path: string
  type: string
  sample?: string
}

export interface TimeRange {
  earliest?: number
  latest?: number
}
