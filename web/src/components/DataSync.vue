<template>
  <div class="data-sync">
    <el-card class="sync-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">数据同步</span>
          <el-tag :type="syncStatus.running ? 'warning' : 'success'" size="large">
            {{ syncStatus.running ? '同步中...' : '空闲' }}
          </el-tag>
        </div>
      </template>

      <el-form label-position="top">
        <el-row :gutter="20">
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
          <el-col :span="12">
            <el-form-item label="同步选项">
              <div class="sync-options">
                <el-checkbox v-model="syncAll" :disabled="syncStatus.running">
                  同步所有数据类型 ({{ features.length }} 项)
                </el-checkbox>
                <el-select
                  v-if="!syncAll"
                  v-model="form.feature_ids"
                  multiple
                  placeholder="请选择要同步的数据类型"
                  style="width: 100%; margin-top: 8px"
                  :disabled="syncStatus.running"
                  collapse-tags
                  collapse-tags-tooltip
                >
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
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item>
          <el-button
            type="primary"
            @click="handleSync"
            :loading="syncStatus.running"
            :disabled="syncStatus.running"
            size="large"
          >
            <el-icon><VideoPlay /></el-icon>
            {{ syncStatus.running ? '同步进行中...' : '开始同步' }}
          </el-button>
          <el-button @click="handleReset" :disabled="syncStatus.running" size="large">
            <el-icon><Refresh /></el-icon>
            重置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="status-card">
      <template #header>
        <span class="card-title">同步状态</span>
      </template>

      <div v-if="syncStatus.running" class="sync-progress">
        <el-progress
          :percentage="progressPercentage"
          :stroke-width="20"
          :show-text="true"
          :format="progressFormat"
          status="success"
        />
        <p class="progress-text">正在同步第 {{ syncStatus.progress }} / {{ syncStatus.total }} 个数据类型...</p>
      </div>

      <div v-else-if="syncStatus.last_sync && syncStatus.last_sync !== '0001-01-01T00:00:00Z'" class="last-sync">
        <el-icon><Clock /></el-icon>
        <span>上次同步时间: {{ formatTime(syncStatus.last_sync) }}</span>
      </div>

      <div v-if="syncStatus.results && Object.keys(syncStatus.results).length > 0" class="sync-results">
        <h4>同步结果</h4>
        <el-table :data="resultTableData" border max-height="300" size="small">
          <el-table-column prop="feature_id" label="Feature ID" width="120" align="center" />
          <el-table-column prop="name" label="数据类型" min-width="150" />
          <el-table-column prop="count" label="同步数量" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="row.count >= 0 ? 'success' : 'danger'">
                {{ row.count >= 0 ? row.count : '失败' }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-empty v-if="!syncStatus.running && (!syncStatus.results || Object.keys(syncStatus.results).length === 0)" description="暂无同步记录" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { syncAPI, logAPI } from '../api'
import { ElMessage } from 'element-plus'
import { VideoPlay, Refresh, Clock } from '@element-plus/icons-vue'

const dateRange = ref<[Date, Date] | null>(null)
const activeShortcut = ref<string | null>(null)
const syncStatus = ref<any>({
  running: false,
  progress: 0,
  total: 0,
  results: {}
})
const syncAll = ref(true)
const form = reactive({
  feature_ids: [] as number[],
})
const features = ref<any[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

const timeShortcuts = [
  { label: '最近1天', hours: 24 },
  { label: '最近7天', hours: 168 },
  { label: '最近30天', hours: 720 },
  { label: '最近90天', hours: 2160 },
]

const progressPercentage = computed(() => {
  if (!syncStatus.value || syncStatus.value.total === 0) return 0
  return Math.round((syncStatus.value.progress / syncStatus.value.total) * 100)
})

const progressFormat = (percentage: number) => `${percentage}%`

const resultTableData = computed(() => {
  if (!syncStatus.value?.results) return []
  return Object.entries(syncStatus.value.results).map(([featureId, count]) => ({
    feature_id: featureId,
    name: getFeatureName(Number(featureId)),
    count: count as number,
  }))
})

const getFeatureName = (featureId: number) => {
  const feature = features.value.find(f => f.id === featureId)
  return feature ? feature.name : `Feature ${featureId}`
}

const formatTime = (timeStr: string) => {
  if (!timeStr) return '-'
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN')
}

onMounted(async () => {
  try {
    const res: any = await logAPI.getFeatures()
    if (res.code === 0) {
      features.value = res.data
    }
    await checkStatus()
    // 如果正在同步，启动轮询
    if (syncStatus.value.running) {
      startPolling()
    }
  } catch (err) {
    console.error(err)
  }
})

onUnmounted(() => {
  stopPolling()
})

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(async () => {
    await checkStatus()
    // 如果同步完成，停止轮询
    if (syncStatus.value && !syncStatus.value.running) {
      stopPolling()
    }
  }, 2000)
}

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
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

const handleSync = async () => {
  if (!dateRange.value) {
    ElMessage.warning('请选择时间范围')
    return
  }

  if (!syncAll.value && form.feature_ids.length === 0) {
    ElMessage.warning('请选择至少一个数据类型')
    return
  }

  try {
    const startTime = Math.floor(dateRange.value[0].getTime() / 1000)
    const endTime = Math.floor(dateRange.value[1].getTime() / 1000)

    const res: any = await syncAPI.sync({
      sync_all: syncAll.value,
      feature_ids: form.feature_ids,
      start_time: startTime,
      end_time: endTime,
    })

    if (res.code === 0) {
      ElMessage.success('同步已启动')
      startPolling()
    } else {
      ElMessage.error(res.msg || '同步启动失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '同步启动失败')
  }
}

const handleReset = () => {
  dateRange.value = null
  activeShortcut.value = null
  syncAll.value = true
  form.feature_ids = []
}

const checkStatus = async () => {
  try {
    const res: any = await syncAPI.status()
    if (res.code === 0) {
      syncStatus.value = res.data
    }
  } catch (err) {
    console.error(err)
  }
}
</script>

<style scoped>
.data-sync {
  padding: 0;
}

.sync-card,
.status-card {
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

.sync-options {
  width: 100%;
}

.sync-progress {
  margin-bottom: 20px;
}

.progress-text {
  text-align: center;
  color: #606266;
  margin-top: 10px;
}

.last-sync {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #909399;
  margin-bottom: 20px;
}

.sync-results h4 {
  margin: 0 0 12px 0;
  font-size: 14px;
  color: #303133;
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