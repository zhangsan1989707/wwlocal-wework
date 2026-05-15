<template>
  <div class="contact-list">
    <el-card class="sync-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">通讯录同步</span>
          <el-tag :type="(syncStatus.running || asyncSyncStatus.running) ? 'warning' : (syncStatus.error_msg || asyncSyncStatus.error_msg) ? 'danger' : 'success'" size="large">
            {{ syncStatus.running || asyncSyncStatus.running ? '同步中...' : '空闲' }}
          </el-tag>
        </div>
      </template>

      <div v-if="asyncSyncStatus.running" class="sync-progress">
        <el-progress
          :percentage="asyncProgressPercentage"
          :stroke-width="20"
          :show-text="true"
          :format="() => asyncPhaseText"
          status="success"
        />
        <p class="progress-text">
          {{ asyncProgressText }}
        </p>
        <p v-if="asyncSyncStatus.job_id" class="job-id">
          Job ID: {{ asyncSyncStatus.job_id }}
        </p>
      </div>

      <div v-else-if="syncStatus.running" class="sync-progress">
        <el-progress
          :percentage="progressPercentage"
          :stroke-width="20"
          :show-text="true"
          :format="() => syncStatus.phase || '进行中'"
          status="success"
        />
        <p class="progress-text">
          {{ phaseText }} {{ syncStatus.progress }} / {{ syncStatus.total }}
        </p>
        <el-button type="danger" size="small" @click="handleCancel" style="margin-top: 12px">
          请求停止同步
        </el-button>
      </div>

      <div v-else class="sync-actions">
        <el-button type="primary" @click="handleSyncFull" size="large">
          同步全部通讯录
        </el-button>
        <el-button type="primary" plain @click="handleSyncIncremental" size="large">
          同步新增人员
        </el-button>
        <el-divider direction="vertical" />
        <el-button type="success" @click="handleSyncAsyncExport" size="large">
          <el-icon><Download /></el-icon>
          异步导出同步 (推荐)
        </el-button>
        <el-button type="success" plain @click="handleSyncIncrementalAsync" size="large">
          <el-icon><Refresh /></el-icon>
          异步增量同步
        </el-button>
        <span v-if="syncStatus.last_sync || asyncSyncStatus.last_sync" class="last-sync">
          上次同步: {{ formatTime(syncStatus.last_sync || asyncSyncStatus.last_sync) }}
        </span>
      </div>
      <div v-if="syncStatus.error_msg || asyncSyncStatus.error_msg" class="sync-error">
        <el-alert :title="syncStatus.error_msg || asyncSyncStatus.error_msg" type="error" show-icon :closable="false" />
      </div>
    </el-card>

    <el-card class="main-card">
      <div class="main-layout">
        <div class="tree-panel">
          <div class="tree-header">
            <span>组织架构</span>
            <el-tag type="primary" size="small">总人数 {{ totalContacts }}</el-tag>
          </div>
          <el-input
            v-model="treeFilterText"
            placeholder="搜索部门"
            clearable
            size="small"
            style="margin-bottom: 8px"
          />
          <el-tree
            ref="treeRef"
            :data="deptTree"
            :props="{ label: 'name', children: 'children' }"
            :filter-node-method="filterNode"
            node-key="id"
            highlight-current
            default-expand-all
            @node-click="handleDeptClick"
          >
            <template #default="{ data }">
              <span class="tree-node">
                <span class="tree-node-label">{{ data.name }}</span>
                <el-tag size="small" type="info">{{ data.member_count }}</el-tag>
              </span>
            </template>
          </el-tree>
        </div>

        <div class="table-panel">
          <div class="search-bar">
            <el-input
              v-model="searchName"
              placeholder="按姓名搜索"
              clearable
              style="width: 180px"
              @keyup.enter="handleGlobalSearch"
            />
            <el-input
              v-model="searchMobile"
              placeholder="按手机号搜索"
              clearable
              style="width: 180px; margin-left: 12px"
              @keyup.enter="handleGlobalSearch"
            />
            <el-button type="primary" @click="handleGlobalSearch" style="margin-left: 12px">搜索</el-button>
            <el-button @click="handleReset">重置</el-button>
          </div>

          <div v-if="selectedDept" class="dept-breadcrumb">
            <span class="breadcrumb-label">当前部门:</span>
            <el-tag closable @close="clearDeptFilter" size="large">
              {{ selectedDept.name }}
            </el-tag>
            <span class="member-count">共 {{ total }} 人</span>
          </div>

          <el-table
            :data="contacts"
            border
            stripe
            size="small"
            style="margin-top: 12px"
            v-loading="loading"
            highlight-current-row
            @row-click="handleRowClick"
          >
            <el-table-column prop="name" label="姓名" width="120" />
            <el-table-column prop="mobile" label="手机号" width="140" />
            <el-table-column prop="position" label="职位" min-width="120" />
            <el-table-column prop="email" label="邮箱" min-width="180" />
            <el-table-column prop="status" label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : row.status === 2 ? 'danger' : 'info'" size="small">
                  {{ row.status === 1 ? '已激活' : row.status === 2 ? '已禁用' : '未激活' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="userid" label="UserID" width="160" />
          </el-table>

          <el-pagination
            v-if="total > 0"
            :current-page="page"
            :page-size="pageSize"
            :total="total"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            @current-change="handlePageChange"
            @size-change="handleSizeChange"
            style="margin-top: 12px; justify-content: center"
          />
        </div>
      </div>
    </el-card>

    <el-drawer
      v-model="drawerVisible"
      :title="drawerContact?.name || '人员详情'"
      size="420px"
    >
      <el-descriptions v-if="drawerContact" :column="1" border>
        <el-descriptions-item label="姓名">{{ drawerContact.name }}</el-descriptions-item>
        <el-descriptions-item label="手机号">{{ drawerContact.mobile || '-' }}</el-descriptions-item>
        <el-descriptions-item label="性别">
          {{ drawerContact.gender === 1 ? '男' : drawerContact.gender === 2 ? '女' : '未知' }}
        </el-descriptions-item>
        <el-descriptions-item label="邮箱">{{ drawerContact.email || '-' }}</el-descriptions-item>
        <el-descriptions-item label="职位">{{ drawerContact.position || '-' }}</el-descriptions-item>
        <el-descriptions-item label="UserID">{{ drawerContact.userid }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="drawerContact.status === 1 ? 'success' : drawerContact.status === 2 ? 'danger' : 'info'" size="small">
            {{ drawerContact.status === 1 ? '已激活' : drawerContact.status === 2 ? '已禁用' : '未激活' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="所属部门">{{ deptNames }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="drawerContact?.mobile" style="margin-top: 16px">
        <el-button type="primary" @click="viewUserLogs">
          <el-icon><Search /></el-icon>
          查看该人员日志
        </el-button>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, inject } from 'vue'
import { contactAPI } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Download, Refresh } from '@element-plus/icons-vue'

const navigate = inject('navigate') as (menu: string, params?: any) => void

const contacts = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const searchName = ref('')
const searchMobile = ref('')
const loading = ref(false)
const syncStatus = ref<any>({ running: false })
const asyncSyncStatus = ref<any>({ running: false })
let pollTimer: ReturnType<typeof setInterval> | null = null
let asyncPollTimer: ReturnType<typeof setInterval> | null = null

const deptTree = ref<any[]>([])
const totalContacts = ref(0)
const treeFilterText = ref('')
const treeRef = ref<any>(null)
const selectedDept = ref<any | null>(null)

const drawerVisible = ref(false)
const drawerContact = ref<any | null>(null)

const progressPercentage = computed(() => {
  if (!syncStatus.value || syncStatus.value.total === 0) return 0
  return Math.round((syncStatus.value.progress / syncStatus.value.total) * 100)
})

const phaseText = computed(() => {
  const phase = syncStatus.value.phase
  if (phase === 'departments') return '同步部门...'
  if (phase === 'members') return '获取成员列表...'
  if (phase === 'details') return '拉取成员详情:'
  return ''
})

const asyncProgressPercentage = computed(() => {
  if (!asyncSyncStatus.value || asyncSyncStatus.value.total === 0) return 0
  return Math.round((asyncSyncStatus.value.progress / asyncSyncStatus.value.total) * 100)
})

const asyncPhaseText = computed(() => {
  const phase = asyncSyncStatus.value.phase
  if (phase === 'init') return '初始化...'
  if (phase === 'exporting') return '正在导出数据...'
  if (phase === 'importing') return '正在导入数据...'
  if (phase === 'comparing') return '比对差异...'
  if (phase === 'done') return '同步完成'
  if (phase === 'error') return '同步失败'
  return phase || '处理中'
})

const asyncProgressText = computed(() => {
  if (asyncSyncStatus.value.imported > 0 || asyncSyncStatus.value.failed > 0) {
    return `已导入 ${asyncSyncStatus.value.imported}, 失败 ${asyncSyncStatus.value.failed} / ${asyncSyncStatus.value.total}`
  }
  if (asyncSyncStatus.value.progress > 0 && asyncSyncStatus.value.total > 0) {
    return `${asyncSyncStatus.value.progress} / ${asyncSyncStatus.value.total}`
  }
  return '等待服务端处理...'
})

const deptNames = computed(() => {
  if (!drawerContact.value?.department) return '-'
  try {
    const ids = JSON.parse(drawerContact.value.department)
    return ids.map((id: number) => {
      const dept = findDeptById(deptTree.value, id)
      return dept ? dept.name : `部门${id}`
    }).join('、') || '-'
  } catch {
    return '-'
  }
})

function findDeptById(tree: any[], id: number): any | null {
  for (const node of tree) {
    if (node.id === id) return node
    if (node.children) {
      const found = findDeptById(node.children, id)
      if (found) return found
    }
  }
  return null
}

const formatTime = (timeStr: string) => {
  if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return '-'
  return new Date(timeStr).toLocaleString('zh-CN')
}

watch(treeFilterText, (val) => {
  treeRef.value?.filter(val)
})

onMounted(async () => {
  await loadDeptTree()
  await loadContacts()
  await checkSyncStatus()
  await checkAsyncSyncStatus()
  if (syncStatus.value.running) startPolling()
  if (asyncSyncStatus.value.running) startAsyncPolling()
})

onUnmounted(() => {
  stopPolling()
  stopAsyncPolling()
})

const loadDeptTree = async () => {
  try {
    const res: any = await contactAPI.getDeptTree()
    if (res.code === 0) {
      deptTree.value = res.data?.tree || []
      totalContacts.value = res.data?.total || 0
    }
  } catch (err) {
    console.error(err)
  }
}

const loadDeptMembers = async () => {
  if (!selectedDept.value) return
  loading.value = true
  try {
    const res: any = await contactAPI.getDeptMembers(selectedDept.value.id, {
      page: page.value,
      page_size: pageSize.value,
    })
    if (res.code === 0) {
      contacts.value = res.data.data || []
      total.value = res.data.total || 0
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

const loadContacts = async () => {
  loading.value = true
  try {
    const res: any = await contactAPI.list({
      page: page.value,
      page_size: pageSize.value,
      name: searchName.value,
      mobile: searchMobile.value,
    })
    if (res.code === 0) {
      contacts.value = res.data.data || []
      total.value = res.data.total || 0
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

const handleDeptClick = (data: any) => {
  selectedDept.value = data
  page.value = 1
  searchName.value = ''
  searchMobile.value = ''
  loadDeptMembers()
}

const clearDeptFilter = () => {
  selectedDept.value = null
  page.value = 1
  treeRef.value?.setCurrentKey(null)
  loadContacts()
}

const handleGlobalSearch = () => {
  selectedDept.value = null
  page.value = 1
  treeRef.value?.setCurrentKey(null)
  loadContacts()
}

const handleReset = () => {
  selectedDept.value = null
  searchName.value = ''
  searchMobile.value = ''
  page.value = 1
  pageSize.value = 20
  treeRef.value?.setCurrentKey(null)
  loadContacts()
  loadDeptTree()
}

const handlePageChange = (p: number) => {
  page.value = p
  if (selectedDept.value) {
    loadDeptMembers()
  } else {
    loadContacts()
  }
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  page.value = 1
  if (selectedDept.value) {
    loadDeptMembers()
  } else {
    loadContacts()
  }
}

const handleRowClick = async (row: any) => {
  try {
    const res: any = await contactAPI.getContact(row.userid || row.user_id)
    if (res.code === 0) {
      drawerContact.value = res.data
      drawerVisible.value = true
    }
  } catch (err) {
    ElMessage.error('获取人员详情失败')
  }
}

const viewUserLogs = () => {
  if (drawerContact.value?.mobile) {
    drawerVisible.value = false
    navigate('query', { mobile: drawerContact.value.mobile })
  }
}

const filterNode = (value: string, data: any) => {
  if (!value) return true
  return data.name.includes(value)
}

const handleSyncFull = async () => {
  try {
    await ElMessageBox.confirm(
      '将从政务微信拉取所有通讯录数据（对新用户逐个获取详情），确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await contactAPI.sync()
    if (res.code === 0) {
      ElMessage.success('通讯录同步已启动')
      await checkSyncStatus()
      startPolling()
    }
  } catch (err: any) {
    ElMessage.error(err.message || '同步启动失败')
  }
}

const handleSyncIncremental = async () => {
  try {
    await ElMessageBox.confirm(
      '将只拉取新增用户详情，确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await contactAPI.syncIncremental()
    if (res.code === 0) {
      ElMessage.success('通讯录同步已启动')
      await checkSyncStatus()
      startPolling()
    }
  } catch (err: any) {
    ElMessage.error(err.message || '同步启动失败')
  }
}

const handleCancel = async () => {
  try {
    const res: any = await contactAPI.cancel()
    if (res.code === 0) ElMessage.success('已发送取消请求')
  } catch (err: any) {
    ElMessage.error(err.message || '取消失败')
  }
}

const checkSyncStatus = async () => {
  try {
    const res: any = await contactAPI.status()
    if (res.code === 0) syncStatus.value = res.data
  } catch (err) { console.error(err) }
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(async () => {
    await checkSyncStatus()
    if (!syncStatus.value.running) {
      stopPolling()
      await loadDeptTree()
      if (selectedDept.value) {
        await loadDeptMembers()
      } else {
        await loadContacts()
      }
    }
  }, 2000)
}

const stopPolling = () => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

const checkAsyncSyncStatus = async () => {
  try {
    const res: any = await contactAPI.asyncSyncStatus()
    if (res.code === 0) asyncSyncStatus.value = res.data
  } catch (err) { console.error(err) }
}

const startAsyncPolling = () => {
  stopAsyncPolling()
  asyncPollTimer = setInterval(async () => {
    await checkAsyncSyncStatus()
    if (!asyncSyncStatus.value.running) {
      stopAsyncPolling()
      await loadDeptTree()
      if (selectedDept.value) {
        await loadDeptMembers()
      } else {
        await loadContacts()
      }
    }
  }, 2000)
}

const stopAsyncPolling = () => {
  if (asyncPollTimer) { clearInterval(asyncPollTimer); asyncPollTimer = null }
}

const handleSyncAsyncExport = async () => {
  try {
    await ElMessageBox.confirm(
      '使用政务微信异步导出接口，适用于大规模通讯录数据同步（推荐用于9万人以上场景），确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await contactAPI.syncAsyncExport()
    if (res.code === 0) {
      ElMessage.success('异步同步已启动')
      await checkAsyncSyncStatus()
      startAsyncPolling()
    }
  } catch (err: any) {
    ElMessage.error(err.message || '同步启动失败')
  }
}

const handleSyncIncrementalAsync = async () => {
  try {
    await ElMessageBox.confirm(
      '使用异步方式仅同步新增用户，确定开始吗？',
      '确认同步',
      { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' }
    )
  } catch { return }

  try {
    const res: any = await contactAPI.syncIncrementalAsync()
    if (res.code === 0) {
      ElMessage.success('异步增量同步已启动')
      await checkAsyncSyncStatus()
      startAsyncPolling()
    }
  } catch (err: any) {
    ElMessage.error(err.message || '同步启动失败')
  }
}
</script>

<style scoped>
.contact-list {
  padding: 0;
}

.sync-card {
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

.sync-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sync-progress {
  margin-bottom: 12px;
}

.progress-text {
  text-align: center;
  color: #606266;
  margin-top: 10px;
}

.job-id {
  text-align: center;
  color: #909399;
  margin-top: 8px;
  font-size: 13px;
  font-family: monospace;
}

.last-sync {
  color: #909399;
  font-size: 14px;
}

.sync-error {
  margin-top: 12px;
}

.main-card :deep(.el-card__body) {
  padding: 0;
}

.main-layout {
  display: flex;
  min-height: 500px;
}

.tree-panel {
  width: 260px;
  border-right: 1px solid #ebeef5;
  padding: 12px;
  overflow-y: auto;
  flex-shrink: 0;
}

.tree-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.tree-node {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
  padding-right: 4px;
}

.tree-node-label {
  font-size: 13px;
}

.table-panel {
  flex: 1;
  padding: 16px;
  overflow-x: auto;
}

.search-bar {
  display: flex;
  align-items: center;
}

.dept-breadcrumb {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.breadcrumb-label {
  font-size: 13px;
  color: #909399;
}

.member-count {
  font-size: 13px;
  color: #909399;
  margin-left: auto;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  border-bottom: 1px solid #ebeef5;
}

:deep(.el-tree-node__content) {
  height: 32px;
}
</style>
