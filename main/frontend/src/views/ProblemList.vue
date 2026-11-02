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
      <el-table-column width="44" align="center">
        <template #default="{ row }">
          <span
            class="status-dot"
            :class="row.my_status || 'todo'"
            :title="statusText(row.my_status)"
          ></span>
        </template>
      </el-table-column>
      <el-table-column label="题号" width="100">
        <template #default="{ row }">
          <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="标题" min-width="220">
        <template #default="{ row }">
          <router-link class="problem-link" :to="`/problems/${row.id}`">{{ row.title }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="难度" width="110">
        <template #default="{ row }">
          <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
        </template>
      </el-table-column>
      <el-table-column label="作者" width="120">
        <template #default="{ row }">
          <span v-if="row.author_name">{{ row.author_name }}</span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="通过率" width="150">
        <template #default="{ row }">
          <div class="rate-cell">
            <div class="rate-bar">
              <span :style="{ width: rateWidth(row.acceptance_rate) }"></span>
            </div>
            <span class="rate-text">{{ rateText(row.acceptance_rate) }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="提交 / 通过人数" width="140" align="right">
        <template #default="{ row }">
          <span class="tabular">{{ row.submit_count }} / {{ row.accepted_count }}</span>
        </template>
      </el-table-column>
      <el-table-column label="标签" min-width="170">
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
      <el-table-column label="积分" width="76" align="right" prop="points" />
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
import DifficultyTag from '@/components/DifficultyTag.vue'

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

// 通过率进度条宽度：至少保留 2% 以显示颜色
const rateWidth = (value) => `${Math.max(2, Math.min(100, Number(value || 0)))}%`

const statusText = (status) => {
  if (status === 'solved') return '已通过'
  if (status === 'attempted') return '尝试过'
  return '未作答'
}

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

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dcdfe6;
}

.status-dot.solved {
  background: #00b42a;
}

.status-dot.attempted {
  background: #ff7d00;
}

.problem-link {
  font-weight: 500;
}

.rate-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rate-bar {
  flex: 1;
  height: 4px;
  border-radius: 2px;
  background: #f2f3f5;
  overflow: hidden;
}

.rate-bar span {
  display: block;
  height: 100%;
  border-radius: 2px;
  background: var(--akioi-primary);
}

.rate-text {
  width: 46px;
  text-align: right;
  font-size: 12px;
  color: var(--akioi-text-weak);
  font-variant-numeric: tabular-nums;
}

.tabular {
  color: var(--akioi-text-weak);
  font-variant-numeric: tabular-nums;
}
</style>
