<template>
  <Login v-if="!authStore.isLoggedIn" />
  <div v-else class="app-container">
    <el-container>
      <el-header>
        <div class="header-left">
          <h1>政务微信开放数据审计平台</h1>
          <span class="env-tag">本地部署</span>
        </div>
        <div class="header-right">
          <span>{{ authStore.username }}</span>
          <span class="header-action" @click="showPwDialog = true">修改密码</span>
          <span class="header-divider">|</span>
          <span class="header-action" @click="handleLogout">退出</span>
        </div>
      </el-header>
      <el-container class="main-container">
        <el-aside width="200px">
          <el-menu
            :default-active="activeMenu"
            :default-openeds="defaultOpeneds"
            unique-opened
            class="el-menu-vertical"
            @select="handleMenuSelect"
          >
            <el-sub-menu index="business">
              <template #title>
                <el-icon><DataLine /></el-icon>
                <span>运营</span>
              </template>
              <el-menu-item index="dashboard" title="运营总览">
                <el-icon><DataLine /></el-icon>
                <span>运营总览</span>
              </el-menu-item>
              <el-menu-item index="query" title="日志审计">
                <el-icon><Document /></el-icon>
                <span>日志审计</span>
              </el-menu-item>
              <el-menu-item index="behavior" title="行为查询">
                <el-icon><Search /></el-icon>
                <span>行为查询</span>
              </el-menu-item>
              <el-menu-item index="contacts" title="通讯录">
                <el-icon><User /></el-icon>
                <span>通讯录</span>
              </el-menu-item>
            </el-sub-menu>
            <el-sub-menu index="data-ops">
              <template #title>
                <el-icon><Refresh /></el-icon>
                <span>数据运维</span>
              </template>
              <el-menu-item v-if="authStore.role === 'super_admin'" index="sync" title="同步任务">
                <el-icon><Refresh /></el-icon>
                <span>同步任务</span>
              </el-menu-item>
              <el-menu-item index="adminoper" title="企微操作日志">
                <el-icon><Setting /></el-icon>
                <span>企微操作日志</span>
              </el-menu-item>
              <el-menu-item v-if="authStore.role === 'super_admin'" index="features" title="数据类型配置">
                <el-icon><Setting /></el-icon>
                <span>数据类型配置</span>
              </el-menu-item>
            </el-sub-menu>
            <el-sub-menu v-if="authStore.role === 'super_admin'" index="system-admin">
              <template #title>
                <el-icon><Monitor /></el-icon>
                <span>系统管理</span>
              </template>
              <el-menu-item index="ops-dashboard" title="运维中心">
                <el-icon><DataLine /></el-icon>
                <span>运维中心</span>
              </el-menu-item>
              <el-menu-item index="system" title="系统状态">
                <el-icon><Monitor /></el-icon>
                <span>系统状态</span>
              </el-menu-item>
              <el-menu-item index="keys" title="密钥管理">
                <el-icon><Key /></el-icon>
                <span>密钥管理</span>
              </el-menu-item>
              <el-menu-item index="users" title="用户权限">
                <el-icon><User /></el-icon>
                <span>用户权限</span>
              </el-menu-item>
            </el-sub-menu>
          </el-menu>
        </el-aside>
        <el-main class="main-content">
          <router-view />
        </el-main>
      </el-container>
    </el-container>

    <el-dialog v-model="showPwDialog" title="修改密码" width="400px" :close-on-click-modal="false">
      <el-form :model="pwForm" label-width="80px">
        <el-form-item label="旧密码">
          <el-input v-model="pwForm.old_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwForm.new_password" type="password" show-password placeholder="至少 8 位，含大小写字母和数字" />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="pwForm.confirm" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPwDialog = false">取消</el-button>
        <el-button type="primary" :loading="pwLoading" @click="handleChangePassword">确认修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from './stores/auth'
import { authAPI } from './api'
import Login from './components/Login.vue'
import {
  DataLine, Document, Refresh, User, Setting, Key, Monitor,
  Search,
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const activeMenu = computed(() => {
  const path = route.path.slice(1) || 'dashboard'
  return path
})

const defaultOpeneds = computed(() => {
  const dataOpsMenus = ['sync', 'adminoper', 'features']
  const systemAdminMenus = ['ops-dashboard', 'system', 'keys', 'users']

  if (dataOpsMenus.includes(activeMenu.value)) {
    return ['data-ops']
  }
  if (systemAdminMenus.includes(activeMenu.value)) {
    return ['system-admin']
  }
  return ['business']
})

const handleMenuSelect = (index: string) => {
  router.push('/' + index)
}

const handleLogout = () => {
  authStore.logout()
  router.push('/dashboard')
}

const showPwDialog = ref(false)
const pwLoading = ref(false)
const pwForm = ref({ old_password: '', new_password: '', confirm: '' })

const handleChangePassword = async () => {
  if (pwForm.value.new_password.length < 8) {
    ElMessage.warning('新密码长度不能少于 8 位')
    return
  }
  const pw = pwForm.value.new_password
  if (!/[A-Z]/.test(pw) || !/[a-z]/.test(pw) || !/[0-9]/.test(pw)) {
    ElMessage.warning('新密码必须包含大小写字母和数字')
    return
  }
  if (pwForm.value.new_password !== pwForm.value.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  pwLoading.value = true
  try {
    const res = await authAPI.changePassword({
      old_password: pwForm.value.old_password,
      new_password: pwForm.value.new_password,
    })
    if (res.code === 0) {
      ElMessage.success('密码修改成功，请重新登录')
      showPwDialog.value = false
      pwForm.value = { old_password: '', new_password: '', confirm: '' }
      authStore.logout()
      router.push('/dashboard')
    } else {
      ElMessage.error(res.msg || '修改失败')
    }
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '修改失败')
  } finally {
    pwLoading.value = false
  }
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
  overflow: hidden;
}

.app-container {
  height: 100%;
}

.el-header {
  background-color: #1a365d;
  color: white;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 52px !important;
  box-shadow: 0 1px 4px rgba(0,0,0,.15);
  position: relative;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h1 {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.5px;
}

.env-tag {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 3px;
  background: rgba(255,255,255,.15);
  color: rgba(255,255,255,.8);
  border: 1px solid rgba(255,255,255,.2);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  color: rgba(255,255,255,.85);
}

.header-action {
  cursor: pointer;
  color: rgba(255,255,255,.85);
  transition: color 0.2s;
}

.header-action:hover {
  color: #fff;
}

.header-divider {
  color: rgba(255,255,255,.3);
  font-size: 12px;
}

.main-container {
  height: calc(100vh - 52px);
  overflow: hidden;
}

.el-aside {
  background-color: #f8f9fa;
  border-right: 1px solid #e8e8e8;
  transition: width 0.3s ease;
  overflow: hidden;
  flex-shrink: 0;
  height: 100%;
  position: sticky;
  top: 0;
}

.el-menu-vertical {
  border-right: none;
  height: 100%;
  overflow-y: auto;
}

.main-content {
  background-color: #f0f2f5;
  padding: 16px;
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  height: 100%;
}
</style>
