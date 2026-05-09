<template>
  <div class="app-container">
    <el-container>
      <el-header>
        <h1>政务微信数据查询平台</h1>
      </el-header>
      <el-container>
        <el-aside width="200px">
          <el-menu
            :default-active="activeMenu"
            class="el-menu-vertical"
            @select="handleMenuSelect"
          >
            <el-menu-item index="query">
              <span>日志查询</span>
            </el-menu-item>
            <el-menu-item index="sync">
              <span>数据同步</span>
            </el-menu-item>
            <el-menu-item index="keys">
              <span>密钥管理</span>
            </el-menu-item>
          </el-menu>
        </el-aside>
        <el-main>
          <LogQuery v-if="activeMenu === 'query'" />
          <DataSync v-else-if="activeMenu === 'sync'" />
          <KeyManagement v-else-if="activeMenu === 'keys'" />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import LogQuery from './components/LogQuery.vue'
import DataSync from './components/DataSync.vue'
import KeyManagement from './components/KeyManagement.vue'

const activeMenu = ref('query')

const handleMenuSelect = (index: string) => {
  activeMenu.value = index
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
}

.el-aside {
  background-color: #f5f5f5;
}

.el-main {
  background-color: #fff;
}
</style>