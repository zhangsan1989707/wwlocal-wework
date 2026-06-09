<template>
  <div class="dashboard-v2">
    <div class="dashboard-header">
      <div class="header-left">
        <div>
          <div class="page-title">运营总览</div>
          <div class="page-subtitle">按授权范围查看组织使用情况</div>
        </div>
        <span class="header-label">数据日期</span>
        <el-date-picker
          v-model="selectedDate"
          type="date"
          value-format="YYYY-MM-DD"
          placeholder="选择日期"
          :clearable="false"
          style="width: 160px;"
          @change="refreshData"
        />
        <el-tag v-if="overview?.scope" size="small" type="info">
          {{ overview.scope.role === 'super_admin' ? '全量数据' : `授权部门 ${overview.scope.dept_ids.length} 个` }}
        </el-tag>
      </div>
      <el-button type="primary" :loading="loading" @click="refreshData">刷新</el-button>
    </div>

    <el-alert
      class="data-source-alert"
      :title="`最新统计日期：${selectedDate}`"
      description="数据来源：夜间 1 点分析预计算的运营总览数据。"
      type="info"
      show-icon
      :closable="false"
    />

    <el-alert v-if="!loading && noData" title="当前范围暂无运营数据" description="可能是所选日期尚未完成统计，或当前授权部门没有可统计数据。" type="info" show-icon :closable="false" style="margin-bottom: 16px;" />

    <div class="kpi-row kpi-row-primary">
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.active) }}</div>
        <div class="kpi-label">活跃用户</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value" :class="rateActiveClass">{{ formatPermille(overview?.rate_active) }}</div>
        <div class="kpi-label">活跃率</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value" :class="{ 'kpi-bad': (overview?.inactive || 0) > 0 }">{{ formatNum(overview?.inactive) }}</div>
        <div class="kpi-label">未活跃用户</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value kpi-good">{{ formatPermille(overview?.rate_activation) }}</div>
        <div class="kpi-label">激活率</div>
      </el-card>
    </div>

    <div class="kpi-row kpi-row-secondary">
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.registered) }}</div>
        <div class="kpi-label">注册用户</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.activated) }}</div>
        <div class="kpi-label">激活用户</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.login_users) }}</div>
        <div class="kpi-label">登录人数</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.msg_count) }}</div>
        <div class="kpi-label">消息发送量</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.msg_sender) }}</div>
        <div class="kpi-label">消息发送人数</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.app_access_user) }}</div>
        <div class="kpi-label">访问应用人数</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.group_created) }}</div>
        <div class="kpi-label">创建群聊数</div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-value">{{ formatNum(overview?.devices?.total) }}</div>
        <div class="kpi-label">使用设备数</div>
      </el-card>
    </div>

    <!-- Section 3: Trend Chart -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">趋势分析</span>
          <div class="chart-controls">
            <el-select v-model="trendMetric" style="width: 150px;" @change="fetchTrend">
              <el-option label="活跃用户" value="active" />
              <el-option label="使用人数" value="usage_users" />
              <el-option label="消息发送量" value="msg_count" />
              <el-option label="消息发送人数" value="msg_sender" />
              <el-option label="登录人数" value="login_users" />
              <el-option label="创建群聊" value="group_created" />
            </el-select>
            <el-radio-group v-model="trendGranularity" @change="fetchTrend">
              <el-radio-button value="day">日</el-radio-button>
              <el-radio-button value="week">周</el-radio-button>
              <el-radio-button value="month">月</el-radio-button>
              <el-radio-button value="quarter">季</el-radio-button>
            </el-radio-group>
            <el-button size="small" @click="exportTrendCSV">导出</el-button>
          </div>
        </div>
      </template>
      <div ref="trendChartRef" class="chart-container" v-loading="trendLoading"></div>
    </el-card>

    <!-- Section 4: Device Distribution -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">设备分布</span>
          <el-button size="small" @click="exportDevicesCSV">导出 CSV</el-button>
        </div>
      </template>
      <div class="device-section">
        <div ref="deviceChartRef" class="device-chart" v-loading="loading"></div>
        <div class="device-list" v-if="overview?.devices?.types?.length">
          <div v-for="d in overview.devices.types" :key="d.type" class="device-item">
            <span class="device-name">{{ d.name || d.type }}</span>
            <span class="device-count">{{ formatNum(d.count) }}</span>
            <span class="device-pct">{{ d.percentage.toFixed(1) }}%</span>
          </div>
        </div>
      </div>
    </el-card>

    <!-- Section 5: Department Stats Table -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">部门统计</span>
          <el-button size="small" @click="exportDepartmentsCSV">导出 CSV</el-button>
        </div>
      </template>
      <el-table :data="departments" stripe v-loading="deptLoading" style="width: 100%">
        <el-table-column prop="dept_name" label="部门名称" min-width="150" />
        <el-table-column prop="total_contacts" label="总人数" width="100" align="center" sortable />
        <el-table-column prop="active" label="活跃人数" width="100" align="center" sortable>
          <template #default="{ row }">
            <el-tag type="success" size="small">{{ row.active }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="inactive" label="未活跃人数" width="110" align="center" sortable>
          <template #default="{ row }">
            <el-tag :type="row.inactive > 0 ? 'danger' : 'info'" size="small">{{ row.inactive }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="活跃率" width="100" align="center" sortable sort-by="active_rate">
          <template #default="{ row }">
            <span :class="row.active_rate >= 80 ? 'text-good' : row.active_rate >= 50 ? '' : 'text-bad'">
              {{ row.active_rate.toFixed(1) }}%
            </span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Section 6: User Detail Table -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">用户明细</span>
          <el-button type="success" size="small" @click="exportUsersCSV" :disabled="!users.length">导出 CSV</el-button>
        </div>
      </template>
      <el-tabs v-model="userListType" @tab-change="onUserTabChange">
        <el-tab-pane label="未活跃用户" name="inactive" />
        <el-tab-pane label="活跃用户" name="active" />
        <el-tab-pane label="未登录用户" name="no_login" />
      </el-tabs>
      <el-table :data="users" stripe v-loading="userLoading" style="width: 100%" @row-click="handleUserClick" highlight-current-row>
        <el-table-column prop="name" label="姓名" width="120">
          <template #default="{ row }">
            <span class="link-text">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="手机号" width="140">
          <template #default="{ row }">{{ maskMobile(row.mobile) }}</template>
        </el-table-column>
        <el-table-column prop="user_id" label="用户ID" width="160" />
        <el-table-column prop="department" label="部门" min-width="150" />
      </el-table>
      <el-pagination
        v-if="userTotal > userPageSize"
        style="margin-top: 16px; justify-content: center;"
        layout="total, prev, pager, next"
        :total="userTotal"
        :page-size="userPageSize"
        v-model:current-page="userPage"
        @current-change="fetchUsers"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { dashboardV2Api } from '../api'
import type { DashboardV2Overview, DashboardV2DeptStat, DashboardV2UserItem } from '../types/api'

const router = useRouter()

let isMounted = { value: true }
onUnmounted(() => { isMounted.value = false })

// --- State ---
const loading = ref(false)
const selectedDate = ref(getYesterday())

const overview = ref<DashboardV2Overview | null>(null)
const departments = ref<DashboardV2DeptStat[]>([])
const deptLoading = ref(false)

// Trend
const trendLoading = ref(false)
const trendMetric = ref('active')
const trendGranularity = ref('day')
const trendChartRef = ref<HTMLElement>()
let trendChart: echarts.ECharts | undefined

// Device
const deviceChartRef = ref<HTMLElement>()
let deviceChart: echarts.ECharts | undefined

// Users
const userListType = ref('inactive')
const users = ref<DashboardV2UserItem[]>([])
const userTotal = ref(0)
const userPage = ref(1)
const userPageSize = 20
const userLoading = ref(false)

// --- Helpers ---
function getYesterday(): string {
  const d = new Date()
  d.setDate(d.getDate() - 1)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function formatNum(n?: number): string {
  if (n === undefined || n === null) return '-'
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}

function formatPermille(n?: number): string {
  if (n === undefined || n === null) return '-'
  return (n / 10).toFixed(1) + '%'
}

function maskMobile(mobile?: string): string {
  if (!mobile) return '-'
  return mobile.replace(/^(\d{3})\d{4}(\d+)/, '$1****$2')
}

const rateActiveClass = computed(() => {
  const v = overview.value?.rate_active
  if (v === undefined) return ''
  return v >= 500 ? 'kpi-good' : v >= 200 ? '' : 'kpi-bad'
})

const noData = computed(() => {
  const o = overview.value
  if (!o) return true
  return o.registered === 0 && o.active === 0 && o.msg_count === 0
})

// --- Data fetching ---
async function refreshData() {
  await Promise.all([fetchOverview(), fetchDepartments(), fetchUsers()])
}

async function fetchOverview() {
  loading.value = true
  try {
    const res = await dashboardV2Api.getOverview(selectedDate.value)
    if (!isMounted.value) return
    if (res.code === 0 && res.data) {
      overview.value = res.data
      renderDeviceChart()
    }
  } catch (err: unknown) {
    if (isMounted.value) ElMessage.error(err instanceof Error ? err.message : '加载概览数据失败')
  } finally {
    if (isMounted.value) loading.value = false
  }
}

async function fetchTrend() {
  trendLoading.value = true
  try {
    const res = await dashboardV2Api.getTrend({
      metric_type: trendMetric.value,
      end_date: selectedDate.value,
      granularity: trendGranularity.value,
    })
    if (!isMounted.value) return
    if (res.code === 0 && res.data) {
      renderTrendChart(res.data.periods, res.data.series)
    }
  } catch (err: unknown) {
    if (isMounted.value) ElMessage.error(err instanceof Error ? err.message : '加载趋势数据失败')
  } finally {
    if (isMounted.value) trendLoading.value = false
  }
}

async function fetchDepartments() {
  deptLoading.value = true
  try {
    const res = await dashboardV2Api.getDepartments(selectedDate.value)
    if (!isMounted.value) return
    if (res.code === 0 && res.data) {
      departments.value = res.data
    }
  } catch (err: unknown) {
    if (isMounted.value) ElMessage.error(err instanceof Error ? err.message : '加载部门数据失败')
  } finally {
    if (isMounted.value) deptLoading.value = false
  }
}

async function fetchUsers() {
  userLoading.value = true
  try {
    const res = await dashboardV2Api.getUsers({
      date: selectedDate.value,
      list_type: userListType.value,
      page: userPage.value,
      page_size: userPageSize,
    })
    if (!isMounted.value) return
    if (res.code === 0 && res.data) {
      users.value = res.data.users || []
      userTotal.value = res.data.total || 0
    }
  } catch (err: unknown) {
    if (isMounted.value) ElMessage.error(err instanceof Error ? err.message : '加载用户列表失败')
  } finally {
    if (isMounted.value) userLoading.value = false
  }
}

function onUserTabChange() {
  userPage.value = 1
  fetchUsers()
}

// --- Charts ---
function renderTrendChart(periods: string[], series: Record<string, number[]>) {
  if (!trendChartRef.value) return
  if (!trendChart) {
    trendChart = echarts.init(trendChartRef.value)
  }
  const seriesList = Object.entries(series).map(([name, data]) => ({
    name,
    type: 'line' as const,
    data,
    smooth: true,
    areaStyle: { opacity: 0.15 },
  }))
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0 },
    grid: { left: '3%', right: '4%', bottom: '12%', top: '8%', containLabel: true },
    xAxis: { type: 'category', data: periods, boundaryGap: false },
    yAxis: { type: 'value' },
    series: seriesList,
  }, true)
}

