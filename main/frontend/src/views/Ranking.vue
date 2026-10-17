<template>
  <div class="panel">
    <h3 class="panel__title">全站积分榜</h3>
    <p class="muted">每题积分按该题历史最高评测得分比例折算，满分题计入通过数。</p>
    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column prop="rank" label="排名" width="90" />
      <el-table-column label="用户" min-width="220">
        <template #default="{ row }">
          <router-link :to="`/users/${row.user_id}`">{{ row.nickname || row.username }}</router-link>
          <span class="muted">（{{ row.username }}）</span>
        </template>
      </el-table-column>
      <el-table-column label="积分" width="140">
        <template #default="{ row }"><strong>{{ Number(row.points || 0).toFixed(2) }}</strong></template>
      </el-table-column>
      <el-table-column prop="solved_count" label="满分题数" width="120" />
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
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { listRankings } from '@/api'

const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const load = async () => {
  loading.value = true
  try {
    const data = await listRankings({ page: page.value, page_size: pageSize.value })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
