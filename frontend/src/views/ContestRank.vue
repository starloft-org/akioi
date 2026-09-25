<template>
  <div class="panel">
    <div class="rank-head">
      <h3 class="panel__title">比赛榜单</h3>
      <div class="rank-actions">
        <el-button size="small" @click="exportCsv">导出 CSV</el-button>
        <el-button size="small" type="primary" @click="load">刷新</el-button>
      </div>
    </div>

    <div v-if="frozen" class="frozen-tip">已封榜，榜单不再更新</div>

    <el-table v-loading="loading" :data="items" size="small" row-key="user_id">
      <el-table-column label="名次" width="80" prop="rank" />
      <el-table-column label="用户名" min-width="140">
        <template #default="{ row }">
          <router-link :to="`/users/${row.user_id}`">{{ row.username }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="昵称" min-width="120">
        <template #default="{ row }">{{ row.nickname || '-' }}</template>
      </el-table-column>
      <el-table-column label="总分" width="90" prop="total_score" />
      <el-table-column label="通过题数" width="100" prop="accepted_num" />
      <el-table-column
        v-for="col in problemCols"
        :key="col.problem_id"
        :label="col.label"
        width="110"
        align="center"
      >
        <template #default="{ row }">
          <div :class="{ 'rank-accepted': cellOf(row, col.problem_id).accepted }">
            {{ cellOf(row, col.problem_id).score }}
          </div>
          <div v-if="cellOf(row, col.problem_id).attempts > 0" class="muted rank-attempts">
            尝试 {{ cellOf(row, col.problem_id).attempts }} 次
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { contestRankCsvUrl, getContestRank } from '@/api'

const route = useRoute()

const loading = ref(true)
const items = ref([])
const frozen = ref(false)

const problemCols = computed(() => {
  const first = items.value[0]
  return first?.problems || []
})

const cellOf = (row, problemId) => row.problems?.find((cell) => cell.problem_id === problemId) || {}

const load = async () => {
  loading.value = true
  try {
    const data = await getContestRank(route.params.id)
    items.value = data.items || []
    frozen.value = !!data.frozen
  } catch (e) {
    items.value = []
    frozen.value = false
  } finally {
    loading.value = false
  }
}

const exportCsv = () => {
  window.open(contestRankCsvUrl(route.params.id))
}

onMounted(load)
</script>

<style scoped>
.rank-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.rank-head .panel__title {
  margin-bottom: 0;
}

.rank-actions {
  display: flex;
  gap: 8px;
}

.frozen-tip {
  margin-bottom: 12px;
  padding: 8px 12px;
  background: #fff7e6;
  border: 1px solid #ffd591;
  border-radius: var(--akioi-radius);
  color: #ad6800;
  font-size: 13px;
}

.rank-accepted {
  color: #52c41a;
  font-weight: 600;
}

.rank-attempts {
  font-size: 12px;
}
</style>
