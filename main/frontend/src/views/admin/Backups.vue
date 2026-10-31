<template>
  <div class="panel">
    <div class="toolbar">
      <el-button type="primary" :loading="creating" @click="createBackup">立即备份</el-button>
      <el-button @click="load">刷新</el-button>
      <span class="muted">备份文件保存在服务器的 {{ dir || 'BACKUP_DIR' }} 目录中</span>
    </div>

    <el-alert
      class="backup-alert"
      type="info"
      :closable="false"
      show-icon
      title="备份内容为整库结构与数据（含已软删除的记录与题目测试数据清单），恢复时用 gunzip -c 解压后导入 MySQL。"
    />

    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="name" label="文件名" min-width="280" />
      <el-table-column label="大小" width="120">
        <template #default="{ row }">{{ formatSize(row.size) }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="190">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" :loading="downloading === row.name" @click="download(row)">下载</el-button>
          <el-button size="small" @click="removeBackup(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>还没有备份文件，点击「立即备份」生成一份</template>
    </el-table>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminCreateBackup, adminDeleteBackup, adminDownloadBackup, adminListBackups, saveBlob } from '@/api'
import { formatTime } from '@/utils/format'

const loading = ref(false)
const creating = ref(false)
const downloading = ref('')
const items = ref([])
const dir = ref('')

const formatSize = (size) => {
  if (!size) {
    return '0 B'
  }
  if (size < 1024) {
    return `${size} B`
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

const load = async () => {
  loading.value = true
  try {
    const data = await adminListBackups()
    items.value = data.items || []
    dir.value = data.dir || ''
  } catch (e) {
    items.value = []
  } finally {
    loading.value = false
  }
}

const createBackup = async () => {
  creating.value = true
  try {
    await adminCreateBackup()
    ElMessage.success('备份已生成')
    await load()
  } catch (e) {
    ElMessage.error('生成备份失败')
  } finally {
    creating.value = false
  }
}

const download = async (row) => {
  downloading.value = row.name
  try {
    const blob = await adminDownloadBackup(row.name)
    saveBlob(blob, row.name)
  } catch (e) {
    ElMessage.error('下载备份失败')
  } finally {
    downloading.value = ''
  }
}

const removeBackup = async (row) => {
  try {
    await ElMessageBox.confirm(`确定删除备份 ${row.name} 吗？`, '删除备份', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await adminDeleteBackup(row.name)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error('删除备份失败')
  }
}

onMounted(load)
</script>

<style scoped>
.backup-alert {
  margin-bottom: 12px;
}
</style>
