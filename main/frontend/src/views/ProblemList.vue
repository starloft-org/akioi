<template>
  <div class="panel">
    <h3 class="panel__title">题库</h3>
    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        placeholder="题号或标题"
        clearable
        style="width: 200px"
        @keyup.enter="applyFilter"
      />
      <el-select v-model="query.difficulty" placeholder="难度" clearable style="width: 130px">
        <el-option v-for="item in DIFFICULTIES" :key="item" :label="item" :value="item" />
      </el-select>
      <el-input
        v-model="query.tag"
        placeholder="标签"
        clearable
        style="width: 140px"
        @keyup.enter="applyFilter"
      />
      <el-input
        v-model="query.source"
        placeholder="来源"
        clearable
        style="width: 160px"
        @keyup.enter="applyFilter"
      />
      <el-select v-model="query.status" placeholder="我的状态" clearable style="width: 140px">
        <el-option label="未通过" value="unsolved" />
        <el-option label="尝试过" value="attempted" />
        <el-option label="已通过" value="solved" />
      </el-select>
      <el-button type="primary" @click="applyFilter">筛选</el-button>
      <el-button @click="resetFilter">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column label="题号" width="110">
        <template #default="{ row }">
          <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="标题" min-width="220">
        <template #default="{ row }">
          <router-link :to="`/problems/${row.id}`">{{ row.title }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="难度" width="100">
        <template #default="{ row }">
          <el-tag v-if="row.difficulty" size="small">{{ row.difficulty }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="points" label="积分" width="80" />
      <el-table-column label="标签" min-width="180">
        <template #default="{ row }">
          <el-tag
            v-for="tag in row.tags"
            :key="tag"
            class="tag-click"
            type="info"
            size="small"
            @click="filterByTag(tag)"
          >
            {{ tag }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="通过率" width="100">
        <template #default="{ row }">{{ rateText(row.acceptance_rate) }}</template>
      </el-table-column>
      <el-table-column label="提交/通过" width="120">
        <template #default="{ row }">{{ row.submit_count }} / {{ row.accepted_count }}</template>
      </el-table-column>
      <el-table-column label="我的状态" width="110">
        <template #default="{ row }">
          <el-tag v-if="row.my_status === 'solved'" type="success" size="small">已通过</el-tag>
          <el-tag v-else-if="row.my_status === 'attempted'" type="warning" size="small">尝试过</el-tag>
          <span v-else class="muted">-</span>
        </template>
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
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { listProblems } from '@/api'
import { DIFFICULTIES } from '@/utils/format'

const loading = ref(true)
const items = ref([])
const total = ref(0)

const query = reactive({
  page: 1,
  page_size: 20,
  keyword: '',
  difficulty: '',
  tag: '',
  source: '',
  status: ''
})

const rateText = (value) => `${Number(value || 0).toFixed(1)}%`

const buildParams = () => {
  const params = { page: query.page, page_size: query.page_size }
  if (query.keyword) {
    params.keyword = query.keyword
  }
  if (query.difficulty) {
    params.difficulty = query.difficulty
  }
  if (query.tag) {
    params.tag = query.tag
  }
  if (query.source) {
    params.source = query.source
  }
  if (query.status) {
    params.status = query.status
  }
  return params
}

const load = async () => {
  loading.value = true
  try {
    const data = await listProblems(buildParams())
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
  query.keyword = ''
  query.difficulty = ''
  query.tag = ''
  query.source = ''
  query.status = ''
  query.page = 1
  load()
}

const filterByTag = (tag) => {
  query.tag = tag
  query.page = 1
  load()
}

onMounted(load)
</script>

<style scoped>
.tag-click {
  margin-right: 4px;
  cursor: pointer;
}
</style>
