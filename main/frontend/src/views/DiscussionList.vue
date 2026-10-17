<template>
  <div class="panel">
    <div class="toolbar toolbar-between">
      <div>
        <h3 class="panel__title">讨论区</h3>
        <span v-if="problemId" class="muted">当前仅显示题目 #{{ problemId }} 的讨论</span>
      </div>
      <el-button type="primary" @click="openCreate">发起讨论</el-button>
    </div>
    <el-empty v-if="!loading && items.length === 0" description="暂无讨论" />
    <el-table v-else v-loading="loading" :data="items">
      <el-table-column label="主题" min-width="300">
        <template #default="{ row }">
          <router-link :to="`/discussions/${row.id}`">{{ row.title }}</router-link>
          <el-tag v-if="row.problem_id" size="small" type="info" style="margin-left: 8px">{{ row.problem_display_id }} {{ row.problem_title }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="作者" width="160"><template #default="{ row }">{{ row.nickname || row.username }}</template></el-table-column>
      <el-table-column prop="comment_count" label="评论" width="90" />
      <el-table-column label="更新时间" width="180"><template #default="{ row }">{{ formatTime(row.updated_at) }}</template></el-table-column>
    </el-table>
    <div class="pagination-bar"><el-pagination v-model:current-page="page" :page-size="20" :total="total" layout="total, prev, pager, next" @current-change="load" /></div>

    <el-dialog v-model="dialogVisible" title="发起讨论" width="680px">
      <el-input v-model="form.title" maxlength="128" show-word-limit placeholder="讨论标题" />
      <el-input-number v-model="form.problem_id" :min="0" :step="1" placeholder="关联题目 ID（可选）" style="margin-top: 12px" />
      <el-input v-model="form.content" type="textarea" :rows="12" maxlength="50000" show-word-limit placeholder="讨论内容" style="margin-top: 12px" />
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="submitting" @click="submit">发布</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createDiscussion, listDiscussions } from '@/api'
import { useUserStore } from '@/store/user'
import { formatTime } from '@/utils/format'
const route = useRoute(), router = useRouter(), store = useUserStore()
const problemId = computed(() => Number(route.query.problem_id || 0))
const loading = ref(false), items = ref([]), total = ref(0), page = ref(1), dialogVisible = ref(false), submitting = ref(false)
const form = reactive({ title: '', content: '', problem_id: 0 })
const load = async () => {
  loading.value = true
  try { const data = await listDiscussions({ page: page.value, page_size: 20, problem_id: problemId.value || undefined }); items.value = data.items || []; total.value = data.total || 0 } finally { loading.value = false }
}
const openCreate = () => {
  if (!store.isLogin) return router.push({ path: '/login', query: { redirect: route.fullPath } })
  form.title = ''; form.content = ''; form.problem_id = problemId.value || 0; dialogVisible.value = true
}
const submit = async () => {
  if (!form.title.trim() || !form.content.trim()) return ElMessage.warning('请填写标题和内容')
  submitting.value = true
  try { const data = await createDiscussion(form); dialogVisible.value = false; router.push(`/discussions/${data.id}`) } finally { submitting.value = false }
}
onMounted(load)
</script>

<style scoped>.toolbar-between { justify-content: space-between; align-items: flex-start; }</style>
