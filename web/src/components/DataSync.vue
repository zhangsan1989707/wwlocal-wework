<template>
  <div class="data-sync">
    <el-card>
      <template #header>
        <span>数据同步</span>
      </template>
      <el-form label-width="120px">
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="同步选项">
          <el-checkbox v-model="syncAll">同步所有数据类型</el-checkbox>
        </el-form-item>
        <el-form-item label="数据类型" v-if="!syncAll">
          <el-select v-model="form.feature_ids" multiple placeholder="请选择" style="width: 100%">
            <el-option
              v-for="item in features"
              :key="item.id"
              :label="`${item.id} - ${item.name}`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSync" :loading="syncing">开始同步</el-button>
          <el-button @click="handleCheckStatus">检查状态</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top: 16px" v-if="syncStatus">
      <template #header>
        <span>同步状态</span>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="同步状态">
          <el-tag :type="syncStatus.running ? 'warning' : 'success'">
            {{ syncStatus.running ? '同步中' : '空闲' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { syncAPI, logAPI } from '../api'
import { ElMessage } from 'element-plus'

const dateRange = ref<[Date, Date] | null>(null)
const syncing = ref(false)
const syncStatus = ref<any>(null)
const syncAll = ref(true)
const form = reactive({
  feature_ids: [] as number[],
})
const features = ref<any[]>([])

onMounted(async () => {
  try {
    const res: any = await logAPI.getFeatures()
    if (res.code === 0) {
      features.value = res.data
    }
    await handleCheckStatus()
  } catch (err) {
    console.error(err)
  }
})

const handleSync = async () => {
  if (!dateRange.value) {
    ElMessage.warning('请选择时间范围')
    return
  }

  syncing.value = true
  try {
    const startTime = Math.floor(dateRange.value[0].getTime() / 1000)
    const endTime = Math.floor(dateRange.value[1].getTime() / 1000)

    const res: any = await syncAPI.sync({
      sync_all: syncAll.value,
      feature_ids: form.feature_ids,
      start_time: startTime,
      end_time: endTime,
    })

    if (res.code === 0) {
      ElMessage.success('同步已启动')
      await handleCheckStatus()
    } else {
      ElMessage.error(res.msg || '同步启动失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '同步启动失败')
  } finally {
    syncing.value = false
  }
}

const handleCheckStatus = async () => {
  try {
    const res: any = await syncAPI.status()
    if (res.code === 0) {
      syncStatus.value = res.data
    }
  } catch (err) {
    console.error(err)
  }
}
</script>