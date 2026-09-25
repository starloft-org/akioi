<template>
  <div class="panel">
    <h3 class="panel__title">比赛</h3>
    <div class="toolbar">
      <el-select v-model="query.status" placeholder="状态" clearable style="width: 140px">
        <el-option label="未开始" value="pending" />
        <el-option label="进行中" value="running" />
        <el-option label="已结束" value="ended" />
      </el-select>
      <el-button type="primary" @click="applyFilter">筛选</el-button>
      <el-button @click="resetFilter">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column label="比赛" min-width="260">
        <template #default="{ row }">
          <div>
            <router-link :to="`/contests/${row.id}`">{{ row.title }}</router-link>
          </div>
          <div v-if="row.description" class="muted contest-desc">{{ row.description }}</div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="contestStatusType(row.status)" size="small">
            {{ contestStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="开始时间" width="170">
        <template #default="{ row }">{{ formatTime(row.start_at) }}</template>
      </el-table-column>
      <el-table-column label="结束时间" width="170">
        <template #default="{ row }">{{ formatTime(row.end_at) }}</template>
      </el-table-column>
      <el-table-column label="题目数" width="90" prop="problem_count" />
      <el-table-column label="参与人数" width="100" prop="participant_count" />
    </el-table>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, prev, pager, next, sizes"
        @current-change="load"
        @size-change="handleSizeChange"
      />
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { listContests } from '@/api'
import { contestStatusText, contestStatusType, formatTime } from '@/utils/format'

const loading = ref(true)
const items = ref([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, status: '' })

const load = async () => {
  loading.value = true
  try {
    const params = { page: query.page, page_size: query.page_size }
    if (query.status) {
      params.status = query.status
    }
    const data = await listContests(params)
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
  query.page = 1
  load()
}

onMounted(load)
</script>

<style scoped>
.contest-desc {
  margin-top: 2px;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 420px;
}
</style>
