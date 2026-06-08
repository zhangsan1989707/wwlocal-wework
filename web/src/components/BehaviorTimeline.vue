<template>
  <div class="behavior-timeline">
    <el-card class="query-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">行为查询</span>
          <div class="header-actions">
            <el-button type="primary" :loading="loading" @click="handleQuery">
              <el-icon><Search /></el-icon>
              查询
            </el-button>
            <el-button type="success" :loading="exporting" @click="handleExport">
              <el-icon><Download /></el-icon>
              导出 CSV
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
          <el-col :span="8">
            <el-form-item label="OpenID / 手机号">
              <el-input v-model="form.openid" placeholder="输入要追踪的 openid" clearable />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="日志类型">
              <el-select
                v-model="form.feature_ids"
                multiple
                collapse-tags
                collapse-tags-tooltip
                placeholder="默认查询已启用类型"
                style="width: 100%"
              >
                <el-option
                  v-for="item in features"
                  :key="item.feature_id"
                  :label="item.name"
                  :value="item.feature_id"
                >
                  <span style="float: left">{{ item.name }}</span>
                  <span class="feature-id">{{ item.feature_id }}</span>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="时间范围">
              <div class="time-range">
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
                  range-separator="至"
                  start-placeholder="开始时间"
                  end-placeholder="结束时间"
                  style="width: 100%; margin-top: 8px"
                  @change="activeShortcut = null"
                />
              </div>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-card>

    <el-card class="result-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">行为时间线</span>
          <span v-if="total > 0" class="result-count">共 {{ total }} 条记录</span>
        </div>
      </template>

      <div v-if="featureSummaries.length > 0" class="query-scope">
        <el-table :data="featureSummaries" size="small" stripe border max-height="220">
          <el-table-column prop="feature_id" label="类型编号" width="110" align="center" />
          <el-table-column prop="feature_name" label="日志类型" min-width="160" show-overflow-tooltip />
          <el-table-column label="状态" width="110" align="center">
            <template #default="{ row }">
              <el-tag :type="featureStatusType(row.status)" size="small">
                {{ featureStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="tables" label="月表" width="80" align="center" />
          <el-table-column prop="queried_tables" label="已查表" width="80" align="center" />
          <el-table-column prop="matched_rows" label="命中" width="80" align="center" />
          <el-table-column prop="reason" label="说明" min-width="220" show-overflow-tooltip />
        </el-table>
      </div>

      <el-table
        :data="records"
        border
        stripe
        v-loading="loading"
        style="width: 100%"
        max-height="560"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <pre class="json-detail">{{ formatJSON(row.data) }}</pre>
          </template>
        </el-table-column>
        <el-table-column prop="log_date" label="时间" width="180" align="center" />
        <el-table-column label="日志类型" width="180">
          <template #default="{ row }">
            <div class="feature-cell">
              <span>{{ row.feature_name }}</span>
              <el-tag size="small" effect="plain">{{ row.feature_id }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="命中字段" min-width="220">
          <template #default="{ row }">
            <div class="matched-fields">
              <el-tag
                v-for="field in row.matched_fields"
                :key="field.field"
                size="small"
                type="success"
                effect="light"
              >
                {{ field.label }}: {{ field.display_value || field.value || field.field }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="摘要" min-width="320">
          <template #default="{ row }">
            <span class="summary">{{ formatSummary(row) }}</span>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && records.length === 0" description="暂无行为记录" />

      <el-pagination
        v-if="total > 0"
        class="pagination"
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[20, 50, 100, 200]"
        layout="total, sizes, prev, pager, next"
        background
        @current-change="handleQuery"
        @size-change="handleQuery"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, Download } from '@element-plus/icons-vue'
import { logAPI, syncFeatureAPI, contactAPI } from '../api'
import type { BehaviorFeatureSummary, BehaviorRecord, SyncFeature } from '../types/api'

interface FeatureItem {
  feature_id: number
  name: string
}

const form = reactive({
  openid: '',
  feature_ids: [] as number[],
})
const pagination = reactive({
  page: 1,
  page_size: 50,
})
const dateRange = ref<[Date, Date] | null>(null)
const activeShortcut = ref<string | null>('最近7天')
const features = ref<FeatureItem[]>([])
const records = ref<BehaviorRecord[]>([])
const featureSummaries = ref<BehaviorFeatureSummary[]>([])
const total = ref(0)
const loading = ref(false)
const exporting = ref(false)
const contactNames = ref<Record<string, string>>({})

const timeShortcuts = [
  { label: '最近1天', hours: 24 },
  { label: '最近7天', hours: 168 },
  { label: '最近14天', hours: 336 },
  { label: '最近31天', hours: 744 },
]

onMounted(async () => {
  applyTimeShortcut(timeShortcuts[1])
  try {
    const res = await syncFeatureAPI.list()
    if (res.code === 0) {
      features.value = (res.data || []).map((item: SyncFeature) => ({
        feature_id: item.feature_id,
        name: item.name,
      }))
    }
  } catch {
    ElMessage.error('加载日志类型失败')
  }
})

const applyTimeShortcut = (shortcut: { label: string; hours: number }) => {
  activeShortcut.value = shortcut.label
  const end = new Date()
  const start = new Date(end.getTime() - shortcut.hours * 60 * 60 * 1000)
  dateRange.value = [start, end]
}

const handleQuery = async () => {
  if (!validateQueryForm()) {
    return
  }

  loading.value = true
  try {
    const res = await logAPI.behaviorQuery({
      ...buildQueryPayload(pagination.page_size),
      page: pagination.page,
    })
    if (res.code === 0 && res.data) {
      records.value = res.data.data
      featureSummaries.value = res.data.features || []
      total.value = res.data.total
      await fetchContactNames(records.value)
    } else {
      ElMessage.error(res.msg || '查询失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '查询失败')
  } finally {
    loading.value = false
  }
}

const validateQueryForm = () => {
  if (!form.openid.trim()) {
    ElMessage.warning('请输入 OpenID 或手机号')
    return false
  }
  if (!dateRange.value) {
    ElMessage.warning('请选择时间范围')
    return false
  }
  return true
}

const buildQueryPayload = (pageSize: number) => ({
  openid: form.openid.trim(),
  feature_ids: form.feature_ids,
  start_time: Math.floor((dateRange.value as [Date, Date])[0].getTime() / 1000),
  end_time: Math.floor((dateRange.value as [Date, Date])[1].getTime() / 1000),
  page: 1,
  page_size: pageSize,
})

const handleExport = async () => {
  if (!validateQueryForm()) {
    return
  }
  exporting.value = true
  try {
    const res = await fetch(logAPI.behaviorExportCSVURL(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
      },
      body: JSON.stringify(buildQueryPayload(50000)),
    })
    if (!res.ok) {
      throw new Error('导出失败')
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `behavior_timeline_${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '导出失败')
  } finally {
    exporting.value = false
  }
}

const handleReset = () => {
  form.openid = ''
  form.feature_ids = []
  pagination.page = 1
  pagination.page_size = 50
  records.value = []
  featureSummaries.value = []
  total.value = 0
  applyTimeShortcut(timeShortcuts[1])
}

const formatJSON = (data: Record<string, unknown>) => {
  return JSON.stringify(data, null, 2)
}

const extractContactIds = (data: BehaviorRecord[]) => {
  const ids = new Set<string>()
  const walk = (value: unknown) => {
    if (Array.isArray(value)) {
      value.forEach(walk)
      return
    }
    if (value && typeof value === 'object') {
      const obj = value as Record<string, unknown>
      for (const key of ['openid', 'userid', 'user_id', 'sender_openid', 'receiver_openid']) {
        const item = obj[key]
        if (typeof item === 'string' && item.trim()) {
          ids.add(item.trim())
        }
      }
      Object.values(obj).forEach(walk)
    }
  }

  for (const row of data) {
    row.matched_fields.forEach((field) => {
      if (field.value) ids.add(field.value)
    })
    walk(row.data)
  }
  return Array.from(ids).slice(0, 200)
}

const fetchContactNames = async (data: BehaviorRecord[]) => {
  const ids = extractContactIds(data).filter((id) => !contactNames.value[id])
  if (ids.length === 0) return
  try {
    const res = await contactAPI.getNames(ids)
    if (res.code === 0 && res.data) {
      contactNames.value = { ...contactNames.value, ...res.data }
    }
  } catch {
    // ignore contact name misses
  }
}

const formatContactValue = (value: unknown) => {
  if (typeof value !== 'string' || value === '') {
    return value
  }
  const name = contactNames.value[value]
  return name ? `${value}(${name})` : value
}

const formatSummary = (row: BehaviorRecord) => {
  const data = row.data
  const keys = ['msgid', 'msg_type', 'chatid', 'name', 'deviceid', 'cli_ip', 'access_ip']
  const parts: string[] = []
  for (const key of keys) {
    const value = data[key]
    if (value !== undefined && value !== null && value !== '') {
      parts.push(`${key}: ${String(value)}`)
    }
  }
  for (const field of row.matched_fields) {
    parts.push(`${field.label}: ${field.display_value || formatContactValue(field.value)}`)
  }
  if (parts.length > 0) return parts.join(' | ')
  const text = JSON.stringify(data)
  return text.length > 120 ? text.slice(0, 120) + '...' : text
}

const featureStatusType = (status: BehaviorFeatureSummary['status']) => {
  if (status === 'queried') return 'success'
  if (status === 'no_match') return 'warning'
  return 'info'
}

const featureStatusText = (status: BehaviorFeatureSummary['status']) => {
  if (status === 'queried') return '已命中'
  if (status === 'no_match') return '未命中'
  return '已跳过'
}
</script>

<style scoped>
.behavior-timeline {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  font-weight: 600;
  color: #1a365d;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.feature-id {
  float: right;
  color: #8492a6;
  font-size: 12px;
}

.feature-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.time-shortcuts {
  display: flex;
  flex-wrap: wrap;
}

.matched-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.summary {
  color: #4a5568;
  font-size: 13px;
}

.json-detail {
  margin: 0;
  padding: 12px;
  background: #f8fafc;
  border-radius: 6px;
  color: #2d3748;
  font-size: 12px;
  line-height: 1.5;
  max-height: 360px;
  overflow: auto;
}

.result-count {
  color: #4a5568;
  font-size: 13px;
}

.query-scope {
  margin-bottom: 14px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
