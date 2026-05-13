<template>
  <Login v-if="!isLoggedIn" @login="handleLogin" />
  <div v-else class="app-container">
    <el-container>
      <el-header>
        <h1>政务微信数据查询平台</h1>
        <div class="header-right">
          <span>{{ username }}</span>
          <el-button type="text" style="color: #fff" @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-container>
        <el-aside width="200px">
          <el-menu
            :default-active="activeMenu"
            class="el-menu-vertical"
            @select="handleMenuSelect"
          >
            <el-menu-item index="dashboard">
              <span>看板</span>
            </el-menu-item>
            <el-menu-item index="query">
              <span>日志查询</span>
            </el-menu-item>
            <el-menu-item index="sync">
              <span>数据同步</span>
            </el-menu-item>
            <el-menu-item index="keys">
              <span>密钥管理</span>
            </el-menu-item>
            <el-menu-item index="contacts">
              <span>通讯录</span>
            </el-menu-item>
            <el-menu-item index="opslog">
              <span>操作日志</span>
            </el-menu-item>
          </el-menu>
        </el-aside>
        <el-main>
          <Dashboard v-if="activeMenu === 'dashboard'" />
          <LogQuery v-else-if="activeMenu === 'query'" />
          <DataSync v-else-if="activeMenu === 'sync'" />
          <KeyManagement v-else-if="activeMenu === 'keys'" />
          <ContactList v-else-if="activeMenu === 'contacts'" />
          <OperationLog v-else-if="activeMenu === 'opslog'" />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Login from './components/Login.vue'
import Dashboard from './components/Dashboard.vue'
import LogQuery from './components/LogQuery.vue'
import DataSync from './components/DataSync.vue'
import KeyManagement from './components/KeyManagement.vue'
import ContactList from './components/ContactList.vue'
import OperationLog from './components/OperationLog.vue'

const activeMenu = ref('dashboard')
const isLoggedIn = ref(!!localStorage.getItem('token'))
const username = ref(localStorage.getItem('username') || '')

const handleMenuSelect = (index: string) => {
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
  background-color: #409eff;
  color: white;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 14px;
}

.el-aside {
  background-color: #f5f5f5;
}

.el-main {
  background-color: #fff;
}
</style>
