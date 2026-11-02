<template>
  <div class="panel">
    <h3 class="panel__title">我的题单</h3>
    <p class="panel__hint">
      这里只显示你自己创建的个人题单，只有自己能看；团队题单请在团队页面查看。题单本身不公开。
    </p>

    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新建题单</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column label="标题" min-width="220">
        <template #default="{ row }">
          <router-link class="problem-link" :to="`/problem-sets/personal/${row.id}`">{{ row.title }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="题目数" width="90" align="right">
        <template #default="{ row }">
          <span class="tabular">{{ row.problem_count }}</span>
        </template>
      </el-table-column>
      <el-table-column label="说明" min-width="240">
        <template #default="{ row }">
          <span class="muted">{{ row.description || '暂无说明' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">查看</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <el-dialog v-model="dialog" title="新建题单" width="540px">
    <el-form label-width="90px">
      <el-form-item label="标题">
        <el-input v-model="form.title" maxlength="128" />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="3" maxlength="255" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import { createMyProblemSet, deleteMyProblemSet, listMyProblemSets } from '@/api'

const router = useRouter()
const loading = ref(false)
const items = ref([])
const dialog = ref(false)
const saving = ref(false)
const form = ref({ title: '', description: '' })

function openDetail(row) {
  router.push(`/problem-sets/personal/${row.id}`)
}

async function load() {
  loading.value = true
  try {
    const data = await listMyProblemSets()
    items.value = data.items || []
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '加载题单失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { title: '', description: '' }
  dialog.value = true
}

async function save() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写题单标题')
    return
  }
  saving.value = true
  try {
    const data = await createMyProblemSet({
      title: form.value.title,
      description: form.value.description,
      problem_ids: []
    })
    dialog.value = false
    ElMessage.success('题单已创建')
    const id = data.set?.id
    if (id) {
      router.push(`/problem-sets/personal/${id}`)
    } else {
      load()
    }
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '创建题单失败')
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除题单「${row.title}」？`, '删除题单', { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteMyProblemSet(row.id)
    ElMessage.success('题单已删除')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '删除题单失败')
  }
}

onMounted(load)
</script>
