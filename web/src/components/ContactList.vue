<template>
  <div class="contact-list">
    <el-card class="sync-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">通讯录同步</span>
          <el-tag :type="syncStatus.running ? 'warning' : syncStatus.error_msg ? 'danger' : 'success'" size="large">
            {{ syncStatus.running ? '同步中...' : '空闲' }}
          </el-tag>
        </div>
      </template>
      <div v-if="syncStatus.running" class="sync-progress">
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
        <span v-if="syncStatus.last_sync" class="last-sync">
          上次同步: {{ formatTime(syncStatus.last_sync) }}
        </span>
      </div>
      <div v-if="syncStatus.error_msg" class="sync-error">
        <el-alert :title="syncStatus.error_msg" type="error" show-icon :closable="false" />
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
        <el-descriptions-item label="UserID">{{ drawerContact.user_id }}</el-descriptions-item>
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
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { contactAPI } from '../api'
import type { ApiResponse, Contact, ContactSyncStatus, Department, DeptMember, PaginatedResponse } from '../types/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'

const router = useRouter()

const contacts = ref<(Contact | DeptMember)[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const searchName = ref('')
const searchMobile = ref('')
const loading = ref(false)
const syncStatus = ref<ContactSyncStatus>({ running: false, progress: 0, total: 0, synced: 0, failed: 0 })
let pollTimer: ReturnType<typeof setInterval> | null = null

const deptTree = ref<Department[]>([])
const totalContacts = ref(0)
const treeFilterText = ref('')
const treeRef = ref<{ filter: (val: string) => void; setCurrentKey: (key: number | null) => void } | null>(null)
const selectedDept = ref<Department | null>(null)

const drawerVisible = ref(false)
const drawerContact = ref<Contact | null>(null)

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

const deptNames = computed(() => {
  const dept = drawerContact.value?.department
  if (!dept) return '-'
  try {
    const ids: number[] = typeof dept === 'string' ? JSON.parse(dept) : Array.isArray(dept) ? dept : []
    return ids.map((id: number) => {
      const d = findDeptById(deptTree.value, id)
      return d ? d.name : `部门${id}`
    }).join('、') || '-'
  } catch {
    return '-'
  }
})

function findDeptById(tree: Department[], id: number): Department | null {
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
  if (syncStatus.value.running) startPolling()
})

onUnmounted(() => {
  stopPolling()
})

const loadDeptTree = async () => {
  try {
    const res = await contactAPI.getDeptTree() as unknown as { code: number; data?: { tree: Department[]; total: number } }
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
    const res = await contactAPI.getDeptMembers(selectedDept.value.id, {
      page: page.value,
      page_size: pageSize.value,
    }) as unknown as ApiResponse<PaginatedResponse<DeptMember>>
    if (res.code === 0 && res.data) {
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
    const res = await contactAPI.list({
      page: page.value,
      page_size: pageSize.value,
      name: searchName.value,
      mobile: searchMobile.value,
    }) as unknown as ApiResponse<PaginatedResponse<Contact>>
    if (res.code === 0 && res.data) {
      contacts.value = res.data.data || []
      total.value = res.data.total || 0
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

const handleDeptClick = (data: Department) => {
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

const handleRowClick = async (row: Contact | DeptMember) => {
  try {
    const uid = 'userid' in row ? row.userid : row.user_id
    const res = await contactAPI.getContact(uid) as unknown as ApiResponse<Contact>
    if (res.code === 0) {
      const data = res.data
      if (data) {
        drawerContact.value = data
        drawerVisible.value = true
      }
    }
  } catch (err) {
    ElMessage.error('获取人员详情失败')
  }
}

const viewUserLogs = () => {
  if (drawerContact.value?.mobile) {
    drawerVisible.value = false
    router.push({ path: '/query', query: { mobile: drawerContact.value.mobile } })
  }
}

const filterNode = (value: string, data: Department) => {
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
    const res = await contactAPI.sync() as unknown as ApiResponse<{ message: string }>
    if (res.code === 0) {
      ElMessage.success('通讯录同步已启动')
      await checkSyncStatus()
      startPolling()
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '同步启动失败')
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
    const res = await contactAPI.syncIncremental() as unknown as ApiResponse<{ message: string }>
    if (res.code === 0) {
      ElMessage.success('通讯录同步已启动')
      await checkSyncStatus()
      startPolling()
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '同步启动失败')
  }
}

const handleCancel = async () => {
  try {
    const res = await contactAPI.cancel() as unknown as ApiResponse<{ message: string }>
    if (res.code === 0) ElMessage.success('已发送取消请求')
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '取消失败')
  }
}

const checkSyncStatus = async () => {
  try {
    const res = await contactAPI.status() as unknown as ApiResponse<ContactSyncStatus>
    if (res.code === 0 && res.data) syncStatus.value = res.data
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
  width: 280px;
  min-width: 280px;
  border-right: 1px solid #ebeef5;
  padding: 12px;
  overflow-y: auto;
  overflow-x: hidden;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}

.tree-panel :deep(.el-tree) {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  max-height: calc(100vh - 380px);
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
  min-width: 0;
  overflow: hidden;
}

.tree-node-label {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
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
  overflow: hidden;
}
</style>
