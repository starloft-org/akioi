<template>
  <div class="panel">
    <div class="toolbar">
      <el-input v-model="filters.problem_id" placeholder="题目 ID" clearable style="width: 110px" @keyup.enter="search" />
      <el-input v-model="filters.user_id" placeholder="用户 ID" clearable style="width: 110px" @keyup.enter="search" />
      <el-input v-model="filters.contest_id" placeholder="比赛 ID" clearable style="width: 110px" @keyup.enter="search" />
      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 140px" @change="search">
        <el-option v-for="item in STATUS_OPTIONS" :key="item.value" :label="item.text" :value="item.value" />
      </el-select>
      <el-select v-model="filters.language" placeholder="语言" clearable style="width: 110px" @change="search">
        <el-option v-for="item in LANGUAGES" :key="item.key" :label="item.name" :value="item.key" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="batchVisible = true">批量重测</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="items" border>
      <el-table-column label="ID" width="90">
        <template #default="{ row }">
          <el-link type="primary" @click="openSubmission(row.id)">{{ row.id }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="用户" min-width="110">
        <template #default="{ row }">
          <el-link type="primary" @click="openUser(row.user_id)">{{ row.username }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="题目" min-width="180">
        <template #default="{ row }">{{ row.problem_display_id }} {{ row.problem_title }}</template>
      </el-table-column>
      <el-table-column label="比赛 ID" width="90">
        <template #default="{ row }">{{ row.contest_id ? row.contest_id : '-' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="score" label="得分" width="80" />
      <el-table-column label="耗时" width="90">
        <template #default="{ row }">{{ formatDuration(row.time_ms) }}</template>
      </el-table-column>
      <el-table-column label="内存" width="90">
        <template #default="{ row }">{{ formatMemory(row.memory_kb) }}</template>
      </el-table-column>
      <el-table-column label="语言" width="90">
        <template #default="{ row }">{{ languageName(row.language) }}</template>
      </el-table-column>
      <el-table-column label="提交时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="warning" plain :loading="rejudgingId === row.id" @click="rejudge(row)">
            重测
          </el-button>
          <el-button size="small" @click="openSubmission(row.id)">查看源码</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="load"
      />
    </div>

    <el-dialog v-model="batchVisible" title="批量重测" width="440px">
      <el-radio-group v-model="batchMode">
        <el-radio value="problem">按题目 ID</el-radio>
        <el-radio value="contest">按比赛 ID</el-radio>
        <el-radio value="ids">按提交 ID 列表</el-radio>
      </el-radio-group>
      <el-input v-model="batchValue" :placeholder="batchPlaceholder" style="margin-top: 12px" />
      <template #footer>
        <el-button @click="batchVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchLoading" @click="submitBatch">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminBatchRejudge, adminListSubmissions, adminRejudgeSubmission } from '@/api'
import {
  JUDGE_STATUS,
  LANGUAGES,
  formatDuration,
  formatMemory,
  formatTime,
  languageName,
  statusText,
  statusType
} from '@/utils/format'

const STATUS_OPTIONS = Object.keys(JUDGE_STATUS).map((key) => ({ value: key, text: JUDGE_STATUS[key].text }))

const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filters = reactive({
  problem_id: '',
  user_id: '',
  contest_id: '',
  status: '',
  language: ''
})

const batchVisible = ref(false)
const batchLoading = ref(false)
const rejudgingId = ref(null)
const batchMode = ref('problem')
const batchValue = ref('')

const batchPlaceholder = computed(() =>
  batchMode.value === 'ids' ? '提交 ID，逗号或空格分隔，如 1,2,3' : '请输入数字 ID'
)

// 仅接受正整数，其余情况不参与筛选
const toId = (value) => {
  const num = Number(value)
  return Number.isInteger(num) && num > 0 ? num : undefined
}

const load = async () => {
  loading.value = true
  try {
    const data = await adminListSubmissions({
      page: page.value,
      page_size: pageSize.value,
      problem_id: toId(filters.problem_id),
      user_id: toId(filters.user_id),
      contest_id: toId(filters.contest_id),
      status: filters.status || undefined,
      language: filters.language || undefined
    })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const search = () => {
  page.value = 1
  load()
}

const openSubmission = (id) => {
  window.open(`/submissions/${id}`)
}

const openUser = (id) => {
  window.open(`/users/${id}`)
}

const rejudge = async (row) => {
  try {
    await ElMessageBox.confirm(
      `将重置提交 #${row.id} 的评测结果并重新投递测评，确定继续？`,
      '重新测评',
      { type: 'warning', confirmButtonText: '重新测评', cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }
  rejudgingId.value = row.id
  try {
    await adminRejudgeSubmission(row.id)
    ElMessage.success('已提交重测任务')
    load()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    rejudgingId.value = null
  }
}

const submitBatch = async () => {
  const text = batchValue.value.trim()
  if (!text) {
    ElMessage.error('请输入内容')
    return
  }
  let payload
  if (batchMode.value === 'ids') {
    const ids = text
      .split(/[\s,，]+/)
      .filter(Boolean)
      .map(Number)
      .filter((num) => Number.isInteger(num) && num > 0)
    if (!ids.length) {
      ElMessage.error('提交 ID 需为逗号或空格分隔的正整数')
      return
    }
    payload = { ids }
  } else {
    const num = toId(text)
    if (!num) {
      ElMessage.error('请输入有效的数字 ID')
      return
    }
    payload = batchMode.value === 'problem' ? { problem_id: num } : { contest_id: num }
  }
  batchLoading.value = true
  try {
    await adminBatchRejudge(payload)
    ElMessage.success('已提交重测任务')
    batchVisible.value = false
    batchValue.value = ''
    load()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    batchLoading.value = false
  }
}

onMounted(load)
</script>
