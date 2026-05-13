<template>
  <div class="dashboard">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>未使用人员看板</span>
          <el-button type="primary" :loading="loading" @click="fetchData">刷新</el-button>
        </div>
      </template>

      <div class="filter-row">
        <div class="filter-item">
          <span class="filter-label">时间范围</span>
          <el-radio-group v-model="rangeVal" size="default" @change="onFilterChange">
            <el-radio-button value="week">周</el-radio-button>
            <el-radio-button value="month">月</el-radio-button>
            <el-radio-button value="quarter">季</el-radio-button>
          </el-radio-group>
        </div>
        <div class="filter-item">
          <span class="filter-label">部门</span>
          <el-tree-select
            v-model="deptVal"
            :data="deptTree"
            :props="{ label: 'name', value: 'id', children: 'children' }"
            node-key="id"
            check-strictly
            :default-expanded-keys="expandedKeys"
            :render-after-expand="false"
            filterable
            clearable
            placeholder="全部部门"
            style="width: 300px"
            @change="onFilterChange"
          />
        </div>
        <div class="filter-item">
          <span class="filter-label">未使用≥</span>
          <el-input-number
            v-model="minDays"
            :min="1"
            :max="maxDays"
            style="width: 120px"
            @change="onFilterChange"
          />
          <span class="filter-hint">天 (共{{ totalDays }}天)</span>
        </div>
      </div>

      <div class="stats-row" v-if="!loading && data">
        <div class="stat-card">
          <div class="stat-value">{{ data.total_contacts }}</div>
          <div class="stat-label">总人数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ data.total_contacts - data.inactive_count }}</div>
          <div class="stat-label">达标人数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ data.inactive_count }}</div>
          <div class="stat-label">未达标人数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ inactiveRate }}</div>
          <div class="stat-label">未达标率</div>
        </div>
      </div>

      <div v-if="!loading && data" style="margin-top: 12px; color: #909399; font-size: 13px;">
        统计范围：{{ data.months?.join('、') }}，共{{ totalDays }}天，
        统计指标：{{ featureLabel }}，
        筛选条件：未使用天数 ≥ {{ minInactiveDays }}天
      </div>
    </el-card>

    <el-card shadow="never" style="margin-top: 16px;">
      <template #header>
        <div class="card-header">
          <span>部门统计</span>
        </div>
      </template>

      <el-table :data="data?.dept_stats || []" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="部门" />
        <el-table-column prop="total" label="总人数" width="100" align="center" />
        <el-table-column prop="active" label="达标" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="success" size="small">{{ row.active }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="inactive" label="未达标" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.inactive > 0 ? 'danger' : 'info'" size="small">{{ row.inactive }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="达标率" width="100" align="center">
          <template #default="{ row }">
            {{ row.total > 0 ? ((row.active / row.total) * 100).toFixed(0) + '%' : '-' }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" style="margin-top: 16px;">
      <template #header>
        <div class="card-header">
          <span>未达标人员列表 ({{ data?.inactive_count || 0 }}人)</span>
          <el-button type="success" size="small" @click="exportCSV" :disabled="!data?.inactive_users?.length">导出 CSV</el-button>
        </div>
      </template>

      <el-table :data="pagedData" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="姓名" width="100" />
        <el-table-column prop="mobile" label="手机号" width="140" />
        <el-table-column prop="position" label="职位" />
        <el-table-column prop="department" label="所属部门" />
        <el-table-column prop="active_days" label="活跃天数" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.active_days > 0 ? 'success' : 'danger'" size="small">{{ row.active_days }}天</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="inactive_days" label="未使用天数" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="row.inactive_days >= totalDays ? 'danger' : 'warning'" size="small">{{ row.inactive_days }}天</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="user_id" label="UserID" width="160" />
      </el-table>

      <el-pagination
        v-if="data?.inactive_users?.length > pageSize"
        style="margin-top: 16px; justify-content: center;"
        layout="prev, pager, next"
        :total="data.inactive_users.length"
        :page-size="pageSize"
        v-model:current-page="currentPage"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { dashboardAPI, contactAPI } from '../api'

const loading = ref(false)
const data = ref<any>(null)
const expandedKeys = ref<number[]>([])
const currentPage = ref(1)
const pageSize = 20
const rangeVal = ref('quarter')
const deptVal = ref<number | undefined>(undefined)
const minDays = ref(90)
const deptTree = ref<any[]>([])
const totalDays = ref(90)

const maxDays = computed(() => totalDays.value)

const inactiveRate = computed(() => {
  if (!data.value || data.value.total_contacts === 0) return '0%'
  return ((data.value.inactive_count / data.value.total_contacts) * 100).toFixed(1) + '%'
})

const featureLabel = computed(() => {
  if (!data.value?.feature_names) return ''
  return Object.values(data.value.feature_names).join('、')
})

const minInactiveDays = computed(() => data.value?.min_inactive_days || minDays.value)

const pagedData = computed(() => {
  if (!data.value?.inactive_users) return []
  const start = (currentPage.value - 1) * pageSize
  return data.value.inactive_users.slice(start, start + pageSize)
})

const loadDeptTree = async () => {
  const res: any = await contactAPI.getDeptTree()
  if (res.code === 0) {
    deptTree.value = res.data?.tree || []
    // 默认展开首层节点
    expandedKeys.value = deptTree.value.map((n: any) => n.id)
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    const params: any = { range: rangeVal.value, min_inactive_days: minDays.value }
    if (deptVal.value) params.dept_id = deptVal.value
    const res: any = await dashboardAPI.getInactiveUsers(params)
    if (res.code === 0) {
      data.value = res.data
      totalDays.value = res.data.total_days || totalDays.value
    }
  } finally {
    loading.value = false
  }
}

const onFilterChange = () => {
  currentPage.value = 1
  fetchData()
}

const exportCSV = () => {
  if (!data.value?.inactive_users?.length) return
  const header = '姓名,手机号,职位,所属部门,活跃天数,未使用天数,UserID\n'
  const rows = data.value.inactive_users.map((u: any) =>
    `${u.name},${u.mobile},${u.position},${u.department},${u.active_days},${u.inactive_days},${u.user_id}`
  ).join('\n')
  const blob = new Blob(['﻿' + header + rows], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = '未使用人员.csv'
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  loadDeptTree()
  fetchData()
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 500;
}

.filter-row {
  display: flex;
  gap: 24px;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.filter-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 14px;
  color: #606266;
  white-space: nowrap;
}

.filter-hint {
  font-size: 12px;
  color: #909399;
}

.stats-row {
  display: flex;
  gap: 24px;
}

.stat-card {
  flex: 1;
  text-align: center;
  padding: 16px 0;
  background: #f5f7fa;
  border-radius: 8px;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
}
</style>
