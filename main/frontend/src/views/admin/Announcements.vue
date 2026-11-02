<template>
  <div class="panel">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新建公告</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="title" label="标题" min-width="240" />
      <el-table-column label="可见" width="90">
        <template #default="{ row }">
          <el-tag :type="row.visible ? 'success' : 'info'">{{ row.visible ? '可见' : '隐藏' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="更新时间" width="170">
        <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="removeAnnouncement(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="load"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑公告' : '新建公告'" width="920px">
      <el-form :model="form" label-width="70px">
        <el-form-item label="标题">
          <el-input v-model="form.title" placeholder="公告标题" />
        </el-form-item>
        <el-form-item label="内容">
          <MarkdownEditor v-model="form.content" :rows="12" placeholder="公告正文，支持 Markdown 与 LaTeX 公式，可直接粘贴或上传图片" />
        </el-form-item>
        <el-form-item label="可见">
          <el-switch v-model="form.visible" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  adminCreateAnnouncement,
  adminDeleteAnnouncement,
  adminListAnnouncements,
  adminUpdateAnnouncement
} from '@/api'
import { formatTime } from '@/utils/format'
import MarkdownEditor from '@/components/MarkdownEditor.vue'

const loading = ref(false)
const saving = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const dialogVisible = ref(false)
const editingId = ref(null)

const form = reactive({
  title: '',
  content: '',
  visible: true
})

const load = async () => {
  loading.value = true
  try {
    const data = await adminListAnnouncements({ page: page.value, page_size: pageSize.value })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editingId.value = null
  form.title = ''
  form.content = ''
  form.visible = true
  dialogVisible.value = true
}

const openEdit = (row) => {
  editingId.value = row.id
  form.title = row.title || ''
  form.content = row.content || ''
  form.visible = !!row.visible
  dialogVisible.value = true
}

const save = async () => {
  if (!form.title.trim()) {
    ElMessage.error('请填写标题')
    return
  }
  const payload = { title: form.title.trim(), content: form.content, visible: form.visible }
  saving.value = true
  try {
    if (editingId.value) {
      await adminUpdateAnnouncement(editingId.value, payload)
    } else {
      await adminCreateAnnouncement(payload)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    load()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    saving.value = false
  }
}

const removeAnnouncement = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除公告「${row.title}」？`, '删除确认', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await adminDeleteAnnouncement(row.id)
  } catch (e) {
    return
  }
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
