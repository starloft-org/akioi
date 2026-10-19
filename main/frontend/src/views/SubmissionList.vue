<template>
  <div class="panel">
    <h3 class="panel__title">评测记录</h3>

    <div class="toolbar">
      <el-select v-model="query.status" placeholder="状态" clearable style="width: 160px">
        <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.language" placeholder="语言" clearable style="width: 130px">
        <el-option v-for="item in LANGUAGES" :key="item.key" :label="item.name" :value="item.key" />
      </el-select>
      <el-checkbox v-model="mineOnly" label="只看我的" @change="applyFilter" />
      <el-button type="primary" @click="applyFilter">筛选</el-button>
      <el-button @click="resetFilter">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column label="记录 ID" width="100">
        <template #default="{ row }">
          <router-link class="mono" :to="`/submissions/${row.id}`">#{{ row.id }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="用户" min-width="120">
        <template #default="{ row }">
          <router-link :to="`/users/${row.user_id}`">{{ row.nickname || row.username }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="题目" min-width="200">
        <template #default="{ row }">
          <router-link :to="`/problems/${row.problem_id}`">
            {{ row.problem_display_id }} {{ row.problem_title }}
          </router-link>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="140">
        <template #default="{ row }">
          <StatusTag :status="row.status" />
        </template>
      </el-table-column>
      <el-table-column label="得分" width="80" prop="score" />
      <el-table-column label="耗时" width="100">
        <template #default="{ row }">{{ formatDuration(row.time_ms) }}</template>
      </el-table-column>
      <el-table-column label="内存" width="100">
        <template #default="{ row }">{{ formatMemory(row.memory_kb) }}</template>
      </el-table-column>
      <el-table-column label="语言" width="90">
        <template #default="{ row }">{{ languageName(row.language) }}</template>
      </el-table-column>
      <el-table-column label="提交时间" min-width="160">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
    </el-table>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, prev, pager, next, sizes"
        @current-change="load"
        @size-change="handleSizeChange"
      />
    </div>

    <div v-if="!store.isLogin" class="login-tip">
      <span class="muted">查看评测记录前请先登录</span>
      <el-button type="primary" @click="goLogin">登录</el-button>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { listSubmissions } from '@/api'
import StatusTag from '@/components/StatusTag.vue'
import { useUserStore } from '@/store/user'
import { LANGUAGES, formatDuration, formatMemory, formatTime, languageName } from '@/utils/format'

const store = useUserStore()

const loading = ref(false)
const items = ref([])
const total = ref(0)
const mineOnly = ref(false)
const query = reactive({ page: 1, page_size: 20, status: '', language: '' })

const statusOptions = [
  { label: '等待评测', value: 'Pending' },
  { label: '评测中', value: 'Judging' },
  { label: '答案正确', value: 'Accepted' },
  { label: '答案错误', value: 'WrongAnswer' },
  { label: '超出时间限制', value: 'TimeLimitExceeded' },
  { label: '超出内存限制', value: 'MemoryLimitExceeded' },
  { label: '运行时错误', value: 'RuntimeError' },
  { label: '编译错误', value: 'CompileError' },
  { label: '系统错误', value: 'SystemError' },
  { label: '输出超限', value: 'OutputLimitExceeded' }
]

const load = async () => {
  loading.value = true
  try {
    const params = { page: query.page, page_size: query.page_size }
    if (query.status) {
      params.status = query.status
    }
    if (query.language) {
      params.language = query.language
    }
    if (mineOnly.value && store.isLogin) {
      params.mine = 1
    }
    const data = await listSubmissions(params)
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const applyFilter = () => {
  query.page = 1
  load()
}

const handleSizeChange = () => {
  query.page = 1
  load()
}

const resetFilter = () => {
  query.status = ''
  query.language = ''
  mineOnly.value = false
  query.page = 1
  load()
}

onMounted(() => {
  // 未登录时不发请求，页面底部展示登录引导
  if (store.isLogin) {
    load()
  }
})
</script>

<style scoped>
.login-tip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 0;
}
</style>
