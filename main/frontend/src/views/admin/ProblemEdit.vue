<template>
  <div v-loading="loading" class="panel">
    <h3 class="panel__title">{{ isEdit ? '编辑题目' : '新建题目' }}</h3>

    <el-alert
      v-if="!isEdit"
      type="info"
      :closable="false"
      show-icon
      title="题面和测试数据分两步：保存后会自动进入测试数据上传页"
      style="margin-bottom: 16px"
    />

    <el-form :model="form" label-width="110px">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="题库">
            <el-select
              v-model="form.bank_id"
              placeholder="选择题库"
              style="width: 100%"
              :disabled="form.ownership !== 'public'"
              @change="handleBankChange"
            >
              <el-option
                v-for="bank in banks"
                :key="bank.id"
                :label="`${bank.name}（${bank.prefix} + ${bank.code_length} 位）`"
                :value="bank.id"
              />
            </el-select>
            <span v-if="form.ownership !== 'public'" class="form-hint">
              团队题目固定使用 T 题库，个人题目固定使用 P 题库
            </span>
            <span v-else-if="isEdit" class="form-hint">换题库后保存会按新题库重新编号</span>
          </el-form-item>
        </el-col>
        <el-col v-if="isEdit" :span="12">
          <el-form-item label="题号">
            <el-input v-model="form.display_id" disabled />
          </el-form-item>
        </el-col>
        <el-col :span="isEdit ? 24 : 12">
          <el-form-item label="标题">
            <el-input v-model="form.title" placeholder="题目名称" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="难度">
            <el-select
              v-model="form.difficulty"
              clearable
              placeholder="选择难度"
              style="width: 100%"
              @change="handleDifficultyChange"
            >
              <el-option v-for="item in DIFFICULTIES" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="来源">
            <el-input v-model="form.source" placeholder="题目来源" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="标签">
            <el-select
              v-model="form.tags"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="输入后回车添加"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="归属">
            <el-radio-group v-model="form.ownership" @change="handleOwnershipChange">
              <el-radio value="public">公共题目</el-radio>
              <el-radio value="team">团队题目</el-radio>
              <el-radio value="personal">个人题目</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-col>
        <el-col v-if="form.ownership === 'team'" :span="12">
          <el-form-item label="归属团队">
            <el-select v-model="form.team_id" placeholder="选择团队" style="width: 100%">
              <el-option v-for="item in teams" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col v-if="form.ownership === 'personal'" :span="12">
          <el-form-item label="归属用户">
            <el-select
              v-model="form.owner_id"
              filterable
              remote
              :remote-method="searchUsers"
              placeholder="输入用户名搜索"
              style="width: 100%"
            >
              <el-option
                v-for="item in users"
                :key="item.id"
                :label="`${item.username}（#${item.id}）`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="时限（ms）">
            <el-input-number v-model="form.time_limit_ms" :min="100" :max="30000" :step="100" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="内存（MB）">
            <el-input-number v-model="form.memory_limit_mb" :min="16" :max="2048" :step="16" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="排行榜积分">
            <el-input-number v-model="form.points" :min="0" :max="100000" />
            <span class="form-hint">随难度自动设置，可手动调整</span>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="可见性">
            <el-radio-group v-model="form.visibility">
              <el-radio :value="1">公开</el-radio>
              <el-radio :value="2">隐藏</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="访问权限">
            <el-radio-group v-model="form.access">
              <el-radio :value="1">公开</el-radio>
              <el-radio :value="2">{{ form.ownership === 'team' ? '仅团队成员' : '仅自己' }}</el-radio>
            </el-radio-group>
            <span v-if="form.ownership === 'public'" class="form-hint">公共题目只能设为公开</span>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="比赛专用">
            <el-switch v-model="form.contest_only" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="比对模式">
            <el-select v-model="form.compare_mode" style="width: 100%">
              <el-option v-for="item in COMPARE_MODES" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col v-if="form.compare_mode === 'float'" :span="12">
          <el-form-item label="浮点误差">
            <el-input-number v-model="form.float_eps" :min="0" :precision="9" :step="0.000001" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="题目描述">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="8"
          placeholder="支持 Markdown：标题、列表、表格、行内代码、三反引号代码块、图片等"
        />
      </el-form-item>
      <el-form-item label="输入格式">
        <el-input v-model="form.input_format" type="textarea" :rows="6" />
      </el-form-item>
      <el-form-item label="输出格式">
        <el-input v-model="form.output_format" type="textarea" :rows="6" />
      </el-form-item>
      <el-form-item label="提示 / 说明">
        <el-input v-model="form.hint" type="textarea" :rows="6" />
      </el-form-item>
    </el-form>

    <el-divider content-position="left">样例</el-divider>
    <div v-for="(sample, index) in form.samples" :key="index" style="margin-bottom: 16px">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-input v-model="sample.input" type="textarea" :rows="4" placeholder="样例输入" />
        </el-col>
        <el-col :span="12">
          <el-input v-model="sample.output" type="textarea" :rows="4" placeholder="样例输出" />
        </el-col>
        <el-col :span="24" style="margin-top: 8px">
          <el-input v-model="sample.explanation" type="textarea" :rows="2" placeholder="样例说明（可选）" />
        </el-col>
      </el-row>
      <el-button size="small" style="margin-top: 8px" @click="removeSample(index)">删除样例</el-button>
    </div>
    <el-button class="sample-add" type="primary" plain @click="addSample">+ 添加样例</el-button>

    <div class="toolbar" style="margin-top: 24px">
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      <el-button @click="router.push('/admin/problems')">返回列表</el-button>
      <el-button v-if="isEdit" @click="router.push(`/admin/problems/${problemId}/testdata`)">
        去管理测试数据
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  adminCreateProblem,
  adminGetProblem,
  adminListProblemBanks,
  adminListUsers,
  adminUpdateProblem,
  listTeams
} from '@/api'
import { COMPARE_MODES, DIFFICULTIES, defaultPointsOf } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const problemId = computed(() => (route.params.id ? Number(route.params.id) : null))
const isEdit = computed(() => !!problemId.value)

