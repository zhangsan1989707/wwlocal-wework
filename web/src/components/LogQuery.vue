<template>
  <div class="log-query">
    <el-card class="query-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">查询条件</span>
          <div class="header-actions">
            <el-button type="primary" @click="handleQuery" :loading="loading">
              <el-icon><Search /></el-icon>
              查询
            </el-button>
            <el-button @click="handleReset">
              <el-icon><Refresh /></el-icon>
              重置
            </el-button>
          </div>
        </div>
      </template>

      <el-form :model="form" label-position="top">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="日志类型">
              <el-select
                v-model="form.feature_ids"
                multiple
                placeholder="请选择日志类型（可多选）"
                style="width: 100%"
                collapse-tags
                collapse-tags-tooltip
              >
                <template #prefix>
                  <el-icon><DataAnalysis /></el-icon>
                </template>
                <el-option
                  v-for="item in features"
                  :key="item.feature_id"
                  :label="item.name"
                  :value="item.feature_id"
                >
                  <span style="float: left">{{ item.name }}</span>
                  <span style="float: right; color: #8492a6; font-size: 12px">{{ item.feature_id }}</span>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="时间范围">
              <div class="time-range-container">
                <el-button-group class="time-shortcuts">
                  <el-button
                    v-for="shortcut in timeShortcuts"
                    :key="shortcut.label"
                    :type="activeShortcut === shortcut.label ? 'primary' : 'default'"
                    size="small"
                    @click="applyTimeShortcut(shortcut)"
                  >
                    {{ shortcut.label }}
                  </el-button>
                </el-button-group>
                <el-date-picker
                  v-model="dateRange"
                  type="datetimerange"
                  format="YYYY-MM-DD HH:mm:ss"
                  value-format="YYYY-MM-DD HH:mm:ss"
                  range-separator="至"
                  start-placeholder="开始时间"
                  end-placeholder="结束时间"
                  style="width: 100%; margin-top: 8px"
                  @change="handleDateChange"
                />
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="手机号匹配">
              <el-input
                v-model="form.mobile"
                placeholder="输入手机号，匹配日志中的 openid"
                clearable
              />
            </el-form-item>
          </el-col>
          <el-col :span="16">
            <el-form-item label="&nbsp;">
              <el-checkbox v-model="form.realtime">
                <el-tooltip content="数据库无结果时自动从政务微信实时拉取" placement="top">
                  <span>实时查询 <el-icon><QuestionFilled /></el-icon></span>
                </el-tooltip>
              </el-checkbox>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="筛选条件">
              <div class="conditions-container">
                <div class="conditions-header">
                  <el-button type="primary" link @click="addCondition">
                    <el-icon><Plus /></el-icon>
                    添加字段条件
                  </el-button>
                  <el-button v-if="conditions.length > 0" type="danger" link @click="clearConditions">
                    <el-icon><Delete /></el-icon>
                    清空
                  </el-button>
                </div>
                <div v-if="conditions.length === 0" class="empty-conditions">
                  可按 openid、sender.openid、msg_type 等字段筛选
                </div>
                <div v-else class="conditions-list">
                  <div v-for="(condition, index) in conditions" :key="index" class="condition-item">
                    <el-autocomplete
                      v-model="condition.key"
                      :fetch-suggestions="queryFieldPaths"
                      placeholder="字段名，如 sender.openid"
                      style="width: 200px"
                      clearable
                    />
                    <el-select v-model="condition.operator" style="width: 100px">
                      <el-option label="等于" value="=" />
                      <el-option label="包含" value="like" />
                    </el-select>
                    <el-input
                      v-model="condition.value"
                      placeholder="值"
                      style="flex: 1"
                    />
                    <el-button type="danger" link @click="removeCondition(index)">
                      <el-icon><Close /></el-icon>
                    </el-button>
                  </div>
                </div>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-card>

    <el-card class="result-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">查询结果</span>
          <div class="header-actions">
            <span v-if="total > 0" class="result-count">共 {{ total }} 条记录</span>
            <el-button
              v-if="tableData.length > 0"
              type="success"
              size="small"
              @click="handleExport"
            >
              <el-icon><Download /></el-icon>
              导出 CSV
            </el-button>
          </div>
        </div>
      </template>

      <el-table
        :data="tableData"
        border
        style="width: 100%"
        v-loading="loading"
        max-height="500"
        stripe
        highlight-current-row
      >
        <el-table-column prop="feature_id" label="日志类型编号" width="100" align="center" />
        <el-table-column prop="log_date" label="时间" width="180" align="center" />
        <el-table-column label="数据内容" min-width="400">
          <template #default="{ row, $index }">
            <div class="data-cell">
              <pre v-if="isRowExpanded($index)" class="data-content">{{ formatData(row) }}</pre>
              <span v-else class="data-preview" @click="toggleRow($index)">{{ formatPreview(row) }}</span>
              <el-button
                v-if="isRowExpanded($index)"
                type="primary"
                link
                size="small"
                @click="toggleRow($index)"
              >收起</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-if="total > 0"
        class="pagination"
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 50, 100, 500]"
        layout="total, sizes, prev, pager, next"
        background
        @current-change="handleQuery"
        @size-change="handleQuery"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { logAPI, syncFeatureAPI, contactAPI } from '../api'
