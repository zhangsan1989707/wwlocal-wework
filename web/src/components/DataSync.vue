<template>
  <div class="data-sync">
    <el-card class="sync-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">定期同步</span>
          <el-tag :type="schedulerStatus.running ? 'success' : 'info'" size="large">
            {{ schedulerStatus.running ? '运行中' : '已停止' }}
          </el-tag>
        </div>
      </template>

      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="同步间隔">{{ schedulerStatus.interval || '-' }}</el-descriptions-item>
        <el-descriptions-item label="上次执行">{{ formatTime(schedulerStatus.last_run) }}</el-descriptions-item>
        <el-descriptions-item label="下次执行">
          {{ schedulerStatus.running ? formatTime(schedulerStatus.next_run) : '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="schedulerStatus.running ? 'success' : 'info'" size="small">
            {{ schedulerStatus.running ? '定时任务运行中' : '定时任务已停止' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <div class="scheduler-actions" style="margin-top: 16px">
        <el-button
          v-if="!schedulerStatus.running"
          type="success"
          @click="handleSchedulerStart"
          size="large"
        >
          <el-icon><VideoPlay /></el-icon>
          启动定时同步
        </el-button>
        <el-select
          v-if="!schedulerStatus.running"
          v-model="startDelay"
          size="large"
          style="width: 130px"
        >
          <el-option label="10分钟后开始" value="10m" />
          <el-option label="30分钟后开始" value="30m" />
          <el-option label="1小时后开始" value="1h" />
        </el-select>
        <el-button
          v-else
          type="danger"
          @click="handleSchedulerStop"
          size="large"
        >
          <el-icon><VideoPause /></el-icon>
          停止定时同步
        </el-button>
        <el-button
          type="primary"
          @click="handleIncrementalSync"
          :loading="syncStatus.running"
          :disabled="syncStatus.running"
          size="large"
          plain
        >
          <el-icon><Refresh /></el-icon>
          同步新增数据
        </el-button>
      </div>
    </el-card>

    <el-card class="sync-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">按时间范围同步</span>
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
                  同步所有日志类型 ({{ features.length }} 项)
                </el-checkbox>
                <el-select
                  v-if="!syncAll"
                  v-model="form.feature_ids"
                  multiple
                  placeholder="请选择要同步的日志类型"
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
        <p class="progress-text">正在同步第 {{ syncStatus.progress }} / {{ syncStatus.total }} 个日志类型<span v-if="syncStatus.current_feature"> (当前: {{ syncStatus.current_feature }})</span>...</p>
        <el-button type="danger" size="small" @click="handleCancel" style="margin-top: 12px">
          请求停止同步
        </el-button>
      </div>

      <div v-else-if="syncStatus.last_sync && syncStatus.last_sync !== '0001-01-01T00:00:00Z'" class="last-sync">
        <el-icon><Clock /></el-icon>
        <span>上次同步时间: {{ formatTime(syncStatus.last_sync) }}</span>
        <el-tag v-if="syncStatus.failed > 0" type="danger" size="small" style="margin-left: 12px">
          {{ syncStatus.failed }} 条解密/写入失败
        </el-tag>
      </div>

      <div v-if="syncStatus.results && Object.keys(syncStatus.results).length > 0" class="sync-results">
        <h4>同步结果</h4>
        <el-table :data="resultTableData" border max-height="300" size="small">
          <el-table-column prop="feature_id" label="日志类型编号" width="120" align="center" />
          <el-table-column prop="name" label="数据类型" min-width="150" />
          <el-table-column prop="count" label="同步数量" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="row.count >= 0 ? 'success' : 'danger'">
                {{ row.count >= 0 ? row.count : '失败' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="error" label="错误信息" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.error" class="error-text">{{ row.error }}</span>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" align="center">
            <template #default="{ row }">
              <el-button
                v-if="row.count < 0 || row.error"
                type="primary"
                size="small"
                link
                @click="handleRetryFeature(row.feature_id)"
              >
                重试
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-empty v-if="!syncStatus.running && (!syncStatus.results || Object.keys(syncStatus.results).length === 0)" description="暂无同步记录" />
    </el-card>

    <el-card class="history-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">同步历史</span>
          <el-button size="small" @click="loadSyncHistory" :loading="historyLoading">
            刷新
          </el-button>
        </div>
      </template>
      <el-table :data="syncHistory" stripe size="small" v-loading="historyLoading">
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(row.start_time) }}</template>
        </el-table-column>
        <el-table-column prop="sync_type" label="类型" width="90">
          <template #default="{ row }">
            <el-tag :type="row.sync_type === 'log' ? 'primary' : 'success'" size="small">
              {{ row.sync_type === 'log' ? '日志' : '通讯录' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="trigger" label="触发方式" width="90">
          <template #default="{ row }">
            <el-tag :type="row.trigger === 'scheduler' ? 'warning' : 'info'" size="small">
              {{ row.trigger === 'scheduler' ? '定时' : '手动' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="total" label="总数" width="70" align="center" />
        <el-table-column prop="succeeded" label="成功" width="70" align="center">
          <template #default="{ row }">
            <span :class="row.succeeded > 0 ? 'success-text' : ''">{{ row.succeeded }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="failed" label="失败" width="70" align="center">
          <template #default="{ row }">
            <span :class="row.failed > 0 ? 'error-text' : ''">{{ row.failed }}</span>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="90" align="center">
          <template #default="{ row }">{{ formatDuration(row.duration_ms) }}</template>
        </el-table-column>
        <el-table-column prop="error_msg" label="错误信息" min-width="180" show-overflow-tooltip />
      </el-table>
      <el-pagination
        v-if="historyTotal > 0"
        :current-page="historyPage"
        :page-size="historyPageSize"
        :total="historyTotal"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="handleHistoryPageChange"
        @size-change="handleHistorySizeChange"
        style="margin-top: 12px; justify-content: center"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { syncAPI, schedulerAPI, syncHistoryAPI, syncFeatureAPI } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { VideoPlay, VideoPause, Refresh, Clock } from '@element-plus/icons-vue'

const dateRange = ref<[Date, Date] | null>(null)
const activeShortcut = ref<string | null>(null)
const syncStatus = ref<any>({
  running: false,
  progress: 0,
  total: 0,
  results: {}
})
const schedulerStatus = ref<any>({
  running: false,
  interval: '',
})
const startDelay = ref('1h')
const syncAll = ref(true)
const form = reactive({
  feature_ids: [] as number[],
})
const features = ref<any[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

const syncHistory = ref<any[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyPageSize = ref(10)
const historyLoading = ref(false)

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
  const errors = syncStatus.value?.errors || {}
  return Object.entries(syncStatus.value.results).map(([featureId, count]) => ({
    feature_id: featureId,
    name: getFeatureName(Number(featureId)),
    count: count as number,
    error: errors[featureId] || '',
  }))
})

const getFeatureName = (featureId: number) => {
  const feature = features.value.find(f => f.id === featureId)
  return feature ? feature.name : `Feature ${featureId}`
}

const formatTime = (timeStr: string) => {
  if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return '-'
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN')
}

onMounted(async () => {
  try {
    const res: any = await syncFeatureAPI.list()
    if (res.code === 0) {
      // 只显示启用的 feature 用于同步下拉
      features.value = (res.data || []).filter((f: any) => f.enabled)
    }
  } catch (err) {
    console.error(err)
  }
  await checkStatus()
  await checkSchedulerStatus()
  await loadSyncHistory()
  if (syncStatus.value.running) startPolling()
})

onUnmounted(() => {
  stopPolling()
})

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(async () => {
    await checkStatus()
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

const handleSchedulerStart = async () => {
  try {
    const res: any = await schedulerAPI.start({ start_delay: startDelay.value })
    if (res.code === 0) {
      ElMessage.success('定时同步已启动')
      schedulerStatus.value = res.data
    }
  } catch (err: any) {
    ElMessage.error(err.message || '启动失败')
  }
}

const handleSchedulerStop = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要停止定时同步吗？',
      '确认停止',
      { type: 'warning', confirmButtonText: '停止', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await schedulerAPI.stop()
    if (res.code === 0) {
      ElMessage.success('定时同步已停止')
      schedulerStatus.value = res.data
    }
  } catch (err: any) {
    ElMessage.error(err.message || '停止失败')
  }
}

const handleIncrementalSync = async () => {
  if (syncStatus.value.running) {
    ElMessage.warning('已有同步任务在执行中')
    return
  }

  try {
    await ElMessageBox.confirm(
      '同步新增数据将从上次同步进度继续拉取新数据，确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await schedulerAPI.incrementalSync({ sync_all: true })
    if (res.code === 0) {
      ElMessage.success('同步任务已启动')
      startPolling()
    } else {
      ElMessage.error(res.msg || '增量同步启动失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '增量同步启动失败')
  }
}

const handleRetryFeature = async (featureId: number) => {
  if (syncStatus.value.running) {
    ElMessage.warning('已有同步任务在执行中')
    return
  }

  try {
    await ElMessageBox.confirm(
      `将重新同步日志类型 ${featureId} 的数据，确定开始吗？`,
      '确认重试',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await syncAPI.sync({
      sync_all: false,
      feature_ids: [featureId],
      start_time: 0,
      end_time: 0,
    })
    if (res.code === 0) {
      ElMessage.success('重试同步已启动')
      startPolling()
    } else {
      ElMessage.error(res.msg || '重试启动失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '重试启动失败')
  }
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
    await ElMessageBox.confirm(
      '将按选定时间范围从政务微信拉取数据并解密存储，确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch {
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
      ElMessage.success('全量同步已启动')
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

const handleCancel = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要取消当前同步任务吗？已完成的部分会保留。',
      '确认取消',
      { type: 'warning', confirmButtonText: '确定取消', cancelButtonText: '继续同步' }
    )
  } catch {
    return
  }

  try {
    const res: any = await syncAPI.cancel()
    if (res.code === 0) {
      ElMessage.success('已发送取消请求')
    } else {
      ElMessage.error(res.msg || '取消失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '取消失败')
  }
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

const checkSchedulerStatus = async () => {
  try {
    const res: any = await schedulerAPI.status()
    if (res.code === 0) {
      schedulerStatus.value = res.data
    }
  } catch (err) {
    console.error(err)
  }
}

const loadSyncHistory = async () => {
  historyLoading.value = true
  try {
    const res: any = await syncHistoryAPI.list({
      page: historyPage.value,
      page_size: historyPageSize.value,
    })
    if (res.code === 0) {
      syncHistory.value = res.data.data || []
      historyTotal.value = res.data.total || 0
    }
  } catch (err) {
    console.error(err)
  } finally {
    historyLoading.value = false
  }
}

const handleHistoryPageChange = (p: number) => {
  historyPage.value = p
  loadSyncHistory()
}

const handleHistorySizeChange = (size: number) => {
  historyPageSize.value = size
  historyPage.value = 1
  loadSyncHistory()
}

const formatDuration = (ms: number) => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const min = Math.floor(ms / 60000)
  const sec = Math.round((ms % 60000) / 1000)
  return `${min}m${sec}s`
}

</script>

<style scoped>
.data-sync {
  padding: 0;
}

.sync-card,
.status-card,
.history-card {
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

.scheduler-actions {
  display: flex;
  gap: 12px;
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

.error-text {
  color: #f56c6c;
  font-size: 12px;
}

.success-text {
  color: #67c23a;
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
