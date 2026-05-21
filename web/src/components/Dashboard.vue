<template>
  <div class="dashboard">
    <!-- 总览看板 -->
    <el-card shadow="never" class="overview-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">总览看板</span>
          <el-button type="primary" :loading="overviewLoading" @click="loadOverview" size="small">刷新</el-button>
        </div>
      </template>

      <div class="kpi-grid" v-if="overview">
        <div class="kpi-card">
          <div class="kpi-value">{{ formatRelativeTime(overview.kpis.latest_sync_time || '') }}</div>
          <div class="kpi-label">最新同步时间</div>
          <div class="kpi-sub">{{ formatTime(overview.kpis.latest_sync_time || '') }}</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-value">{{ formatNumber(overview.kpis.synced_7d_count) }}</div>
          <div class="kpi-label">近 7 日同步记录</div>
          <div class="kpi-sub">条</div>
        </div>
        <div class="kpi-card" :class="{ 'kpi-danger': overview.kpis.failed_feature_count > 0 }">
          <div class="kpi-value">{{ overview.kpis.failed_feature_count }}</div>
          <div class="kpi-label">同步失败</div>
          <div class="kpi-sub">个日志类型</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-value">{{ overview.kpis.active_key_version || '-' }}</div>
          <div class="kpi-label">解密密钥</div>
          <div class="kpi-sub">已使用 {{ overview.kpis.active_key_days }} 天 · 共 {{ overview.kpis.key_count }} 个</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-value">{{ formatNumber(overview.kpis.contact_count) }}</div>
          <div class="kpi-label">通讯录人数</div>
          <div class="kpi-sub">上次同步 {{ formatRelativeTime(overview.kpis.contact_last_sync || '') }}</div>
        </div>
        <div class="kpi-card" :class="{ 'kpi-danger': overview.kpis.inactive_rate > 50 }">
          <div class="kpi-value">{{ overview.kpis.inactive_rate?.toFixed(1) }}%</div>
          <div class="kpi-label">未使用率</div>
          <div class="kpi-sub">共 {{ formatNumber(overview.kpis.inactive_count || 0) }} 人未使用</div>
        </div>
      </div>

      <el-skeleton v-if="overviewLoading && !overview" :rows="3" animated />
    </el-card>

    <!-- 最近同步任务 + 问题提醒 -->
    <el-row :gutter="16" class="info-row">
      <el-col :span="16">
        <el-card shadow="never">
          <template #header>
            <span class="card-title">最近同步任务</span>
          </template>
          <el-table :data="overview?.recent_syncs || []" stripe size="small" max-height="250">
            <el-table-column label="时间" width="170">
              <template #default="{ row }">{{ formatTime(row.start_time) }}</template>
            </el-table-column>
            <el-table-column prop="sync_type" label="类型" width="80">
              <template #default="{ row }">
                <el-tag :type="row.sync_type === 'log' ? 'primary' : 'success'" size="small">
                  {{ row.sync_type === 'log' ? '日志' : '通讯录' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="trigger" label="触发" width="80">
              <template #default="{ row }">
                <el-tag :type="row.trigger === 'scheduler' ? 'warning' : 'info'" size="small">
                  {{ row.trigger === 'scheduler' ? '定时' : '手动' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="succeeded" label="成功" width="80" align="center" />
            <el-table-column prop="failed" label="失败" width="80" align="center">
              <template #default="{ row }">
                <span :class="row.failed > 0 ? 'text-danger' : ''">{{ row.failed }}</span>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="90" align="center">
              <template #default="{ row }">{{ formatDuration(row.duration_ms) }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!overviewLoading && (!overview?.recent_syncs || overview.recent_syncs.length === 0)" description="尚未执行过同步任务" />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="never">
          <template #header>
            <span class="card-title">需要处理</span>
          </template>
          <div v-if="overview?.problems && overview.problems.length > 0" class="problem-list">
            <div
              v-for="(p, i) in overview?.problems"
              :key="i"
              class="problem-item"
              @click="handleProblemClick(p)"
            >
              <el-tag :type="p.level === 'critical' ? 'danger' : 'warning'" size="small" class="problem-tag">
                {{ p.level === 'critical' ? '异常' : '提醒' }}
              </el-tag>
              <span class="problem-text">{{ p.message }}</span>
            </div>
          </div>
          <div v-else-if="!overviewLoading" class="problem-ok">
            系统运行正常
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 使用分析（原有功能） -->
    <el-card shadow="never" class="analysis-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">使用分析</span>
          <el-button type="primary" :loading="loading" @click="fetchData" size="small">刷新</el-button>
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
          <div class="stat-label">已使用人数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ data.inactive_count }}</div>
          <div class="stat-label">未使用人数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ inactiveRate }}</div>
          <div class="stat-label">未使用率</div>
        </div>
      </div>

      <div v-if="!loading && data" style="margin-top: 12px; color: #909399; font-size: 13px;">
        统计范围：共{{ totalDays }}天，
        统计指标：{{ featureLabel }}，
        筛选条件：未使用天数 ≥ {{ minInactiveDays }}天
      </div>
    </el-card>

    <el-card shadow="never" style="margin-top: 16px;">
      <template #header>
        <div class="card-header">
          <span class="card-title">部门统计</span>
        </div>
      </template>

      <el-table :data="data?.dept_stats || []" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="部门" />
        <el-table-column prop="total" label="总人数" width="100" align="center" />
        <el-table-column prop="active" label="已使用" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="success" size="small">{{ row.active }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="inactive" label="未使用" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.inactive > 0 ? 'danger' : 'info'" size="small">{{ row.inactive }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="使用率" width="100" align="center">
          <template #default="{ row }">
            {{ row.total > 0 ? ((row.active / row.total) * 100).toFixed(0) + '%' : '-' }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" style="margin-top: 16px;">
      <template #header>
        <div class="card-header">
          <span class="card-title">未使用人员列表 ({{ data?.inactive_count || 0 }}人)</span>
          <el-button type="success" size="small" @click="exportCSV" :disabled="!data?.inactive_users?.length">导出 CSV</el-button>
        </div>
      </template>

      <el-table :data="data?.inactive_users || []" stripe v-loading="loading" style="width: 100%" @row-click="handleUserClick" highlight-current-row>
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
        v-if="(data?.inactive_count || 0) > pageSize"
        style="margin-top: 16px; justify-content: center;"
        layout="total, prev, pager, next"
        :total="data?.inactive_count || 0"
        :page-size="pageSize"
        v-model:current-page="currentPage"
        @current-change="onPageChange"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { dashboardAPI, contactAPI } from '../api'
import type { DashboardOverview, InactiveUsersResponse, Department } from '../types/api'
import { ElMessage } from 'element-plus'

const router = useRouter()

const isMounted = { value: true }
onUnmounted(() => { isMounted.value = false })

// 总览看板
const overview = ref<DashboardOverview | null>(null)
const overviewLoading = ref(false)

// 使用分析
const loading = ref(false)
const data = ref<InactiveUsersResponse | null>(null)
const expandedKeys = ref<number[]>([])
const currentPage = ref(1)
const pageSize = 20
const rangeVal = ref('week')
const deptVal = ref<number | undefined>(undefined)
const minDays = ref(1)
const deptTree = ref<Department[]>([])
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

const loadOverview = async () => {
  overviewLoading.value = true
  try {
    const res = await dashboardAPI.getOverview()
    if (!isMounted.value) return
    if (res.code === 0) {
      overview.value = res.data ?? null
    }
  } catch (err) {
    console.error(err)
  } finally {
    if (isMounted.value) overviewLoading.value = false
  }
}

const loadDeptTree = async () => {
  try {
    const res = await contactAPI.getDeptTree()
    if (!isMounted.value) return
    if (res.code === 0) {
      const d = res.data as unknown as { tree: Department[] }
      deptTree.value = d.tree || []
      expandedKeys.value = d.tree?.map((n: Department) => n.id) || []
    }
  } catch (err: unknown) {
    if (isMounted.value) ElMessage.error(err instanceof Error ? err.message : '加载部门树失败')
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    const params: { range: string; min_inactive_days: number; dept_id?: number; page: number; page_size: number } = {
      range: rangeVal.value,
      min_inactive_days: minDays.value,
      page: currentPage.value,
      page_size: pageSize,
    }
    if (deptVal.value) params.dept_id = deptVal.value
    const res = await dashboardAPI.getInactiveUsers(params)
    if (!isMounted.value) return
    if (res.code === 0 && res.data) {
      data.value = res.data
      totalDays.value = res.data.total_days || totalDays.value
      if (minDays.value > totalDays.value) {
        minDays.value = totalDays.value
      }
    }
  } catch (err: unknown) {
    if (isMounted.value) ElMessage.error(err instanceof Error ? err.message : '加载使用分析失败')
  } finally {
    if (isMounted.value) loading.value = false
  }
}

const onFilterChange = () => {
  currentPage.value = 1
  minDays.value = 1
  fetchData()
}

const onPageChange = () => {
  fetchData()
}

const handleProblemClick = (problem: { action?: string }) => {
  if (problem.action) {
    router.push('/' + problem.action)
  }
}

const handleUserClick = (row: { mobile?: string }) => {
  if (row.mobile) {
    router.push({ path: '/query', query: { mobile: row.mobile } })
  }
}

const exportCSV = () => {
  const url = dashboardAPI.exportInactiveUsersURL({
    range: rangeVal.value,
    dept_id: deptVal.value,
    min_inactive_days: minDays.value,
  })
  window.open(url, '_blank')
}

const formatTime = (timeStr: string) => {
  if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return '-'
  return new Date(timeStr).toLocaleString('zh-CN')
}

const formatRelativeTime = (timeStr: string) => {
  if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return '无记录'
  const diff = Date.now() - new Date(timeStr).getTime()
  if (diff < 0) return '刚刚'
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  return `${days} 天前`
}

const formatNumber = (n: number) => {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}

const formatDuration = (ms: number) => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const min = Math.floor(ms / 60000)
  const sec = Math.round((ms % 60000) / 1000)
  return `${min}m${sec}s`
}

onMounted(() => {
  loadOverview()
  loadDeptTree()
  fetchData()
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.overview-card,
.analysis-card {
  margin-bottom: 16px;
}

.info-row {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 500;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

/* KPI 卡片 */
.kpi-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.kpi-card {
  text-align: center;
  padding: 16px 8px;
  background: #f5f7fa;
  border-radius: 8px;
}

.kpi-card.kpi-danger {
  background: #fef0f0;
}

.kpi-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
  line-height: 1.2;
}

.kpi-danger .kpi-value {
  color: #f56c6c;
}

.kpi-label {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
}

.kpi-sub {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 2px;
}

/* 问题提醒 */
.problem-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.problem-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #fafafa;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.2s;
}

.problem-item:hover {
  background: #ecf5ff;
}

.problem-tag {
  flex-shrink: 0;
}

.problem-text {
  font-size: 13px;
  color: #606266;
}

.problem-ok {
  text-align: center;
  color: #67c23a;
  font-size: 14px;
  padding: 20px 0;
}

/* 使用分析 */
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

.text-danger {
  color: #f56c6c;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}
</style>
