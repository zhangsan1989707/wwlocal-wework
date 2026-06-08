<template>
  <div class="admin-oper-log">
    <el-card class="main-card">
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

      <el-form :model="form" label-position="left" label-width="120px" :inline="true" style="margin-bottom: 20px">
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="dateRange"
            type="datetimerange"
            format="YYYY-MM-DD HH:mm:ss"
            value-format="YYYY-MM-DD HH:mm:ss"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            style="width: 400px"
          />
        </el-form-item>
        <el-form-item label="操作类型">
          <el-select v-model="form.oper_type" placeholder="请选择" clearable style="width: 200px">
            <el-option label="添加" value="add" />
            <el-option label="删除" value="delete" />
            <el-option label="修改" value="update" />
          </el-select>
        </el-form-item>
        <el-form-item label="操作者">
          <el-input v-model="form.oper_userid" placeholder="操作者UserID" clearable style="width: 200px" />
        </el-form-item>
      </el-form>

      <el-table :data="tableData" border stripe style="width: 100%" v-loading="loading">
        <el-table-column prop="time" label="操作时间" width="180">
          <template #default="{ row }">{{ formatTime(row.time) }}</template>
        </el-table-column>
        <el-table-column prop="oper_userid" label="操作者UserID" width="150" />
        <el-table-column prop="oper_name" label="操作者" width="120" />
        <el-table-column prop="oper_type" label="操作类型" width="150" show-overflow-tooltip />
        <el-table-column prop="oper_desc" label="操作描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="app_id" label="应用ID" width="150" />
        <el-table-column prop="oper_data" label="详情" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            <pre style="margin: 0; white-space: pre-wrap; word-break: break-all">{{ row.oper_data }}</pre>
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
import { ref, reactive, onMounted, watchEffect } from 'vue'
import { adminOperLogAPI } from '../api'
import type { AdminOperLog, AdminOperLogParams } from '../types/api'
import { ElMessage } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'

const form = reactive({
  start_time: null as Date | null,
  end_time: null as Date | null,
  oper_type: '',
  oper_userid: '',
})

const dateRange = ref<[Date, Date] | null>(null)
const loading = ref(false)
const tableData = ref<AdminOperLog[]>([])
const total = ref(0)
const pagination = reactive({
  page: 1,
  page_size: 20,
})

const setTodayRange = () => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const end = new Date()
  end.setHours(23, 59, 59, 999)
  dateRange.value = [today, end]
}

let isInitial = true

onMounted(async () => {
  setTodayRange()
  await handleQuery()
  isInitial = false
})

watchEffect(() => {
  if (dateRange.value && !isInitial) {
    pagination.page = 1
    handleQuery()
  }
})

const handleQuery = async () => {
  loading.value = true
  try {
    const params: AdminOperLogParams = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (dateRange.value) {
      params.start_time = Math.floor(dateRange.value[0].getTime() / 1000)
      params.end_time = Math.floor(dateRange.value[1].getTime() / 1000)
    }
    if (form.oper_type) params.oper_type = form.oper_type
    if (form.oper_userid) params.oper_userid = form.oper_userid

    const res = await adminOperLogAPI.query(params)
    if (res.code === 0 && res.data) {
      tableData.value = res.data.data
      total.value = res.data.total
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
  setTodayRange()
  form.oper_type = ''
  form.oper_userid = ''
}

const formatTime = (ts: number | string) => {
  if (!ts) return '-'
  const timestamp = typeof ts === 'string' ? parseInt(ts, 10) : ts
  return new Date(timestamp * 1000).toLocaleString('zh-CN')
}
</script>

<style scoped>
.admin-oper-log {
  padding: 0;
}

.main-card :deep(.el-card__body) {
  padding: 20px;
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

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}
</style>
