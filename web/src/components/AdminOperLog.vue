<template>
  <div class="admin-oper-log">
    <el-card class="filter-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">企微操作日志</span>
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
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="filter.time_range"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            style="width: 400px"
          />
        </el-form-item>
        <el-form-item label="操作类型">
          <el-select v-model="filter.operation" clearable placeholder="全部" style="width: 200px">
            <el-option label="全部" value="" />
            <el-option
              v-for="type in operTypes"
              :key="type"
              :label="getOperationLabel(type)"
              :value="type"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="操作者">
          <el-select v-model="filter.operator" clearable placeholder="全部" style="width: 200px" filterable>
            <el-option label="全部" value="" />
            <el-option
              v-for="user in operUsers"
              :key="user"
              :label="user"
              :value="user"
            />
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
        max-height="600"
        highlight-current-row
      >
        <el-table-column prop="oper_time" label="时间" width="180" align="center">
          <template #default="{ row }">
            {{ formatTime(row.oper_time) }}
          </template>
        </el-table-column>
        <el-table-column prop="oper_userid" label="操作者" width="120" align="center">
          <template #default="{ row }">
            {{ row.oper_name || row.oper_userid }}
          </template>
        </el-table-column>
        <el-table-column prop="oper_type" label="操作类型" width="150" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="getOperationTag(row.oper_type)">{{ getOperationLabel(row.oper_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="oper_type_id" label="操作对象" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.oper_type_id }}
          </template>
        </el-table-column>
        <el-table-column prop="oper_data" label="详情" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.oper_data }}
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
import { adminOperLogAPI } from '../api'
import { ElMessage } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'

const filter = reactive({
  time_range: null as [Date, Date] | null,
  operation: '',
  operator: '',
})

const loading = ref(false)
const tableData = ref<any[]>([])
const total = ref(0)
const operTypes = ref<string[]>([])
const operUsers = ref<string[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
})

const setTodayRange = () => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const end = new Date()
  end.setHours(23, 59, 59, 999)
  filter.time_range = [today, end]
}

onMounted(async () => {
  setTodayRange()
  await Promise.all([
    loadOperTypes(),
    loadOperUsers(),
  ])
  await handleQuery()
})

const loadOperTypes = async () => {
  try {
    const res: any = await adminOperLogAPI.getTypes()
    if (res.code === 0) {
      operTypes.value = res.data || []
    }
  } catch (err) {
    console.error('Failed to load oper types:', err)
  }
}

const loadOperUsers = async () => {
  try {
    const res: any = await adminOperLogAPI.getUsers()
    if (res.code === 0) {
      operUsers.value = res.data || []
    }
  } catch (err) {
    console.error('Failed to load oper users:', err)
  }
}

const handleQuery = async () => {
  loading.value = true
  try {
    const params: any = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (filter.time_range) {
      params.start_time = Math.floor(filter.time_range[0].getTime() / 1000)
      params.end_time = Math.floor(filter.time_range[1].getTime() / 1000)
    }
    if (filter.operation) params.oper_type = filter.operation
    if (filter.operator) params.oper_userid = filter.operator

    const res: any = await adminOperLogAPI.query(params)
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
  setTodayRange()
  filter.operation = ''
  filter.operator = ''
  pagination.page = 1
  handleQuery()
}

const formatTime = (ts: number | string) => {
  if (!ts) return '-'
  const timestamp = typeof ts === 'string' ? parseInt(ts, 10) : ts
  return new Date(timestamp * 1000).toLocaleString('zh-CN')
}

const getOperationLabel = (op: string) => {
  const map: Record<string, string> = {
    'member_dept': '成员与部门变更',
    'permission': '权限管理变更',
    'app': '应用与小程序',
    'admin_tool': '管理工具变更',
    'audit': '审计',
    'external_service': '对外服务',
    'contact_chat': '通讯录与聊天管理',
    'login': '登录管理',
    'external_contact': '外部联系人管理',
    'security': '安全与保密',
    'help': '帮助与反馈',
    'data_protection': '数据防泄漏',
    'other': '其他',
  }
  return map[op] || op
}

const getOperationTag = (op: string) => {
  const map: Record<string, string> = {
    'member_dept': 'primary',
    'permission': 'success',
    'app': 'warning',
    'admin_tool': 'danger',
    'audit': 'info',
    'external_service': '',
    'contact_chat': 'primary',
    'login': 'success',
    'external_contact': 'warning',
    'security': 'danger',
    'help': 'info',
    'data_protection': '',
    'other': 'info',
  }
  return map[op] || 'info'
}
</script>

<style scoped>
.admin-oper-log {
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

.pagination {
  margin-top: 16px;
  justify-content: center;
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

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
