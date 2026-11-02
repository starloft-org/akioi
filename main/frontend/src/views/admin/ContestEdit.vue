<template>
  <div v-loading="loading" class="panel">
    <h3 class="panel__title">{{ isEdit ? '编辑比赛' : '新建比赛' }}</h3>

    <el-form :model="form" label-width="110px">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="标题">
            <el-input v-model="form.title" placeholder="比赛名称" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="比赛口令">
            <el-input v-model="form.password" show-password placeholder="留空表示不需要口令" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="开始时间">
            <el-date-picker
              v-model="form.start_at"
              type="datetime"
              placeholder="选择开始时间"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="结束时间">
            <el-date-picker
              v-model="form.end_at"
              type="datetime"
              placeholder="选择结束时间"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="封榜时长">
            <el-input-number v-model="form.freeze_minutes" :min="0" :step="5" />
            <span class="muted" style="margin-left: 8px">分钟，0 表示不封榜</span>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="归属团队">
            <el-select v-model="form.team_id" clearable placeholder="公开比赛（不属于团队）" style="width: 100%">
              <el-option v-for="item in teams" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="描述">
            <el-input v-model="form.description" type="textarea" :rows="4" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <div class="toolbar">
      <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      <el-button @click="router.push('/admin/contests')">返回列表</el-button>
    </div>

    <template v-if="isEdit">
      <el-divider content-position="left">题目编排</el-divider>
      <div class="toolbar">
        <el-select
          v-model="pickedProblemId"
          filterable
          remote
          :remote-method="searchProblems"
          :loading="searching"
          placeholder="输入题号或标题搜索"
          style="width: 360px"
          @visible-change="handleProblemsVisible"
        >
          <el-option
            v-for="item in problemOptions"
            :key="item.id"
            :label="`${item.display_id} ${item.title}`"
            :value="item.id"
          />
        </el-select>
        <el-button :disabled="!pickedProblemId" @click="addProblem">添加</el-button>
        <el-button type="primary" :loading="savingProblems" @click="saveProblems">保存编排</el-button>
      </div>
      <p class="muted">
        只能选择自己上传的题目，或所有公开题目（含个人题库、团队题库里的公开题）；隐藏题在比赛开始后会自动转为公开。
      </p>

      <el-table :data="selectedProblems" border>
        <el-table-column label="编号" width="110">
          <template #default="{ row }">
            <el-input v-model="row.label" size="small" placeholder="自动生成" />
          </template>
        </el-table-column>
        <el-table-column label="题目" min-width="200">
          <template #default="{ row }">{{ row.display_id }} {{ row.title }}</template>
        </el-table-column>
        <el-table-column label="顺序" width="160">
          <template #default="{ $index }">
            <el-button size="small" :disabled="$index === 0" @click="moveProblem($index, -1)">上移</el-button>
            <el-button
              size="small"
              :disabled="$index === selectedProblems.length - 1"
              @click="moveProblem($index, 1)"
            >
              下移
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="{ $index }">
            <el-button size="small" @click="removeProblemAt($index)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </template>
  </div>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  adminCreateContest,
  adminGetContest,
  adminListProblems,
  adminSetContestProblems,
  adminUpdateContest,
  listTeams
} from '@/api'
import { toLocalInput } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const contestId = ref(route.params.id ? Number(route.params.id) : null)

// 团队下拉：比赛可归属某个团队，归团队后仅成员可见
const teams = ref([])
const loadTeams = async () => {
  const data = await listTeams({ page: 1, page_size: 100 }).catch(() => ({ items: [] }))
  teams.value = data.items || []
}
loadTeams()
const isEdit = computed(() => !!contestId.value)

const loading = ref(false)
const saving = ref(false)
const savingProblems = ref(false)
const searching = ref(false)
const problemOptions = ref([])
const pickedProblemId = ref(null)
const selectedProblems = ref([])

