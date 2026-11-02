<template>
  <div v-loading="loading" class="panel">
    <template v-if="solution">
      <div class="toolbar toolbar-between">
        <div>
          <h2>{{ solution.title }}</h2>
          <div class="muted">作者 {{ solution.nickname || solution.username }} · {{ formatTime(solution.updated_at) }}</div>
        </div>
        <div v-if="solution.can_edit">
          <el-button @click="editing = !editing">{{ editing ? '取消编辑' : '编辑' }}</el-button>
          <el-button type="danger" plain @click="remove">删除</el-button>
        </div>
      </div>
      <el-alert
        v-if="solution.status === 'pending'"
        type="warning"
        :closable="false"
        title="审核中：管理员通过后其他用户才能看到这篇题解"
        style="margin-top: 12px"
      />
      <el-alert
        v-else-if="solution.status === 'rejected'"
        type="error"
        :closable="false"
        :title="`未通过审核${solution.review_note ? '：' + solution.review_note : ''}`"
        style="margin-top: 12px"
      />
      <template v-if="editing">
        <el-input v-model="form.title" maxlength="128" />
        <el-input v-model="form.content" type="textarea" :rows="16" maxlength="50000" show-word-limit style="margin-top: 12px" />
        <el-button type="primary" :loading="saving" style="margin-top: 12px" @click="save">保存</el-button>
      </template>
      <div v-else class="pre-block community-content">{{ solution.content }}</div>
      <div style="margin-top: 16px"><router-link :to="`/problems/${solution.problem_id}/solutions`">返回题解列表</router-link></div>
    </template>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteSolution, getSolution, updateSolution } from '@/api'
import { formatTime } from '@/utils/format'
const route = useRoute(), router = useRouter()
const solution = ref(null), loading = ref(false), editing = ref(false), saving = ref(false)
const form = reactive({ title: '', content: '' })
const load = async () => {
  loading.value = true
  try { const data = await getSolution(route.params.id); solution.value = data.solution; form.title = data.solution.title; form.content = data.solution.content } finally { loading.value = false }
}
const save = async () => {
  if (!form.title.trim() || !form.content.trim()) return ElMessage.warning('请填写标题和内容')
  saving.value = true
  try { await updateSolution(solution.value.id, form); ElMessage.success('已保存，修改后的题解会重新进入审核'); editing.value = false; await load() } finally { saving.value = false }
}
const remove = async () => {
  try { await ElMessageBox.confirm('确认删除这篇题解？', '删除确认', { type: 'warning' }); await deleteSolution(solution.value.id); router.replace(`/problems/${solution.value.problem_id}/solutions`) } catch (e) { /* 用户取消或请求失败 */ }
}
onMounted(load)
</script>

<style scoped>.toolbar-between { justify-content: space-between; align-items: flex-start; }.community-content { margin-top: 20px; min-height: 160px; }</style>
