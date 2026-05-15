<template>
  <div class="system-status">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span class="card-title">系统状态</span>
          <el-button type="primary" :loading="loading" @click="loadStatus" size="small">刷新</el-button>
        </div>
      </template>

      <div v-if="status" class="status-sections">
        <!-- 系统健康 -->
        <div class="section">
          <h4 class="section-title">系统健康</h4>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="数据库连接">
              <el-tag :type="status.health?.db_connected ? 'success' : 'danger'" size="small">
                {{ status.health?.db_connected ? '正常' : '异常' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="服务运行时间">
              {{ formatUptime(status.health?.uptime_seconds || 0) }}
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- 密钥状态 -->
        <div class="section">
          <h4 class="section-title">密钥状态</h4>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="当前激活密钥版本">
              {{ status.key_status?.active_version || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="已使用天数">
              {{ status.key_status?.active_days ?? '-' }} 天
            </el-descriptions-item>
            <el-descriptions-item label="密钥总数">
              {{ status.key_status?.total_keys || 0 }}
            </el-descriptions-item>
            <el-descriptions-item label="备用密钥">
              {{ (status.key_status?.total_keys || 0) > 1 ? (status.key_status.total_keys - 1) + ' 个' : '无' }}
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- 通讯录状态 -->
        <div class="section">
          <h4 class="section-title">通讯录</h4>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="总人数">
              {{ formatNumber(status.contacts?.total || 0) }}
            </el-descriptions-item>
            <el-descriptions-item label="上次同步">
              {{ status.contacts?.last_sync ? formatTime(status.contacts.last_sync) : '未同步' }}
              <el-tag v-if="status.contacts?.sync_age_hours > 168" type="warning" size="small" style="margin-left: 8px">
                超过 7 天
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- 同步覆盖 -->
        <div class="section">
          <h4 class="section-title">同步覆盖</h4>
          <el-table :data="coverageData" stripe size="small" max-height="300">
            <el-table-column prop="feature_id" label="日志类型编号" width="130" align="center" />
            <el-table-column prop="total_synced" label="已同步总数" width="130" align="center">
              <template #default="{ row }">
                {{ formatNumber(row.total_synced) }}
              </template>
            </el-table-column>
            <el-table-column label="最新日志时间" width="180">
              <template #default="{ row }">
                {{ row.last_log_time > 0 ? formatUnixTime(row.last_log_time) : '-' }}
              </template>
            </el-table-column>
            <el-table-column label="数据延迟" width="120" align="center">
              <template #default="{ row }">
                <el-tag v-if="row.data_age_hours !== undefined" :type="row.data_age_hours > 48 ? 'warning' : 'success'" size="small">
                  {{ row.data_age_hours }} 小时
                </el-tag>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column label="上次同步" min-width="170">
              <template #default="{ row }">
                {{ row.last_sync_at ? formatTime(row.last_sync_at) : '-' }}
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 数据库表大小 -->
        <div class="section" v-if="status.table_sizes?.length > 0">
          <h4 class="section-title">数据表存储</h4>
          <el-table :data="status.table_sizes" stripe size="small" max-height="400">
            <el-table-column prop="table" label="表名" min-width="200" />
            <el-table-column prop="rows" label="记录数" width="120" align="center">
              <template #default="{ row }">
                {{ formatNumber(row.rows) }}
              </template>
            </el-table-column>
            <el-table-column label="数据大小" width="120" align="center">
              <template #default="{ row }">
                {{ formatBytes(row.data_bytes) }}
              </template>
            </el-table-column>
            <el-table-column label="索引大小" width="120" align="center">
              <template #default="{ row }">
                {{ formatBytes(row.index_bytes) }}
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>

      <el-skeleton v-if="loading && !status" :rows="8" animated />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { systemAPI } from '../api'

const status = ref<any>(null)
const loading = ref(false)

const coverageData = computed(() => {
  if (!status.value?.sync_coverage) return []
  return Object.entries(status.value.sync_coverage).map(([featureId, info]: [string, any]) => ({
    feature_id: featureId,
    ...info,
  }))
})

const loadStatus = async () => {
  loading.value = true
  try {
    const res: any = await systemAPI.getStatus()
    if (res.code === 0) {
      status.value = res.data
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

const formatUptime = (seconds: number) => {
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时 ${Math.floor((seconds % 3600) / 60)} 分钟`
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  return `${days} 天 ${hours} 小时`
}

const formatTime = (t: string) => {
  if (!t || t === '0001-01-01T00:00:00Z') return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const formatUnixTime = (ts: number) => {
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

const formatNumber = (n: number) => {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}

const formatBytes = (b: string | number) => {
  const bytes = Number(b)
  if (bytes === 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
  return `${(bytes / 1073741824).toFixed(1)} GB`
}

onMounted(() => {
  loadStatus()
})
</script>

<style scoped>
.system-status {
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

.status-sections {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #1a365d;
  margin: 0 0 12px 0;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