const form = reactive({
  title: '',
  description: '',
  start_at: '',
  end_at: '',
  freeze_minutes: 0,
  password: '',
  team_id: null
})

const loadContest = async (id) => {
  loading.value = true
  try {
    const data = await adminGetContest(id)
    const contest = data.contest || {}
    form.title = contest.title || ''
    form.description = contest.description || ''
    form.start_at = toLocalInput(contest.start_at)
    form.end_at = toLocalInput(contest.end_at)
    form.freeze_minutes = contest.freeze_minutes ?? 0
    form.password = ''
    form.team_id = contest.team_id || null
    selectedProblems.value = (data.problems || []).map((item) => ({
      problem_id: item.problem_id,
      display_id: item.display_id || '',
      title: item.title || '',
      label: item.label || ''
    }))
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    loading.value = false
  }
}

watch(
  () => route.params.id,
  (id) => {
    contestId.value = id ? Number(id) : null
    if (contestId.value) {
      loadContest(contestId.value)
    }
  },
  { immediate: true }
)

const buildPayload = () => ({
  title: form.title.trim(),
  description: form.description,
  start_at: form.start_at ? new Date(form.start_at).toISOString() : null,
  end_at: form.end_at ? new Date(form.end_at).toISOString() : null,
  freeze_minutes: form.freeze_minutes,
  password: form.password,
  team_id: form.team_id || 0
})

const handleSave = async () => {
  if (!form.title.trim()) {
    ElMessage.error('请填写标题')
    return
  }
  if (!form.start_at || !form.end_at) {
    ElMessage.error('请选择开始与结束时间')
    return
  }
  if (new Date(form.end_at) <= new Date(form.start_at)) {
    ElMessage.error('结束时间需晚于开始时间')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await adminUpdateContest(contestId.value, buildPayload())
      ElMessage.success('保存成功')
      loadContest(contestId.value)
    } else {
      const data = await adminCreateContest(buildPayload())
      const id = data.contest?.id
      ElMessage.success('创建成功，可继续编排题目')
      if (id) {
        router.replace(`/admin/contests/${id}/edit`)
      } else {
        router.push('/admin/contests')
      }
    }
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    saving.value = false
  }
}

const searchProblems = async (keyword) => {
  searching.value = true
  try {
    const data = await adminListProblems({
      keyword: keyword || undefined,
      page: 1,
      page_size: 20,
      contest_selectable: 1
    })
    problemOptions.value = data.items || []
  } catch (e) {
    problemOptions.value = []
  } finally {
    searching.value = false
  }
}

// 下拉展开时若还没有候选题目，先按空关键词拉一批
const handleProblemsVisible = (visible) => {
  if (visible && !problemOptions.value.length) {
    searchProblems('')
  }
}

const addProblem = () => {
  const id = pickedProblemId.value
  if (!id) {
    return
  }
  if (selectedProblems.value.some((item) => item.problem_id === id)) {
    ElMessage.warning('该题目已在编排列表中')
    return
  }
  const found = problemOptions.value.find((item) => item.id === id)
  selectedProblems.value.push({
    problem_id: id,
    display_id: found?.display_id || '',
    title: found?.title || '',
    label: ''
  })
  pickedProblemId.value = null
}

const moveProblem = (index, offset) => {
  const target = index + offset
  if (target < 0 || target >= selectedProblems.value.length) {
    return
  }
  const list = selectedProblems.value
  const [item] = list.splice(index, 1)
  list.splice(target, 0, item)
}

const removeProblemAt = (index) => {
  selectedProblems.value.splice(index, 1)
}

const saveProblems = async () => {
  savingProblems.value = true
  try {
    const problems = selectedProblems.value.map((item, index) => ({
      problem_id: item.problem_id,
      label: item.label,
      order_index: index
    }))
    await adminSetContestProblems(contestId.value, problems)
    ElMessage.success('编排已保存')
    loadContest(contestId.value)
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    savingProblems.value = false
  }
}
</script>
