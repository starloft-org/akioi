<template>
  <div class="panel">
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="标题关键词"
        clearable
        style="width: 220px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="router.push('/admin/contests/new')">新建比赛</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="title" label="标题" min-width="180" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="contestStatusType(row.status)">{{ contestStatusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="开始时间" width="170">
        <template #default="{ row }">{{ formatTime(row.start_at) }}</template>
      </el-table-column>
      <el-table-column label="结束时间" width="170">
        <template #default="{ row }">{{ formatTime(row.end_at) }}</template>
      </el-table-column>
      <el-table-column prop="freeze_minutes" label="封榜（分钟）" width="110" />
      <el-table-column prop="problem_count" label="题目数" width="90" />
      <el-table-column prop="participant_count" label="参与人数" width="100" />
      <el-table-column label="需要口令" width="100">
        <template #default="{ row }">{{ row.needs_password ? '是' : '否' }}</template>
      </el-table-column>
      <el-table-column label="报名" width="90">
        <template #default>一律报名</template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="router.push(`/admin/contests/${row.id}/edit`)">编辑</el-button>
          <el-button size="small" @click="openRank(row)">榜单</el-button>
          <el-button size="small" @click="removeContest(row)">删除</el-button>
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
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminDeleteContest, adminListContests } from '@/api'
import { contestStatusText, contestStatusType, formatTime } from '@/utils/format'

const router = useRouter()
const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const load = async () => {
  loading.value = true
  try {
    const data = await adminListContests({
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

const openRank = (row) => {
  window.open(`/contests/${row.id}/rank`)
}

const removeContest = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除比赛「${row.title}」？`, '删除确认', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await adminDeleteContest(row.id)
  } catch (e) {
    return
  }
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
