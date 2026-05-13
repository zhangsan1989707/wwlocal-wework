<template>
  <div class="key-management">
    <el-card>
      <template #header>
        <span>密钥列表</span>
      </template>
      <el-table :data="keys" border style="width: 100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="version" label="版本" width="120" />
        <el-table-column prop="private_key_path" label="密钥路径" />
        <el-table-column prop="is_active" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'">
              {{ row.is_active ? '已激活' : '未激活' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button
              type="primary"
              size="small"
              :disabled="row.is_active"
              @click="handleActivate(row.version)"
            >
              激活
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card style="margin-top: 16px">
      <template #header>
        <span>添加新密钥</span>
      </template>
      <el-form :model="form" label-width="120px">
        <el-form-item label="版本号">
          <el-input v-model="form.version" placeholder="如: v2" />
        </el-form-item>
        <el-form-item label="私钥内容">
          <el-input
            v-model="form.private_key_pem"
            type="textarea"
            placeholder="-----BEGIN RSA PRIVATE KEY----- ..."
            :rows="10"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleAdd">添加</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { keyAPI } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const keys = ref<any[]>([])
const form = reactive({
  version: '',
  private_key_pem: '',
})

onMounted(async () => {
  await loadKeys()
})

const loadKeys = async () => {
  try {
    const res: any = await keyAPI.list()
    if (res.code === 0) {
      keys.value = res.data
    }
  } catch (err) {
    ElMessage.error('加载密钥列表失败')
  }
}

const handleAdd = async () => {
  if (!form.version || !form.private_key_pem) {
    ElMessage.warning('请填写完整信息')
    return
  }

  try {
    const res: any = await keyAPI.add(form)
    if (res.code === 0) {
      ElMessage.success('添加成功')
      form.version = ''
      form.private_key_pem = ''
      await loadKeys()
    } else {
      ElMessage.error(res.msg || '添加失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '添加失败')
  }
}

const handleActivate = async (version: string) => {
  try {
    await ElMessageBox.confirm(
      `确定要激活密钥版本 "${version}" 吗？激活后其他版本将被停用。`,
      '确认激活',
      { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' }
    )
  } catch {
    return
  }

  try {
    const res: any = await keyAPI.activate({ version })
    if (res.code === 0) {
      ElMessage.success('激活成功')
      await loadKeys()
    } else {
      ElMessage.error(res.msg || '激活失败')
    }
  } catch (err: any) {
    ElMessage.error(err.message || '激活失败')
  }
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('zh-CN')
}
</script>