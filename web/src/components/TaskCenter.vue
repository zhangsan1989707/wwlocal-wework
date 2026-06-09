<template>
  <div class="task-center">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">任务中心</span>
          <el-button type="primary" size="small" :loading="loading" @click="loadTasks">
            刷新
          </el-button>
        </div>
      </template>

      <el-alert
        v-if="queueDisabled"
        title="任务队列未启用"
        description="Redis 未连接或任务队列配置不可用，当前只能使用同步页面的直接任务状态。"
        type="warning"
        show-icon
        :closable="false"
        class="status-alert"
      />

      <el-table :data="tasks" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="type" label="任务类型" width="130">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ taskTypeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进度" width="170">
          <template #default="{ row }">
            <el-progress
              :percentage="progressPercent(row)"
              :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : undefined"
              :stroke-width="12"
            />
          </template>
        </el-table-column>
        <el-table-column label="范围" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            {{ formatRange(row) }}
          </template>
        </el-table-column>
        <el-table-column label="日志类型" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.feature_ids?.length ? row.feature_ids.join(', ') : '全部' }}
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="错误" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span :class="{ 'error-text': row.error }">{{ row.error || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="showDetail(row)">详情</el-button>
            <el-button
              v-if="row.status === 'pending'"
              size="small"
              type="danger"
              link
              @click="cancelTask(row)"
            >
              取消
            </el-button>
            <el-button
              v-if="row.status === 'failed'"
              size="small"
              type="primary"
              link
              @click="retryTask(row)"
            >
              重试
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && !queueDisabled && tasks.length === 0" description="暂无任务" />
    </el-card>

    <el-dialog v-model="detailVisible" title="任务详情" width="720px">
      <el-descriptions v-if="selectedTask" :column="2" border size="small">
        <el-descriptions-item label="任务ID" :span="2">{{ selectedTask.id }}</el-descriptions-item>
        <el-descriptions-item label="任务类型">{{ taskTypeText(selectedTask.type) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ statusText(selectedTask.status) }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ formatTime(selectedTask.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="更新时间">{{ formatTime(selectedTask.updated_at) }}</el-descriptions-item>
        <el-descriptions-item label="时间范围" :span="2">{{ formatRange(selectedTask) }}</el-descriptions-item>
        <el-descriptions-item label="错误" :span="2">{{ selectedTask.error || '-' }}</el-descriptions-item>
      </el-descriptions>
      <pre v-if="selectedTask?.result" class="json-detail">{{ formatJSON(selectedTask.result) }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { taskAPI } from '../api'
import type { TaskInfo } from '../types/api'

const tasks = ref<TaskInfo[]>([])
const loading = ref(false)
const queueDisabled = ref(false)
const detailVisible = ref(false)
const selectedTask = ref<TaskInfo | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  await loadTasks()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})

const loadTasks = async () => {
  loading.value = true
  try {
    const res = await taskAPI.list()
    if (res.code === 0) {
      tasks.value = res.data || []
      queueDisabled.value = false
    } else {
      ElMessage.error(res.msg || '加载任务失败')
    }
  } catch (err: unknown) {
    tasks.value = []
    queueDisabled.value = err instanceof Error && err.message.includes('disabled')
    if (!queueDisabled.value) {
      ElMessage.error(err instanceof Error ? err.message : '加载任务失败')
    }
  } finally {
    loading.value = false
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(() => {
    if (!detailVisible.value) {
      loadTasks()
    }
  }, 5000)
}

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

const taskTypeText = (type: TaskInfo['type']) => {
  if (type === 'log_sync') return '开放数据同步'
  if (type === 'contact_sync') return '通讯录同步'
  if (type === 'admin_log_sync') return '企微操作日志'
  return type
}

const statusText = (status: TaskInfo['status']) => {
  const texts: Record<TaskInfo['status'], string> = {
    pending: '等待中',
    running: '执行中',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消',
  }
  return texts[status] || status
}

const statusTagType = (status: TaskInfo['status']) => {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  if (status === 'cancelled') return 'info'
  return 'primary'
}

const progressPercent = (task: TaskInfo) => {
  if (task.status === 'completed') return 100
  if (task.total <= 0) return task.status === 'running' ? 1 : 0
  return Math.min(100, Math.round((task.progress / task.total) * 100))
}

const formatTime = (value?: string) => {
  if (!value || value === '0001-01-01T00:00:00Z') return '-'
  return new Date(value).toLocaleString('zh-CN')
}

const formatUnixTime = (value?: number) => {
  if (!value || value <= 0) return ''
  return new Date(value * 1000).toLocaleString('zh-CN')
}

const formatRange = (task: TaskInfo) => {
  const start = formatUnixTime(task.start_time)
  const end = formatUnixTime(task.end_time)
  if (!start && !end) return '-'
  return `${start || '-'} 至 ${end || '-'}`
}

const formatJSON = (value: Record<string, unknown>) => {
  return JSON.stringify(value, null, 2)
}

const showDetail = (task: TaskInfo) => {
  selectedTask.value = task
  detailVisible.value = true
}

const cancelTask = async (task: TaskInfo) => {
  try {
    await ElMessageBox.confirm('确定要取消该等待任务吗？', '确认取消', {
      type: 'warning',
      confirmButtonText: '取消任务',
      cancelButtonText: '返回',
    })
  } catch { return }

  try {
    const res = await taskAPI.cancel(task.id)
    if (res.code === 0) {
      ElMessage.success('任务已取消')
      await loadTasks()
    } else {
      ElMessage.error(res.msg || '取消失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '取消失败')
  }
}

const retryTask = async (task: TaskInfo) => {
  try {
    const res = await taskAPI.retry(task.id)
    if (res.code === 0) {
      ElMessage.success('任务已重新提交')
      await loadTasks()
    } else {
      ElMessage.error(res.msg || '重试失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '重试失败')
  }
}
</script>

<style scoped>
.task-center {
  padding: 0;
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

.status-alert {
  margin-bottom: 16px;
}

.error-text {
  color: #f56c6c;
}

.json-detail {
  margin: 16px 0 0;
  padding: 12px;
  background: #f8fafc;
  border-radius: 6px;
  color: #2d3748;
  font-size: 12px;
  line-height: 1.5;
  max-height: 360px;
  overflow: auto;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
