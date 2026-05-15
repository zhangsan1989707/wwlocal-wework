<template>
  <Login v-if="!isLoggedIn" @login="handleLogin" />
  <div v-else class="app-container">
    <el-container>
      <el-header>
        <div class="header-left">
          <h1>政务微信开放数据审计平台</h1>
          <span class="env-tag">本地部署</span>
        </div>
        <div class="header-right">
          <span>{{ username }}</span>
          <el-button type="text" style="color: rgba(255,255,255,.85)" @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-container>
        <el-aside :width="isCollapsed ? '64px' : '200px'">
          <div class="collapse-btn" @click="isCollapsed = !isCollapsed">
            <el-icon :size="16">
              <DArrowLeft v-if="!isCollapsed" />
              <DArrowRight v-else />
            </el-icon>
          </div>
          <el-menu
            :default-active="activeMenu"
            :collapse="isCollapsed"
            :collapse-transition="false"
            class="el-menu-vertical"
            @select="handleMenuSelect"
          >
            <el-menu-item index="dashboard" title="总览看板">
              <el-icon><DataLine /></el-icon>
              <span>总览看板</span>
            </el-menu-item>
            <el-menu-item index="query" title="日志审计">
              <el-icon><Document /></el-icon>
              <span>日志审计</span>
            </el-menu-item>
            <el-menu-item index="contacts" title="通讯录">
              <el-icon><User /></el-icon>
              <span>通讯录</span>
            </el-menu-item>
            <el-menu-item index="sync" title="同步任务">
              <el-icon><Refresh /></el-icon>
              <span>同步任务</span>
            </el-menu-item>
            <el-menu-item index="adminoper" title="企微操作日志">
              <el-icon><Setting /></el-icon>
              <span>企微操作日志</span>
            </el-menu-item>
            <el-menu-item index="keys" title="密钥管理">
              <el-icon><Key /></el-icon>
              <span>密钥管理</span>
            </el-menu-item>
            <el-menu-item index="features" title="数据类型配置">
              <el-icon><Setting /></el-icon>
              <span>数据类型配置</span>
            </el-menu-item>
            <el-menu-item index="system" title="系统状态">
              <el-icon><Monitor /></el-icon>
              <span>系统状态</span>
            </el-menu-item>
          </el-menu>
        </el-aside>
        <el-main>
          <Dashboard v-if="activeMenu === 'dashboard'" />
          <LogQuery v-else-if="activeMenu === 'query'" />
          <DataSync v-else-if="activeMenu === 'sync'" />
          <FeatureConfig v-else-if="activeMenu === 'features'" />
          <KeyManagement v-else-if="activeMenu === 'keys'" />
          <ContactList v-else-if="activeMenu === 'contacts'" />
          <AdminOperLog v-else-if="activeMenu === 'adminoper'" />
          <SystemStatus v-else-if="activeMenu === 'system'" />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref, provide } from 'vue'
import Login from './components/Login.vue'
import Dashboard from './components/Dashboard.vue'
import LogQuery from './components/LogQuery.vue'
import DataSync from './components/DataSync.vue'
import KeyManagement from './components/KeyManagement.vue'
import ContactList from './components/ContactList.vue'
import AdminOperLog from './components/AdminOperLog.vue'
import FeatureConfig from './components/FeatureConfig.vue'
import SystemStatus from './components/SystemStatus.vue'
import {
  DataLine, Document, Refresh, User, Setting, Key, Monitor,
  DArrowLeft, DArrowRight,
} from '@element-plus/icons-vue'

const activeMenu = ref('dashboard')
const isLoggedIn = ref(!!localStorage.getItem('token'))
const username = ref(localStorage.getItem('username') || '')
const isCollapsed = ref(false)
const navigateParams = ref<any>(null)

const navigate = (menu: string, params?: any) => {
  navigateParams.value = params || null
  activeMenu.value = menu
}

provide('navigate', navigate)
provide('navigateParams', navigateParams)

const handleMenuSelect = (index: string) => {
  navigateParams.value = null
  activeMenu.value = index
}

const handleLogin = (user: string) => {
  isLoggedIn.value = true
  username.value = user
}

const handleLogout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  isLoggedIn.value = false
  username.value = ''
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

.el-aside {
  background-color: #f8f9fa;
  border-right: 1px solid #e8e8e8;
  transition: width 0.3s ease;
  overflow: hidden;
  flex-shrink: 0;
}

.collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 40px;
  cursor: pointer;
  color: #909399;
  border-bottom: 1px solid #e8e8e8;
  transition: color 0.2s, background-color 0.2s;
}

.collapse-btn:hover {
  color: #2b6cb0;
  background-color: #ecf5ff;
}

.el-menu-vertical {
  border-right: none;
}

/* 折叠态：隐藏文字 */
.el-menu--collapse .el-menu-item span {
  display: none;
}

/* 折叠态 tooltip */
.el-menu--collapse .el-menu-item {
  position: relative;
}

.el-main {
  background-color: #f0f2f5;
  padding: 16px;
  flex: 1;
  min-width: 0;
}
</style>
