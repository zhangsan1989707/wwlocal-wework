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
  conditions?: Record<string, { value: string; operator: string }> | null
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
  gender?: number
  status?: number
  created_at: string
  updated_at: string
}

export interface Department {
  id: number
  name: string
  parent_id: number
  order: number
  member_count?: number
  children?: Department[]
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
  phase?: string
  progress: number
  total: number
  synced: number
  failed: number
  started_at?: string
  last_sync?: string
  error_msg?: string
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
  trigger: string
  start_time: string
  end_time: string
  status: string
  total: number
  succeeded: number
  failed: number
  duration_ms: number
  error_msg?: string
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
  kpis: {
    latest_sync_time?: string
    recent_sync_count: number
    synced_7d_count: number
    failed_feature_count: number
    active_key_version?: string
    active_key_days: number
    key_count: number
    contact_count: number
    contact_last_sync?: string
    inactive_rate: number
    inactive_count: number
  }
  recent_syncs: Array<{
    feature_id: number
    feature_name: string
    succeeded: number
    failed: number
    duration_ms: number
    start_time: string
    sync_type: string
    trigger: string
    error?: string
  }>
  problems: Array<{
    type: string
    level: 'critical' | 'warning' | 'info'
    message: string
    action?: string
  }>
}

export interface InactiveUser {
  name: string
  mobile: string
  position?: string
  department?: string
  user_id: string
  active_days: number
  inactive_days: number
}

export interface InactiveUsersResponse {
  total_contacts: number
  inactive_count: number
  inactive_users: InactiveUser[]
  feature_names: Record<number, string>
  dept_stats: Array<{ id: number; name: string; total: number; active: number; inactive: number }>
  range: string
  total_days: number
  min_inactive_days: number
  page: number
  page_size: number
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

export interface TrendCoverage {
  expected_days: number
  covered_days: number
  rate: number
  by_feature: Record<number, number>
}

export interface TrendFeature {
  id: number
  name: string
  counts: number[]
}

export interface TrendResponse {
  granularity: string
  range: string
  total_days: number
  coverage: TrendCoverage
  dates: string[]
  series: {
    active_users: number[]
    total_contacts: number
  }
  features: TrendFeature[]
}

export interface TrendDeptStat {
  id: number
  name: string
  total: number
  active: number
  inactive: number
  active_rate: number
  avg_active_days: number
}

export interface TrendDeptResponse {
  departments: TrendDeptStat[]
}
