<template>
  <div class="log-query">
    <el-card>
      <template #header>
        <span>查询条件</span>
      </template>
      <el-form :model="form" label-width="120px">
        <el-form-item label="数据类型">
          <el-select v-model="form.feature_ids" multiple placeholder="请选择" style="width: 100%">
            <el-option
              v-for="item in features"
              :key="item.id"
              :label="`${item.id} - ${item.name}`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
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
        <el-form-item label="动态条件">
          <el-input v-model="conditionStr" type="textarea" placeholder="JSON格式，如: {&quot;openid&quot;: &quot;xxx&quot;}" :rows="2" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleQuery" :loading="loading">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top: 16px">
      <template #header>
        <span>查询结果</span>
      </template>
      <el-table :data="tableData" border style="width: 100%" v-loading="loading" max-height="500">
        <el-table-column prop="feature_id" label="FeatureID" width="100" />
        <el-table-column prop="log_date" label="时间" width="180" />
        <el-table-column prop="idc" label="IDC" width="100" />
        <el-table-column label="数据内容">
          <template #default="{ row }">
            <pre style="max-width: 600px; overflow-x: auto; font-size: 12px">{{ formatData(row) }}</pre>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-if="total > 0"
        style="margin-top: 16px; justify-content: center"
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 50, 100, 500]"
        layout="total, sizes, prev, pager, next"
        @current-change="handleQuery"
        @size-change="handleQuery"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { logAPI } from '../api'
import { ElMessage } from 'element-plus'

const form = reactive({
  feature_ids: [] as number[],
  start_time: 0,
  end_time: 0,
  conditions: null as any,
})

const dateRange = ref<[Date, Date] | null>(null)
const conditionStr = ref('')
const loading = ref(false)
const tableData = ref<any[]>([])
const total = ref(0)
const pagination = reactive({
  page: 1,
  page_size: 50,
})
const features = ref<any[]>([])

onMounted(async () => {
  try {
    const res: any = await logAPI.getFeatures()
    if (res.code === 0) {
      features.value = res.data
    }
    const timeRes: any = await logAPI.getTimeRange()
    if (timeRes.code === 0) {
      dateRange.value = [
        new Date(timeRes.data.start_time * 1000),
        new Date(timeRes.data.end_time * 1000),
      ]
    }
  } catch (err) {
    ElMessage.error('加载数据失败')
  }
})

const handleQuery = async () => {
  if (form.feature_ids.length === 0) {
    ElMessage.warning('请选择至少一个数据类型')
    return
  }
  if (!dateRange.value) {
    ElMessage.warning('请选择时间范围')
    return
  }

  loading.value = true
  try {
    form.start_time = Math.floor(dateRange.value[0].getTime() / 1000)
    form.end_time = Math.floor(dateRange.value[1].getTime() / 1000)

    if (conditionStr.value) {
      try {
        form.conditions = JSON.parse(conditionStr.value)
      } catch {
        ElMessage.error('条件JSON格式错误')
        loading.value = false
        return
      }
    }

    const res: any = await logAPI.query({
      ...form,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    if (res.code === 0) {
      tableData.value = res.data.data
      total.value = res.data.total
    } else {
      ElMessage.error(res.msg || '查询失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '查询失败')
  } finally {
    loading.value = false
  }
}

const handleReset = () => {
  form.feature_ids = []
  form.conditions = null
  conditionStr.value = ''
  tableData.value = []
  total.value = 0
  pagination.page = 1
  pagination.page_size = 50
}

const formatData = (row: any) => {
  const { feature_id, log_date, idc, ...rest } = row
  return JSON.stringify(rest, null, 2)
}
</script>