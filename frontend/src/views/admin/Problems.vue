<template>
  <div class="panel">
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="题号 / 标题 / 关键词"
        clearable
        style="width: 220px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-select v-model="visibilityFilter" placeholder="可见性" clearable style="width: 120px">
        <el-option label="公开" :value="1" />
        <el-option label="隐藏" :value="2" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="router.push('/admin/problems/new')">新建题目</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="visibleItems" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="display_id" label="题号" width="100" />
      <el-table-column prop="title" label="标题" min-width="180" />
      <el-table-column prop="difficulty" label="难度" width="100" />
      <el-table-column label="标签" min-width="160">
        <template #default="{ row }">
          <el-tag v-for="tag in row.tags" :key="tag" size="small" style="margin-right: 4px">{{ tag }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="可见性" width="90">
        <template #default="{ row }">
          <el-tag :type="row.visibility === 1 ? 'success' : 'info'">
            {{ row.visibility === 1 ? '公开' : '隐藏' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="比赛专用" width="100">
        <template #default="{ row }">{{ row.contest_only ? '是' : '否' }}</template>
      </el-table-column>
      <el-table-column prop="testcase_count" label="测试点" width="90" />
      <el-table-column prop="total_score" label="总分" width="80" />
      <el-table-column label="提交 / 通过" width="120">
        <template #default="{ row }">{{ row.submit_count }} / {{ row.accepted_count }}</template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="router.push(`/admin/problems/${row.id}/edit`)">编辑</el-button>
          <el-button size="small" @click="router.push(`/admin/problems/${row.id}/testdata`)">数据</el-button>
          <el-button size="small" @click="removeProblem(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="load"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminDeleteProblem, adminListProblems } from '@/api'

const router = useRouter()
const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const visibilityFilter = ref('')

const visibleItems = computed(() => {
  if (!visibilityFilter.value) {
    return items.value
  }
  return items.value.filter((row) => row.visibility === visibilityFilter.value)
})

const load = async () => {
  loading.value = true
  try {
    const data = await adminListProblems({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined
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

const removeProblem = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除题目「${row.display_id} ${row.title}」？`, '删除确认', {
      type: 'warning'
    })
  } catch (e) {
    return
  }
  try {
    await adminDeleteProblem(row.id)
  } catch (e) {
    return
  }
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
