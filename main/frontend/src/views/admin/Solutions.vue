<template>
  <div class="panel">
    <div class="toolbar toolbar-between">
      <div>
        <h3 class="panel__title">题解审核</h3>
        <p class="panel__hint">用户发布的题解默认待审核，通过后其他用户才能看到；作者修改已通过的题解会重新进入待审核。</p>
      </div>
      <div class="toolbar">
        <el-radio-group v-model="status" @change="search">
          <el-radio-button value="pending">待审核</el-radio-button>
          <el-radio-button value="approved">已通过</el-radio-button>
          <el-radio-button value="rejected">已拒绝</el-radio-button>
          <el-radio-button value="">全部</el-radio-button>
        </el-radio-group>
        <el-input
          v-model="keyword"
          placeholder="题解标题"
          clearable
          style="width: 200px"
          @keyup.enter="search"
        />
        <el-button @click="search">筛选</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column label="题目" min-width="220">
        <template #default="{ row }">
          <router-link v-if="row.problem_id" :to="`/problems/${row.problem_id}`" class="problem-link">
            {{ row.problem_display_id }} {{ row.problem_title }}
          </router-link>
          <span v-else class="muted">题目已删除</span>
        </template>
      </el-table-column>
      <el-table-column label="题解标题" min-width="200">
        <template #default="{ row }">
          <el-link type="primary" :underline="false" @click="openPreview(row)">{{ row.title }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="作者" width="140">
        <template #default="{ row }">{{ row.nickname || row.username }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="提交时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="210">
        <template #default="{ row }">
          <el-button link type="primary" @click="openPreview(row)">查看</el-button>
          <el-button link type="success" :disabled="row.status === 'approved'" @click="approve(row)">通过</el-button>
          <el-button link type="danger" :disabled="row.status === 'rejected'" @click="openReject(row)">驳回</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-if="total > pageSize"
      class="pager"
      layout="prev, pager, next, total"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="onPageChange"
    />

    <el-dialog v-model="previewVisible" :title="preview?.title || '题解内容'" width="760px">
      <template v-if="preview">
        <p class="muted">
          <router-link v-if="preview.problem_id" :to="`/problems/${preview.problem_id}`">
            {{ preview.problem_display_id }} {{ preview.problem_title }}
          </router-link>
          · 作者 {{ preview.nickname || preview.username }} · {{ statusText(preview.status) }}
        </p>
        <div class="pre-block preview-content">{{ preview.content }}</div>
      </template>
      <template #footer>
        <el-button @click="previewVisible = false">关闭</el-button>
        <el-button v-if="preview && preview.status !== 'rejected'" type="danger" plain @click="openReject(preview)">驳回</el-button>
        <el-button v-if="preview && preview.status !== 'approved'" type="success" @click="approve(preview)">通过</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rejectVisible" title="驳回题解" width="520px">
      <p class="muted">驳回后作者能看到说明，可以修改后重新提交审核。</p>
      <el-input v-model="rejectNote" type="textarea" :rows="3" maxlength="255" show-word-limit placeholder="驳回原因（可选，作者可见）" />
      <template #footer>
        <el-button @click="rejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="saving" @click="reject">确认驳回</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminListSolutions, adminReviewSolution } from '@/api'
import { formatTime } from '@/utils/format'

const loading = ref(false)
const saving = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const status = ref('pending')
const keyword = ref('')
const previewVisible = ref(false)
const preview = ref(null)
const rejectVisible = ref(false)
const rejectTarget = ref(null)
const rejectNote = ref('')

const statusText = (value) => ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' }[value] || value)
const statusTagType = (value) => (value === 'approved' ? 'success' : value === 'rejected' ? 'danger' : 'warning')

async function load() {
  loading.value = true
  try {
    const data = await adminListSolutions({
      page: page.value,
      page_size: pageSize,
      status: status.value || undefined,
      keyword: keyword.value || undefined
    })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '加载题解失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function onPageChange(value) {
  page.value = value
  load()
}

function openPreview(row) {
  preview.value = row
  previewVisible.value = true
}

async function review(row, nextStatus, note = '') {
  saving.value = true
  try {
    await adminReviewSolution(row.id, { status: nextStatus, note })
    ElMessage.success(nextStatus === 'approved' ? '题解已通过' : '题解已驳回')
    previewVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '审核失败')
  } finally {
    saving.value = false
  }
}

function approve(row) {
  review(row, 'approved')
}

function openReject(row) {
  rejectTarget.value = row
  rejectNote.value = row.review_note || ''
  rejectVisible.value = true
}

async function reject() {
  if (!rejectTarget.value) return
  await review(rejectTarget.value, 'rejected', rejectNote.value)
  rejectVisible.value = false
}

onMounted(load)
</script>

<style scoped>
.toolbar-between {
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px;
}
.preview-content {
  margin-top: 12px;
  max-height: 60vh;
  overflow: auto;
}
.pager {
  margin-top: 12px;
  justify-content: flex-end;
}
</style>
