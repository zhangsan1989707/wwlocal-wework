<template>
  <div class="feature-config">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">日志类型配置</span>
          <el-button type="primary" @click="handleSave" :loading="saving" size="small">
            保存更改
          </el-button>
        </div>
      </template>

      <el-table :data="features" stripe size="small" v-loading="loading">
        <el-table-column prop="feature_id" label="日志类型编号" width="120" align="center" />
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column label="启用同步" width="100" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" />
          </template>
        </el-table-column>
        <el-table-column label="上次同步" width="170">
          <template #default="{ row }">{{ formatTime(row.last_sync_at) }}</template>
        </el-table-column>
        <el-table-column label="已同步总数" width="120" align="center">
          <template #default="{ row }">
            <span>{{ row.total_synced > 0 ? formatNumber(row.total_synced) : '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="最新日志时间" width="120">
          <template #default="{ row }">
            <span v-if="row.last_log_time > 0" class="time-text">{{ formatUnixTime(row.last_log_time) }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
          <el-table-column label="操作" width="120" align="center">
            <template #default="{ row }">
              <el-button type="primary" size="small" link @click="viewRecentLogs(row.feature_id)">
                查看最近日志
              </el-button>
            </template>
          </el-table-column>
        </el-table>

      <div class="actions">
        <el-button @click="handleEnableAll" size="small">全部启用</el-button>
        <el-button @click="handleDisableAll" size="small" type="danger" plain>停用全部日志类型</el-button>
      </div>
    </el-card>

    <el-card class="admin-oper-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">企微操作日志</span>
          <div class="header-actions">
            <el-button type="success" size="small" @click="handleSyncAdminOperLog" :loading="adminSyncLoading">
              同步
            </el-button>
            <el-button type="primary" size="small" @click="handleViewAdminOperLog">
              查看日志
            </el-button>
          </div>
        </div>
      </template>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="状态">
          <el-tag :type="adminOperLogStats.running ? 'success' : 'info'" size="small">
            {{ adminOperLogStats.running ? '同步中' : '空闲' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="已同步总数">{{ adminOperLogStats.total > 0 ? formatNumber(adminOperLogStats.total) : '-' }}</el-descriptions-item>
        <el-descriptions-item label="最新日志时间">
          <span v-if="adminOperLogStats.last_time">{{ formatTime(adminOperLogStats.last_time) }}</span>
          <span v-else>-</span>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { syncFeatureAPI, adminOperLogAPI } from '../api'
import type { ApiResponse, SyncFeature, AdminOperLogStats } from '../types/api'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const features = ref<SyncFeature[]>([])
const loading = ref(false)
const saving = ref(false)
const adminSyncLoading = ref(false)
const adminOperLogStats = ref<AdminOperLogStats>({
  running: false,
  total: 0,
})
let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  await loadFeatures()
  await loadAdminOperLogStats()
})

onUnmounted(() => {
  stopPolling()
})

const loadFeatures = async () => {
  loading.value = true
  try {
    const res = await syncFeatureAPI.list() as unknown as ApiResponse<SyncFeature[]>
    if (res.code === 0) {
      features.value = res.data || []
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

const loadAdminOperLogStats = async () => {
  try {
    const res = await adminOperLogAPI.syncStatus() as unknown as ApiResponse<AdminOperLogStats>
    if (res.code === 0 && res.data) {
      adminOperLogStats.value = res.data
      if (adminOperLogStats.value.running) {
        startPolling()
      }
    }
  } catch (err) {
    console.error(err)
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(async () => {
    await loadAdminOperLogStats()
    if (!adminOperLogStats.value.running) {
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

const handleSave = async () => {
  saving.value = true
  try {
    const data = features.value.map(f => ({
      feature_id: f.feature_id,
      enabled: f.enabled,
    }))
    const res = await syncFeatureAPI.update({ features: data }) as unknown as ApiResponse<{ message: string }>
    if (res.code === 0) {
      ElMessage.success('保存成功')
    } else {
      ElMessage.error(res.msg || '保存失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

const handleEnableAll = () => {
  features.value.forEach(f => { f.enabled = true })
}

const handleDisableAll = async () => {
  try {
    await ElMessageBox.confirm(
      '停用后所有日志类型将不再参与定时同步，确定继续吗？',
      '确认停用全部',
      { type: 'warning', confirmButtonText: '确定停用', cancelButtonText: '取消' }
    )
  } catch { return }
  features.value.forEach(f => { f.enabled = false })
}

const viewRecentLogs = (featureId: number) => {
  const end = new Date()
  const start = new Date(end.getTime() - 7 * 24 * 60 * 60 * 1000)
  router.push({
    path: '/query',
    query: {
      feature_ids: String(featureId),
      start_time: String(Math.floor(start.getTime() / 1000)),
      end_time: String(Math.floor(end.getTime() / 1000)),
    },
  })
}

const handleSyncAdminOperLog = async () => {
  try {
    await ElMessageBox.confirm(
      '将同步最新的管理员操作日志，确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  adminSyncLoading.value = true
  try {
    const res = await adminOperLogAPI.sync({}) as unknown as ApiResponse<{ running: boolean; message: string }>
    if (res.code === 0) {
      ElMessage.success('同步任务已启动')
      await loadAdminOperLogStats()
      startPolling()
    } else {
      ElMessage.error(res.msg || '同步启动失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '同步启动失败')
  } finally {
    adminSyncLoading.value = false
  }
}

const handleViewAdminOperLog = () => {
  router.push('/adminoper')
}

const formatTime = (timeStr: string) => {
  if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return '-'
  return new Date(timeStr).toLocaleString('zh-CN')
}

const formatUnixTime = (ts: number) => {
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

const formatNumber = (n: number) => {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}
</script>

<style scoped>
.feature-config {
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.actions {
  margin-top: 16px;
  display: flex;
  gap: 8px;
}

.time-text {
  font-size: 12px;
  color: #909399;
}

.admin-oper-card {
  margin-top: 16px;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