function renderDeviceChart() {
  const types = overview.value?.devices?.types
  if (!deviceChartRef.value || !types?.length) return
  if (!deviceChart) {
    deviceChart = echarts.init(deviceChartRef.value)
  }
  deviceChart.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { orient: 'vertical', left: 'left', top: 'center' },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['60%', '50%'],
      label: { show: true, formatter: '{b}\n{d}%' },
      data: types.map(d => ({ name: d.name || d.type, value: d.count })),
    }],
  }, true)
}

function handleResize() {
  trendChart?.resize()
  deviceChart?.resize()
}

// --- Actions ---
function handleUserClick(row: DashboardV2UserItem) {
  if (row.mobile) {
    router.push({ path: '/query', query: { mobile: row.mobile } })
  }
}

function exportUsersCSV() {
  dashboardV2Api.exportUsers({
    date: selectedDate.value,
    list_type: userListType.value,
  }).then((res) => {
    downloadBlob(res, `users_${userListType.value}_${selectedDate.value}.csv`)
  }).catch(() => {
    ElMessage.error('导出失败')
  })
}

function downloadBlob(res: Blob, filename: string) {
  const blob = new Blob([res], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

function exportTrendCSV() {
  dashboardV2Api.exportTrend({
    metric_type: trendMetric.value,
    end_date: selectedDate.value,
    granularity: trendGranularity.value,
  }).then((res) => {
    downloadBlob(res, `trend_${trendMetric.value}_${trendGranularity.value}.csv`)
  }).catch(() => ElMessage.error('导出失败'))
}

function exportDepartmentsCSV() {
  dashboardV2Api.exportDepartments(selectedDate.value).then((res) => {
    downloadBlob(res, `departments_${selectedDate.value}.csv`)
  }).catch(() => ElMessage.error('导出失败'))
}

function exportDevicesCSV() {
  dashboardV2Api.exportDevices(selectedDate.value).then((res) => {
    downloadBlob(res, `devices_${selectedDate.value}.csv`)
  }).catch(() => ElMessage.error('导出失败'))
}

// --- Lifecycle ---
onMounted(() => {
  refreshData()
  fetchTrend()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  trendChart?.dispose()
  deviceChart?.dispose()
})
</script>

<style scoped>
.dashboard-v2 {
  padding: 0;
}

.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  line-height: 1.2;
}

.page-subtitle {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.header-label {
  font-size: 14px;
  color: #606266;
  white-space: nowrap;
}

.data-source-alert {
  margin-bottom: 16px;
}

/* KPI cards */
.kpi-row {
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
}

.kpi-row-primary {
  grid-template-columns: repeat(4, 1fr);
}

.kpi-row-secondary {
  grid-template-columns: repeat(4, 1fr);
}

.kpi-card {
  text-align: center;
  padding: 4px 0;
}

.kpi-card :deep(.el-card__body) {
  padding: 16px 8px;
}

.kpi-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
  line-height: 1.2;
}

.kpi-row-secondary .kpi-value {
  font-size: 24px;
}

.kpi-value.kpi-good {
  color: #67c23a;
}

.kpi-value.kpi-bad {
  color: #f56c6c;
}

.kpi-label {
  font-size: 13px;
  color: #909399;
  margin-top: 6px;
}

/* Section cards */
.section-card {
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

.chart-controls {
  display: flex;
  gap: 12px;
  align-items: center;
}

.chart-container {
  width: 100%;
  height: 320px;
}

/* Device section */
.device-section {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

.device-chart {
  width: 360px;
  height: 280px;
  flex-shrink: 0;
}

.device-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.device-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
}

.device-name {
  flex: 1;
  font-size: 14px;
  color: #303133;
}

.device-count {
  font-size: 14px;
  font-weight: 500;
  color: #606266;
  min-width: 60px;
  text-align: right;
}

.device-pct {
  font-size: 13px;
  color: #909399;
  min-width: 50px;
  text-align: right;
}

/* Text helpers */
.text-good {
  color: #67c23a;
  font-weight: 500;
}

.text-bad {
  color: #f56c6c;
  font-weight: 500;
}

.link-text {
  color: #409eff;
  cursor: pointer;
}

.link-text:hover {
  text-decoration: underline;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}

:deep(.el-tabs__header) {
  margin-bottom: 12px;
}
</style>
