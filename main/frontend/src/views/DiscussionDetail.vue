<template>
  <div v-loading="loading">
    <div v-if="discussion" class="panel">
      <div class="toolbar toolbar-between">
        <div>
          <h2>{{ discussion.title }}</h2>
          <div class="muted">{{ discussion.nickname || discussion.username }} · {{ formatTime(discussion.created_at) }}</div>
          <router-link v-if="discussion.problem_id" :to="`/problems/${discussion.problem_id}`">{{ discussion.problem_display_id }} {{ discussion.problem_title }}</router-link>
        </div>
        <div v-if="discussion.can_edit"><el-button @click="startEdit">编辑</el-button><el-button type="danger" plain @click="removeDiscussion">删除</el-button></div>
      </div>
      <template v-if="editing">
        <el-input v-model="editForm.title" maxlength="128" />
        <el-input v-model="editForm.content" type="textarea" :rows="12" maxlength="50000" show-word-limit style="margin-top: 12px" />
        <div style="margin-top: 12px"><el-button type="primary" :loading="saving" @click="saveDiscussion">保存</el-button><el-button @click="editing = false">取消</el-button></div>
      </template>
      <div v-else class="pre-block community-content">{{ discussion.content }}</div>
    </div>

    <div v-if="discussion" class="panel">
      <h3 class="panel__title">评论（{{ comments.length }}）</h3>
      <el-empty v-if="comments.length === 0" description="暂无评论" />
      <div v-for="item in comments" :key="item.id" class="comment-item">
        <div class="comment-head"><span>{{ item.nickname || item.username }}</span><span class="muted">{{ formatTime(item.created_at) }}</span></div>
        <el-input v-if="editingComment === item.id" v-model="commentEditContent" type="textarea" :rows="3" />
        <div v-else class="pre-block">{{ item.content }}</div>
        <div v-if="item.can_edit" class="comment-actions">
          <template v-if="editingComment === item.id"><el-button size="small" type="primary" @click="saveComment(item)">保存</el-button><el-button size="small" @click="editingComment = 0">取消</el-button></template>
          <template v-else><el-button size="small" text @click="beginCommentEdit(item)">编辑</el-button><el-button size="small" text type="danger" @click="removeComment(item)">删除</el-button></template>
        </div>
      </div>
      <div class="comment-compose">
        <template v-if="!commentsOpen"><span class="muted">评论区已关闭，暂时无法发表评论</span></template>
        <template v-else-if="store.isLogin"><el-input v-model="newComment" type="textarea" :rows="4" maxlength="5000" show-word-limit placeholder="发表友善的评论" /><el-button type="primary" :loading="commenting" style="margin-top: 8px" @click="submitComment">发表评论</el-button></template>
        <el-button v-else type="primary" @click="router.push({ path: '/login', query: { redirect: route.fullPath } })">登录后评论</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createComment, deleteComment, deleteDiscussion, getDiscussion, updateComment, updateDiscussion } from '@/api'
import { useUserStore } from '@/store/user'
import { formatTime } from '@/utils/format'
const route = useRoute(), router = useRouter(), store = useUserStore()
// 站点信息未加载或后端未下发时按开启处理，避免误显「已关闭」
const commentsOpen = computed(() => store.site?.comments_enabled !== false)
const loading = ref(false), discussion = ref(null), comments = ref([]), editing = ref(false), saving = ref(false)
const newComment = ref(''), commenting = ref(false), editingComment = ref(0), commentEditContent = ref('')
const editForm = reactive({ title: '', content: '' })
const load = async () => { loading.value = true; try { const data = await getDiscussion(route.params.id); discussion.value = data.discussion; comments.value = data.comments || [] } finally { loading.value = false } }
const startEdit = () => { editForm.title = discussion.value.title; editForm.content = discussion.value.content; editing.value = true }
const saveDiscussion = async () => {
  if (!editForm.title.trim() || !editForm.content.trim()) return ElMessage.warning('请填写标题和内容')
  saving.value = true; try { await updateDiscussion(discussion.value.id, editForm); editing.value = false; ElMessage.success('已保存'); await load() } finally { saving.value = false }
}
const removeDiscussion = async () => { try { await ElMessageBox.confirm('确认删除该讨论及全部评论？', '删除确认', { type: 'warning' }); await deleteDiscussion(discussion.value.id); router.replace('/discussions') } catch (e) { /* 取消或失败 */ } }
const submitComment = async () => {
  if (!newComment.value.trim()) return ElMessage.warning('评论不能为空')
  commenting.value = true; try { await createComment(discussion.value.id, { content: newComment.value }); newComment.value = ''; await load() } finally { commenting.value = false }
}
const beginCommentEdit = (item) => { editingComment.value = item.id; commentEditContent.value = item.content }
const saveComment = async (item) => { if (!commentEditContent.value.trim()) return; await updateComment(item.id, { content: commentEditContent.value }); editingComment.value = 0; await load() }
const removeComment = async (item) => { try { await ElMessageBox.confirm('确认删除这条评论？', '删除确认'); await deleteComment(item.id); await load() } catch (e) { /* 取消或失败 */ } }
onMounted(load)
</script>

<style scoped>
.toolbar-between { justify-content: space-between; align-items: flex-start; }
.community-content { margin-top: 20px; min-height: 120px; }
.comment-item { padding: 14px 0; border-bottom: 1px solid var(--akioi-border); }
.comment-head { display: flex; gap: 12px; margin-bottom: 8px; font-size: 13px; }
.comment-actions { margin-top: 6px; text-align: right; }
.comment-compose { margin-top: 20px; }
</style>
