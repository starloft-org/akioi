<template>
  <div class="panel">
    <div class="panel-head">
      <h3 class="panel__title">比赛评测记录</h3>
      <el-button size="small" @click="router.push(`/contests/${route.params.id}`)">返回比赛</el-button>
    </div>

    <template v-if="store.isLogin">
      <el-table v-loading="loading" :data="items" size="small">
        <el-table-column label="记录 ID" width="100">
          <template #default="{ row }">
            <router-link class="mono" :to="`/submissions/${row.id}`">#{{ row.id }}</router-link>
          </template>
        </el-table-column>
        <el-table-column label="用户" min-width="140">
          <template #default="{ row }">
            <router-link :to="`/users/${row.user_id}`">{{ row.nickname || row.username }}</router-link>
          </template>
        </el-table-column>
        <el-table-column label="题目" min-width="220">
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
    </template>

    <div v-else class="login-tip">
      <span class="muted">登录后可查看比赛评测记录</span>
      <el-button type="primary" @click="goLogin">登录</el-button>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listContestSubmissions } from '@/api'
import StatusTag from '@/components/StatusTag.vue'
import { useUserStore } from '@/store/user'
import { formatDuration, formatMemory, formatTime, languageName } from '@/utils/format'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const items = ref([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20 })

const goLogin = () => {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

const load = async () => {
  loading.value = true
  try {
    const data = await listContestSubmissions(route.params.id, {
      page: query.page,
      page_size: query.page_size
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

const handleSizeChange = () => {
  query.page = 1
  load()
}

onMounted(() => {
  if (store.isLogin) {
    load()
  }
})
</script>

<style scoped>
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.panel-head .panel__title {
  margin-bottom: 0;
}

.login-tip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 0;
}
</style>