import type { LogQueryParams, SyncFeature } from '../types/api'
import { ElMessage } from 'element-plus'
import { Search, Refresh, DataAnalysis, Plus, Delete, Close, QuestionFilled, Download } from '@element-plus/icons-vue'

interface Condition {
  key: string
  operator: string
  value: string
}

interface FeatureItem {
  feature_id: number
  name: string
  enabled: boolean
}

interface LogRow {
  feature_id: number
  log_date: string
  log_time: number
  openid?: string
  sender?: { openid?: string; name?: string }
  msg_type?: string
  chat_type?: string
  _decrypt_failed?: boolean
  [key: string]: unknown
}

const form = reactive<LogQueryParams>({
  feature_ids: [],
  start_time: 0,
  end_time: 0,
  conditions: null,
  mobile: '',
  realtime: true,
  page: 1,
  page_size: 50,
})

const dateRange = ref<[Date, Date] | null>(null)
const activeShortcut = ref<string | null>(null)
const conditions = ref<Condition[]>([])
const loading = ref(false)
const rawTableData = ref<LogRow[]>([])
const tableData = ref<LogRow[]>([])
const rawTotal = ref(0)
const total = ref(0)
const pagination = reactive({
  page: 1,
  page_size: 50,
})
const features = ref<FeatureItem[]>([])
const fieldPaths = ref<string[]>([])
const expandedRows = ref<number[]>([])
const contactNames = ref<Record<string, string>>({})

const timeShortcuts = [
  { label: '最近1小时', hours: 1 },
  { label: '最近6小时', hours: 6 },
  { label: '最近1天', hours: 24 },
  { label: '最近7天', hours: 168 },
  { label: '最近30天', hours: 720 },
]

const route = useRoute()

onMounted(async () => {
  try {
    const res = await syncFeatureAPI.list()
    if (res.code === 0) {
      features.value = (res.data || []).map((f: SyncFeature) => ({ feature_id: f.feature_id, name: f.name, enabled: f.enabled }))
    }
  } catch {
    ElMessage.error('加载数据失败')
  }
  loadFieldPaths()

  const query = route.query
  if (query.mobile) {
    form.mobile = String(query.mobile)
  }
  if (query.feature_ids) {
    form.feature_ids = String(query.feature_ids).split(',').map(Number).filter(Boolean)
  }
  if (query.date_start && query.date_end) {
    dateRange.value = [
      new Date(Number(query.date_start) * 1000),
      new Date(Number(query.date_end) * 1000),
    ]
    activeShortcut.value = '最近7天'
  }
  if (query.mobile || form.feature_ids.length > 0) {
    if (!dateRange.value) {
      try {
        const timeRes = await logAPI.getTimeRange()
        if (timeRes.code === 0) {
          const t = timeRes.data as unknown as { start_time: number; end_time: number; now: number }
          dateRange.value = [
            new Date(t.start_time ? t.start_time * 1000 : (Date.now() - 7 * 24 * 3600 * 1000)),
            new Date(t.end_time ? t.end_time * 1000 : Date.now()),
          ]
        }
      } catch { /* ignore */ }
    }
    if (form.feature_ids.length > 0) {
      handleQuery()
    }
  } else {
    try {
      const timeRes = await logAPI.getTimeRange()
      if (timeRes.code === 0) {
        const t = timeRes.data as unknown as { start_time: number; end_time: number; now: number }
        dateRange.value = [
          new Date(t.start_time ? t.start_time * 1000 : (Date.now() - 7 * 24 * 3600 * 1000)),
          new Date(t.end_time ? t.end_time * 1000 : Date.now()),
        ]
      }
    } catch { /* ignore */ }
  }
})

const loadFieldPaths = async () => {
  try {
    const res = await logAPI.getFieldPaths()
    if (res.code === 0 && res.data) {
      fieldPaths.value = (res.data as unknown as Array<{ path: string }>).map((p) => p.path)
    }
  } catch {
    // ignore
  }
}

