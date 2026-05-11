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
            <el-form-item label="数据类型">
              <el-select
                v-model="form.feature_ids"
                multiple
                placeholder="请选择数据类型（可多选）"
                style="width: 100%"
                collapse-tags
                collapse-tags-tooltip
              >
                <template #prefix>
                  <el-icon><DataAnalysis /></el-icon>
                </template>
                <el-option
                  v-for="item in features"
                  :key="item.id"
                  :label="item.name"
                  :value="item.id"
                >
                  <span style="float: left">{{ item.name }}</span>
                  <span style="float: right; color: #8492a6; font-size: 12px">{{ item.id }}</span>
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
          <el-col :span="24">
            <el-form-item label="筛选条件">
              <div class="conditions-container">
                <div class="conditions-header">
                  <el-button type="primary" link @click="addCondition">
                    <el-icon><Plus /></el-icon>
                    添加条件
                  </el-button>
                  <el-button v-if="conditions.length > 0" type="danger" link @click="clearConditions">
                    <el-icon><Delete /></el-icon>
                    清空
                  </el-button>
                </div>
                <div v-if="conditions.length === 0" class="empty-conditions">
                  暂无筛选条件，点击"添加条件"开始配置
                </div>
                <div v-else class="conditions-list">
                  <div v-for="(condition, index) in conditions" :key="index" class="condition-item">
                    <el-input
                      v-model="condition.key"
                      placeholder="字段名"
                      style="width: 150px"
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
          <span v-if="total > 0" class="result-count">共 {{ total }} 条记录</span>
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
        <el-table-column prop="feature_id" label="FeatureID" width="100" align="center" />
        <el-table-column prop="log_date" label="时间" width="180" align="center" />
        <el-table-column prop="idc" label="IDC" width="100" align="center" />
        <el-table-column label="数据内容" min-width="400">
          <template #default="{ row }">
            <pre class="data-content">{{ formatData(row) }}</pre>
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
import { ref, reactive, onMounted } from 'vue'
import { logAPI } from '../api'
import { ElMessage } from 'element-plus'
import { Search, Refresh, DataAnalysis, Plus, Delete, Close } from '@element-plus/icons-vue'

interface Condition {
  key: string
  operator: string
  value: string
}

const form = reactive({
  feature_ids: [] as number[],
  start_time: 0,
  end_time: 0,
  conditions: null as any,
})

const dateRange = ref<[Date, Date] | null>(null)
const activeShortcut = ref<string | null>(null)
const conditions = ref<Condition[]>([])
const loading = ref(false)
const tableData = ref<any[]>([])
const total = ref(0)
const pagination = reactive({
  page: 1,
  page_size: 50,
})
const features = ref<any[]>([])

const timeShortcuts = [
  { label: '最近1小时', hours: 1 },
  { label: '最近6小时', hours: 6 },
  { label: '最近1天', hours: 24 },
  { label: '最近7天', hours: 168 },
  { label: '最近30天', hours: 720 },
]

onMounted(async () => {
  try {
    const res: any = await logAPI.getFeatures()
    if (res.code === 0) {
      features.value = res.data
    }
    const timeRes: any = await logAPI.getTimeRange()
    if (timeRes.code === 0) {
      dateRange.value = [
        new Date(timeRes.data.start_time * 1000),
        new Date(timeRes.data.end_time * 1000),
      ]
    }
  } catch (err) {
    ElMessage.error('加载数据失败')
  }
})

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
}

const clearConditions = () => {
  conditions.value = []
}

const buildConditions = () => {
  if (conditions.value.length === 0) return null

  const result: any = {}
  for (const condition of conditions.value) {
    if (condition.key && condition.value) {
      if (condition.operator === 'like') {
        result[condition.key] = condition.value
      } else {
        result[condition.key] = condition.value
      }
    }
  }
  return Object.keys(result).length > 0 ? result : null
}

const handleQuery = async () => {
  if (form.feature_ids.length === 0) {
    ElMessage.warning('请选择至少一个数据类型')
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
    form.conditions = buildConditions()

    const res: any = await logAPI.query({
      ...form,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    if (res.code === 0) {
      tableData.value = res.data.data
      total.value = res.data.total
    } else {
      ElMessage.error(res.msg || '查询失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '查询失败')
  } finally {
    loading.value = false
  }
}

const handleReset = () => {
  form.feature_ids = []
  form.conditions = null
  conditions.value = []
  activeShortcut.value = null
  tableData.value = []
  total.value = 0
  pagination.page = 1
  pagination.page_size = 50
}

const formatData = (row: any) => {
  const { feature_id, log_date, idc, ...rest } = row
  return JSON.stringify(rest, null, 2)
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