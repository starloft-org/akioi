<template>
  <div>
    <div class="panel">
      <div class="toolbar toolbar-between">
        <div>
          <h3 class="panel__title">{{ problem ? `${problem.display_id} ${problem.title}` : '' }} 题解</h3>
          <router-link :to="`/problems/${problemId}`">返回题目</router-link>
        </div>
        <el-button type="primary" @click="openCreate">发布题解</el-button>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="暂无题解" />
      <el-table v-else v-loading="loading" :data="items">
        <el-table-column label="标题" min-width="260">
          <template #default="{ row }"><router-link :to="`/solutions/${row.id}`">{{ row.title }}</router-link></template>
        </el-table-column>
        <el-table-column label="作者" width="180">
          <template #default="{ row }"><router-link :to="`/users/${row.user_id}`">{{ row.nickname || row.username }}</router-link></template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" title="发布题解" width="680px">
      <el-input v-model="form.title" maxlength="128" show-word-limit placeholder="题解标题" />
      <el-input v-model="form.content" type="textarea" :rows="14" maxlength="50000" show-word-limit placeholder="写下你的思路、证明与复杂度分析" style="margin-top: 12px" />
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="submitting" @click="submit">发布</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createSolution, getProblem, listSolutions } from '@/api'
import { useUserStore } from '@/store/user'
import { formatTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const store = useUserStore()
const problemId = computed(() => Number(route.params.id))
const problem = ref(null)
const items = ref([])
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const form = reactive({ title: '', content: '' })

const load = async () => {
  loading.value = true
  try {
    const [problemData, listData] = await Promise.all([getProblem(problemId.value), listSolutions(problemId.value, { page_size: 100 })])
    problem.value = problemData.problem
    items.value = listData.items || []
  } finally {
    loading.value = false
  }
}
const openCreate = () => {
  if (!store.isLogin) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  form.title = ''
  form.content = ''
  dialogVisible.value = true
}
const submit = async () => {
  if (!form.title.trim() || !form.content.trim()) return ElMessage.warning('请填写标题和内容')
  submitting.value = true
  try {
    const data = await createSolution(problemId.value, form)
    dialogVisible.value = false
    router.push(`/solutions/${data.id}`)
  } finally {
    submitting.value = false
  }
}
onMounted(load)
</script>

<style scoped>.toolbar-between { justify-content: space-between; align-items: flex-start; }</style>