const queryFieldPaths = (query: string, cb: (results: { value: string }[]) => void) => {
  const q = query.toLowerCase()
  const results = fieldPaths.value
    .filter(p => !q || p.toLowerCase().includes(q))
    .slice(0, 20)
    .map(p => ({ value: p }))
  cb(results)
}

const applyTimeShortcut = (shortcut: { label: string; hours: number }) => {
  activeShortcut.value = shortcut.label
  const end = new Date()
  const start = new Date(end.getTime() - shortcut.hours * 60 * 60 * 1000)
  dateRange.value = [start, end]
}

const handleDateChange = () => {
  activeShortcut.value = null
}

const addCondition = () => {
  conditions.value.push({ key: '', operator: '=', value: '' })
}

const removeCondition = (index: number) => {
  conditions.value.splice(index, 1)
  applyClientFilter()
}

const clearConditions = () => {
  conditions.value = []
  applyClientFilter()
}

const resolveNestedValue = (data: Record<string, unknown>, path: string): unknown => {
  const parts = path.split('.')
  let current: unknown = data
  for (const part of parts) {
    if (current == null || typeof current !== 'object') return null
    current = (current as Record<string, unknown>)[part]
  }
  return current
}

const matchCondition = (row: LogRow, cond: Condition): boolean => {
  if (!cond.key || !cond.value) return true
  const value = resolveNestedValue(row, cond.key)
  if (value == null) return false
  const valStr = String(value).toLowerCase()
  const expStr = cond.value.toLowerCase()
  if (cond.operator === '=') {
    return valStr === expStr
  }
  return valStr.includes(expStr)
}

const applyClientFilter = () => {
  const validConds = conditions.value.filter(c => c.key && c.value)
  if (validConds.length === 0 || rawTableData.value.length === 0) {
    tableData.value = rawTableData.value
    total.value = rawTotal.value
    return
  }
  tableData.value = rawTableData.value.filter(row =>
    validConds.every(cond => matchCondition(row, cond))
  )
  total.value = tableData.value.length
}

watch(conditions, () => {
  applyClientFilter()
}, { deep: true })

const buildConditions = (): Record<string, { value: string; operator: string }> | null => {
  if (conditions.value.length === 0) return null

  const result: Record<string, { value: string; operator: string }> = {}
  for (const condition of conditions.value) {
    if (condition.key && condition.value) {
      result[condition.key] = {
        value: condition.value,
        operator: condition.operator,
      }
    }
  }
  return Object.keys(result).length > 0 ? result : null
}

const extractOpenids = (data: LogRow[]): string[] => {
  const ids = new Set<string>()
  for (const row of data) {
    if (row.openid) ids.add(row.openid)
    if (row.sender?.openid) ids.add(row.sender.openid)
    if (row.tolist && Array.isArray(row.tolist)) {
      for (const u of row.tolist as Array<{ userid?: string }>) {
        if (u?.userid) ids.add(u.userid)
      }
    }
  }
  return Array.from(ids)
}

const fetchContactNames = async (data: LogRow[]) => {
  const ids = extractOpenids(data)
  if (ids.length === 0) return
  try {
    const res = await contactAPI.getNames(ids)
    if (res.code === 0 && res.data) {
      contactNames.value = { ...contactNames.value, ...res.data }
    }
  } catch {
    // ignore
  }
}

