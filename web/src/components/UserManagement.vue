<template>
  <div class="user-management">
    <div class="page-header">
      <h2>用户权限管理</h2>
      <el-button type="primary" @click="openCreate">新建用户</el-button>
    </div>

    <el-table :data="users" stripe v-loading="loading" style="width: 100%">
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column label="角色" width="140">
        <template #default="{ row }">
          <el-tag :type="row.role === 'super_admin' ? 'danger' : 'primary'">
            {{ roleLabel(row.role) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="授权部门" min-width="220">
        <template #default="{ row }">
          {{ deptNames(row.dept_ids) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="openReset(row)">重置密码</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑用户' : '新建用户'" width="520px" :close-on-click-modal="false">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="!!editing" />
        </el-form-item>
        <el-form-item v-if="!editing" label="密码">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="超级管理员" value="super_admin" />
            <el-option label="部门管理员" value="dept_admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
        <el-form-item v-if="form.role === 'dept_admin'" label="授权部门">
          <el-tree-select
            v-model="form.dept_ids"
            :data="deptTree"
            multiple
            show-checkbox
            check-strictly
            node-key="id"
            :props="{ label: 'name', children: 'children' }"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="resetVisible" title="重置密码" width="420px" :close-on-click-modal="false">
      <el-input v-model="resetPassword" type="password" show-password placeholder="不少于 8 位" />
      <template #footer>
        <el-button @click="resetVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePassword">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { contactAPI, userAPI } from '../api'
import type { AdminUser, Department } from '../types/api'

const users = ref<AdminUser[]>([])
const deptTree = ref<Department[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const resetVisible = ref(false)
const editing = ref<AdminUser | null>(null)
const resetting = ref<AdminUser | null>(null)
const resetPassword = ref('')

const form = ref({
  username: '',
  password: '',
  role: 'dept_admin',
  enabled: true,
  dept_ids: [] as number[],
})

function roleLabel(role: string) {
  return role === 'super_admin' ? '超级管理员' : '部门管理员'
}

function collectDeptNames(nodes: Department[], ids: Set<number>, names: string[]) {
  for (const node of nodes) {
    if (ids.has(node.id)) names.push(node.name)
    if (node.children?.length) collectDeptNames(node.children, ids, names)
  }
}

function deptNames(ids: number[]) {
  if (!ids?.length) return '-'
  const names: string[] = []
  collectDeptNames(deptTree.value, new Set(ids), names)
  return names.length ? names.join('、') : ids.join('、')
}

async function fetchData() {
  loading.value = true
  try {
    const [userRes, deptRes] = await Promise.all([userAPI.list(), contactAPI.getDeptTree()])
    if (userRes.code === 0) users.value = userRes.data || []
    if (deptRes.code === 0) deptTree.value = deptRes.data?.tree || []
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = { username: '', password: '', role: 'dept_admin', enabled: true, dept_ids: [] }
  dialogVisible.value = true
}

function openEdit(user: AdminUser) {
  editing.value = user
  form.value = {
    username: user.username,
    password: '',
    role: user.role,
    enabled: user.enabled,
    dept_ids: [...(user.dept_ids || [])],
  }
  dialogVisible.value = true
}

async function saveUser() {
  if (!form.value.username) {
    ElMessage.warning('请输入用户名')
    return
  }
  if (!editing.value && form.value.password.length < 8) {
    ElMessage.warning('密码长度不能少于 8 位')
    return
  }
  saving.value = true
  try {
    const payload = {
      username: form.value.username,
      password: form.value.password,
      role: form.value.role,
      enabled: form.value.enabled,
      dept_ids: form.value.role === 'dept_admin' ? form.value.dept_ids : [],
    }
    const res = editing.value
      ? await userAPI.update(editing.value.id, payload)
      : await userAPI.create(payload)
    if (res.code === 0) {
      ElMessage.success('保存成功')
      dialogVisible.value = false
      fetchData()
    } else {
      ElMessage.error(res.msg || '保存失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

function openReset(user: AdminUser) {
  resetting.value = user
  resetPassword.value = ''
  resetVisible.value = true
}

async function savePassword() {
  if (!resetting.value || resetPassword.value.length < 8) {
    ElMessage.warning('密码长度不能少于 8 位')
    return
  }
  saving.value = true
  try {
    const res = await userAPI.resetPassword(resetting.value.id, resetPassword.value)
    if (res.code === 0) {
      ElMessage.success('密码已重置')
      resetVisible.value = false
    } else {
      ElMessage.error(res.msg || '重置失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '重置失败')
  } finally {
    saving.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.user-management {
  padding: 0;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
}

.page-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: #303133;
}
</style>