const loading = ref(false)
const submitting = ref(false)

const form = reactive({
  display_id: '',
  bank_id: null,
  title: '',
  description: '',
  input_format: '',
  output_format: '',
  hint: '',
  samples: [],
  time_limit_ms: 1000,
  memory_limit_mb: 256,
  difficulty: '',
  tags: [],
  source: '',
  points: 50,
  visibility: 1,
  contest_only: false,
  compare_mode: 'default',
  float_eps: 0.000001,
  team_id: null,
  owner_id: null,
  ownership: 'public',
  access: 1
})

// 团队下拉：题目可归属某个团队，归团队后仅成员可见
const teams = ref([])
const loadTeams = async () => {
  const data = await listTeams({ page: 1, page_size: 100 }).catch(() => ({ items: [] }))
  teams.value = data.items || []
}

// 题库下拉：题号由题库前缀与题号长度决定，新建题目必须选择题库
const banks = ref([])
const loadBanks = async () => {
  const data = await adminListProblemBanks().catch(() => ({ items: [] }))
  banks.value = data.items || []
  // 新建时优先用列表页带过来的题库，其次默认第一个题库；编辑时以题目自身归属为准
  if (!isEdit.value && !form.bank_id && banks.value.length) {
    const fromQuery = Number(route.query.bank_id) || 0
    form.bank_id = banks.value.some((item) => item.id === fromQuery) ? fromQuery : banks.value[0].id
  }
}

// 编辑时换题库会触发重新编号，先提示用户
const handleBankChange = (value) => {
  const bank = banks.value.find((item) => item.id === value)
  if (isEdit.value && bank) {
    ElMessage.warning(`保存后会按题库「${bank.name}」重新分配题号`)
  }
}

// 个人题目需要指定归属用户，按用户名远程搜索
const users = ref([])
const searchUsers = async (keyword) => {
  if (!keyword) {
    users.value = []
    return
  }
  const data = await adminListUsers({ keyword, page: 1, page_size: 20 }).catch(() => ({ items: [] }))
  users.value = data.items || []
}

// 切换归属时同步清理不属于该归属的字段，并校正访问权限默认值
const handleOwnershipChange = (value) => {
  if (value === 'public') {
    form.access = 1
    form.owner_id = null
    form.team_id = null
  } else if (value === 'team') {
    form.owner_id = null
    form.access = form.access === 1 ? 2 : form.access
  } else {
    form.team_id = null
    form.access = form.access === 1 ? 2 : form.access
  }
}