const handleQuery = async () => {
  expandedRows.value = []
  if (form.feature_ids.length === 0) {
    ElMessage.warning('请选择至少一个日志类型')
    return
  }
  if (!dateRange.value) {
    ElMessage.warning('请选择时间范围')
    return
  }

  loading.value = true
  try {
    form.start_time = Math.floor(dateRange.value[0].getTime() / 1000)
    form.end_time = Math.floor(dateRange.value[1].getTime() / 1000)

    const res = await logAPI.query({
      feature_ids: form.feature_ids,
      start_time: form.start_time,
      end_time: form.end_time,
      conditions: buildConditions(),
      mobile: form.mobile,
      realtime: form.realtime,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    if (res.code === 0 && res.data) {
      rawTableData.value = res.data.data as unknown as LogRow[]
      rawTotal.value = res.data.total
      applyClientFilter()
      fetchContactNames(res.data.data as unknown as LogRow[])
    } else {
      ElMessage.error(res.msg || '查询失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '查询失败')
  } finally {
    loading.value = false
  }
}

const handleReset = () => {
  form.feature_ids = []
  form.mobile = ''
  conditions.value = []
  activeShortcut.value = null
  dateRange.value = null
  rawTableData.value = []
  tableData.value = []
  rawTotal.value = 0
  total.value = 0
  pagination.page = 1
  pagination.page_size = 50
}

const priorityKeys = ['openid', 'msg_type', 'chat_type', 'sender', 'roomname', 'tolist', 'content']

const isUnixTimestamp = (key: string, val: unknown): boolean => {
  if (typeof val !== 'number') return false
  if (!key.endsWith('_time') && !key.endsWith('time') && key !== 'log_time') return false
  return val > 1_000_000_000 && val < 2_000_000_000
}

const formatTimestamp = (val: number): string => {
  const d = new Date(val * 1000)
  return d.toLocaleString('zh-CN')
}

const formatValue = (key: string, val: unknown): unknown => {
  if (isUnixTimestamp(key, val)) {
    return `${val} (${formatTimestamp(val as number)})`
  }
  if (typeof val === 'object' && val !== null) {
    const formatted: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(val)) {
      formatted[k] = formatValue(k, v)
    }
    return formatted
  }
  return val
}

const formatData = (row: LogRow) => {
  const { feature_id, log_date, _decrypt_failed, ...rest } = row
  const sorted: Record<string, unknown> = {}
  for (const k of priorityKeys) {
    if (k in rest) {
      sorted[k] = formatValue(k, rest[k])
      delete rest[k]
    }
  }
  for (const [k, v] of Object.entries(rest)) {
    sorted[k] = formatValue(k, v)
  }
  return JSON.stringify(sorted, null, 2)
}

const formatPreview = (row: LogRow) => {
  const { feature_id, log_date, _decrypt_failed, ...rest } = row
  if (_decrypt_failed) return '[解密失败]'
  const parts: string[] = []
  if (rest.openid) {
    const name = contactNames.value[rest.openid]
    parts.push(name ? `${rest.openid}(${name})` : rest.openid)
  }
  if (rest.msg_type != null) parts.push(`msg:${rest.msg_type}`)
  if (rest.chat_type != null) parts.push(`chat:${rest.chat_type}`)
  if (rest.sender?.openid) {
    const name = contactNames.value[rest.sender.openid]
    parts.push(name ? `from:${rest.sender.openid}(${name})` : `from:${rest.sender.openid}`)
  }
  if (parts.length > 0) return parts.join(' | ')
  const str = JSON.stringify(rest)
  return str.length > 80 ? str.slice(0, 80) + '...' : str
}

const isRowExpanded = (index: number) => {
  return expandedRows.value.includes(index)
}

const toggleRow = (index: number) => {
  const idx = expandedRows.value.indexOf(index)
  if (idx >= 0) {
    expandedRows.value.splice(idx, 1)
  } else {
    expandedRows.value.push(index)
  }
}

const handleExport = async () => {
  if (form.feature_ids.length === 0 || !dateRange.value) {
    ElMessage.warning('请先执行查询')
    return
  }

  try {
    const exportForm = {
      feature_ids: form.feature_ids,
      start_time: Math.floor(dateRange.value[0].getTime() / 1000),
      end_time: Math.floor(dateRange.value[1].getTime() / 1000),
      mobile: form.mobile,
      conditions: buildConditions(),
      page: 1,
      page_size: 20000,
      realtime: false,
    }

    const res = await fetch(logAPI.exportCSVURL(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
      },
      body: JSON.stringify(exportForm),
    })

    if (!res.ok) {
      throw new Error('导出失败')
    }

    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `query_result_${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '导出失败')
  }
}
</script>

<style scoped>
.log-query {
  padding: 0;
}

.query-card,
.result-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.time-range-container {
  width: 100%;
}

.time-shortcuts {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.time-shortcuts .el-button {
  border-radius: 4px;
}

.conditions-container {
  width: 100%;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  padding: 12px;
  background-color: #fafafa;
}

.conditions-header {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}

.empty-conditions {
  color: #909399;
  text-align: center;
  padding: 20px;
  font-size: 14px;
}

.conditions-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.condition-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  background-color: #fff;
  border-radius: 4px;
  border: 1px solid #e4e7ed;
}

.result-count {
  font-size: 14px;
  color: #909399;
  font-weight: normal;
}

.data-content {
  max-width: 600px;
  overflow-x: auto;
  font-size: 12px;
  margin: 0;
  padding: 8px;
  background-color: #f5f7fa;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
}

.data-cell {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.data-preview {
  font-size: 12px;
  color: #606266;
  cursor: pointer;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  word-break: break-all;
}

.data-preview:hover {
  color: #409eff;
}

.pagination {
  margin-top: 16px;
  justify-content: center;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: #606266;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
