<template>
  <div class="operation-log">
    <el-card class="filter-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">操作审计</span>
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
      <el-form :model="filter" inline>
        <el-form-item label="操作类型">
          <el-select v-model="filter.action" clearable placeholder="全部" style="width: 150px">
            <el-option v-for="a in actions" :key="a" :label="actionLabels[a] || a" :value="a" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filter.status" clearable placeholder="全部" style="width: 120px">
            <el-option label="成功" value="success" />
            <el-option label="失败" value="fail" />
            <el-option label="未授权" value="unauthorized" />
            <el-option label="参数错误" value="bad_request" />
          </el-select>
        </el-form-item>
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
        stripe
        v-loading="loading"
        max-height="500"
        highlight-current-row
      >
        <el-table-column prop="created_at" label="时间" width="180" align="center">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户" width="100" align="center" />
        <el-table-column prop="action" label="操作" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="getActionTag(row.action)">{{ actionLabels[row.action] || row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="method" label="方法" width="80" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="getMethodTag(row.method)">{{ row.method }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路径" min-width="200" />
        <el-table-column prop="status_code" label="状态码" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="getStatusTag(row.status_code)">{{ row.status_code }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="duration_ms" label="耗时(ms)" width="100" align="center" />
        <el-table-column prop="error_msg" label="错误信息" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.error_msg" class="error-text">{{ row.error_msg }}</span>
            <span v-else class="success-text">-</span>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-if="total > 0"
        class="pagination"
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
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
import { operationLogAPI } from '../api'
import { ElMessage } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'

const actionLabels: Record<string, string> = {
  login: '登录',
  sync: '数据同步',
  query: '日志查询',
  key_management: '密钥管理',
  scheduler: '定时任务',
  contacts: '通讯录',
  operation_log: '操作审计',
}

const filter = reactive({
  action: '',
  status: '',
})

const loading = ref(false)
const tableData = ref<any[]>([])
const total = ref(0)
const actions = ref<string[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
})

onMounted(async () => {
  try {
    const res: any = await operationLogAPI.getActions()
    if (res.code === 0) {
      actions.value = res.data
    }
  } catch {
    // ignore
  }
  handleQuery()
})

const handleQuery = async () => {
  loading.value = true
  try {
    const params: any = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (filter.action) params.action = filter.action
    if (filter.status === 'success') {
      params.status_code = 200
    } else if (filter.status === 'fail') {
      params.status_code = 500
    } else if (filter.status === 'unauthorized') {
      params.status_code = 401
    } else if (filter.status === 'bad_request') {
      params.status_code = 400
    }

    const res: any = await operationLogAPI.list(params)
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
  filter.action = ''
  filter.status = ''
  pagination.page = 1
  handleQuery()
}

const formatTime = (t: string) => {
  if (!t) return '-'
  return t.replace('T', ' ').slice(0, 19)
}

const getActionTag = (action: string) => {
  const map: Record<string, string> = {
    login: 'success',
    sync: 'warning',
    query: '',
    key_management: 'danger',
    scheduler: 'warning',
    contacts: '',
  }
  return map[action] || 'info'
}

const getMethodTag = (method: string) => {
  const map: Record<string, string> = {
    GET: 'success',
    POST: 'warning',
    PUT: '',
    DELETE: 'danger',
  }
  return map[method] || 'info'
}

const getStatusTag = (code: number) => {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 400 && code < 500) return 'warning'
  if (code >= 500) return 'danger'
  return 'info'
}
</script>

<style scoped>
.operation-log {
  padding: 0;
}

.filter-card,
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

.result-count {
  font-size: 14px;
  color: #909399;
}

.error-text {
  color: #f56c6c;
  font-size: 12px;
}

.success-text {
  color: #c0c4cc;
}

.pagination {
  margin-top: 16px;
  justify-content: center;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