const loadProblem = async () => {
  loading.value = true
  try {
    const data = await adminGetProblem(problemId.value)
    const p = data.problem || {}
    form.display_id = p.display_id || ''
    form.bank_id = p.bank_id || null
    form.title = p.title || ''
    form.description = p.description || ''
    form.input_format = p.input_format || ''
    form.output_format = p.output_format || ''
    form.hint = p.hint || ''
    form.samples = (p.samples || []).map((item) => ({
      input: item.input || '',
      output: item.output || '',
      explanation: item.explanation || ''
    }))
    form.time_limit_ms = p.time_limit_ms ?? 1000
    form.memory_limit_mb = p.memory_limit_mb ?? 256
    form.difficulty = p.difficulty || ''
    form.tags = p.tags || []
    form.source = p.source || ''
    form.points = p.points ?? 100
    form.visibility = p.visibility ?? 1
    form.contest_only = !!p.contest_only
    form.compare_mode = p.compare_mode || 'default'
    form.float_eps = p.float_eps ?? 0.000001
    form.team_id = p.team_id || null
    form.owner_id = p.owner_id || null
    form.ownership = p.kind === 'team' ? 'team' : p.kind === 'personal' ? 'personal' : 'public'
    form.access = p.access ?? 1
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    loading.value = false
  }
}

const addSample = () => {
  form.samples.push({ input: '', output: '', explanation: '' })
}

const removeSample = (index) => {
  form.samples.splice(index, 1)
}

// 选择难度时带出该难度的默认积分，之后仍可手动调整。
const handleDifficultyChange = (value) => {
  if (value) {
    form.points = defaultPointsOf(value)
  }
}

// 题号由所属题库自动分配；编辑时回传原题号与题库，换题库后后端会重新编号
const buildPayload = () => ({
  ...(isEdit.value ? { display_id: form.display_id.trim() } : {}),
  ...(form.bank_id ? { bank_id: form.bank_id } : {}),
  title: form.title.trim(),
  description: form.description,
  input_format: form.input_format,
  output_format: form.output_format,
  hint: form.hint,
  samples: form.samples.map((item) => ({
    input: item.input || '',
    output: item.output || '',
    explanation: item.explanation || ''
  })),
  time_limit_ms: form.time_limit_ms,
  memory_limit_mb: form.memory_limit_mb,
  difficulty: form.difficulty,
  tags: form.tags,
  source: form.source,
  points: form.points,
  visibility: form.visibility,
  contest_only: form.contest_only,
  compare_mode: form.compare_mode,
  float_eps: form.float_eps,
  team_id: form.ownership === 'team' ? form.team_id || 0 : 0,
  owner_id: form.ownership === 'personal' ? form.owner_id || 0 : 0,
  access: form.access
})

const handleSubmit = async () => {
  if (!form.title.trim()) {
    ElMessage.error('请填写标题')
    return
  }
  if (form.ownership === 'team' && !form.team_id) {
    ElMessage.error('请选择归属团队')
    return
  }
  if (form.ownership === 'personal' && !form.owner_id) {
    ElMessage.error('请选择归属用户')
    return
  }
  if (!isEdit.value && !form.bank_id && form.ownership === 'public') {
    ElMessage.error('请选择题库')
    return
  }
  submitting.value = true
  try {
    if (isEdit.value) {
      await adminUpdateProblem(problemId.value, buildPayload())
      ElMessage.success('保存成功')
      router.push('/admin/problems')
      return
    }
    const data = await adminCreateProblem(buildPayload())
    const newId = data?.problem?.id
    if (newId) {
      ElMessage.success('题目已创建，接着上传测试数据')
      router.push(`/admin/problems/${newId}/testdata`)
      return
    }
    ElMessage.success('保存成功')
    router.push('/admin/problems')
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadTeams()
  loadBanks()
  if (isEdit.value) {
    loadProblem()
  }
})
</script>

<style scoped>
.sample-add {
  width: 100%;
  height: 36px;
  margin-top: 4px;
}

.form-hint {
  margin-left: 10px;
  font-size: 12px;
  color: var(--akioi-text-weak);
}
</style>
