<template>
  <div class="panel">
    <h3 class="panel__title">我的题目</h3>
    <p class="panel__hint">
      团队题目由队长创建、题号以 T 开头；个人题目由自己创建、题号以 P 开头。题号由题库自动分配，
      访问权限可设为「公开」或「仅团队/仅自己」。
    </p>

    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        placeholder="题号或标题"
        clearable
        style="width: 200px"
        @keyup.enter="search"
      />
      <el-select v-model="query.kind" placeholder="类型" clearable style="width: 130px">
        <el-option label="团队题目" value="team" />
        <el-option label="个人题目" value="personal" />
      </el-select>
      <el-select v-model="query.team_id" placeholder="团队" clearable style="width: 170px">
        <el-option v-for="t in teams" :key="t.id" :label="t.name" :value="t.id" />
      </el-select>
      <el-button type="primary" @click="search">筛选</el-button>
      <el-button type="success" @click="openCreate">新建题目</el-button>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column label="题号" width="110">
        <template #default="{ row }">
          <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="标题" min-width="200">
        <template #default="{ row }">
          <router-link class="problem-link" :to="`/problems/${row.id}`">{{ row.title }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="row.kind === 'team' ? 'warning' : 'success'">
            {{ row.kind === 'team' ? '团队题目' : '个人题目' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="访问权限" width="110">
        <template #default="{ row }">
          <el-tag size="small" :type="row.access === 1 ? 'success' : 'info'">
            {{ accessText(row.access) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="难度" width="100">
        <template #default="{ row }">
          <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="测试点" width="80" align="right">
        <template #default="{ row }">
          <span class="tabular">{{ row.testcase_count }}</span>
        </template>
      </el-table-column>
      <el-table-column label="提交 / 通过" width="110" align="right">
        <template #default="{ row }">
          <span class="tabular">{{ row.submit_count }} / {{ row.accepted_count }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" @click="openTestdata(row)">测试数据</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next, total"
        @current-change="load"
      />
    </div>

    <el-dialog v-model="dialog" :title="editing ? '编辑题目' : '新建题目'" width="720px">
      <el-form label-width="96px" label-position="right">
        <el-form-item v-if="!editing" label="题目类型">
          <el-radio-group v-model="form.ownership">
            <el-radio value="personal">个人题目</el-radio>
            <el-radio value="team">团队题目</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!editing && form.ownership === 'team'" label="归属团队">
          <el-select v-model="form.team_id" placeholder="请选择团队" style="width: 100%">
            <el-option v-for="t in ownerTeams" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
          <p class="form-hint">只有你担任队长的团队可以创建团队题目。</p>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="128" />
        </el-form-item>
        <el-form-item label="访问权限">
          <el-radio-group v-model="form.access">
            <el-radio :value="1">公开</el-radio>
            <el-radio :value="2">{{ form.ownership === 'team' ? '仅团队成员' : '仅自己' }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="难度">
          <el-select v-model="form.difficulty" clearable style="width: 160px">
            <el-option v-for="item in DIFFICULTIES" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="标签">
          <el-select v-model="form.tags" multiple filterable allow-create default-first-option style="width: 100%">
            <el-option v-for="item in TAGS" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="时空限制">
          <el-input-number v-model="form.time_limit_ms" :min="100" :max="30000" :step="100" />
          <span class="muted" style="margin: 0 8px">毫秒</span>
          <el-input-number v-model="form.memory_limit_mb" :min="16" :max="2048" :step="16" />
          <span class="muted" style="margin-left: 8px">MB</span>
        </el-form-item>
        <el-form-item label="分值">
          <el-input-number v-model="form.points" :min="0" :max="100000" />
        </el-form-item>
        <el-form-item label="题目描述">
          <el-input v-model="form.description" type="textarea" :rows="5" />
        </el-form-item>
        <el-form-item label="输入格式">
          <el-input v-model="form.input_format" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="输出格式">
          <el-input v-model="form.output_format" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="提示">
          <el-input v-model="form.hint" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="样例">
          <div style="width: 100%">
            <div v-for="(sample, index) in form.samples" :key="index" class="sample-row">
              <div class="sample-row__index">样例 {{ index + 1 }}</div>
              <el-input v-model="sample.input" type="textarea" :rows="2" placeholder="输入" />
              <el-input v-model="sample.output" type="textarea" :rows="2" placeholder="输出" />
              <el-button link type="danger" @click="form.samples.splice(index, 1)">删除</el-button>
            </div>
            <el-button size="small" @click="addSample">添加样例</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="testdataDialog" title="测试数据" width="720px">
      <p class="panel__hint">
        可以上传单个 zip（包含若干 .in / .out 文件），也可以一次选择多个测试数据文件。
      </p>
      <el-upload :auto-upload="false" :file-list="fileList" multiple @change="onFileChange" @remove="onFileRemove">
        <el-button size="small">选择文件</el-button>
      </el-upload>
      <div class="toolbar" style="margin-top: 12px">
        <el-button type="primary" :loading="uploading" :disabled="!fileList.length" @click="upload">
          上传并替换
        </el-button>
        <el-button type="danger" :disabled="!testcases.length" @click="clearTestdata">清空测试数据</el-button>
        <span class="muted">总分 {{ totalScore }}</span>
      </div>
      <el-table :data="testcases" size="small" style="margin-top: 12px">
        <el-table-column label="#" prop="index" width="60" />
        <el-table-column label="输入大小" width="120">
          <template #default="{ row }">{{ row.input_size }} B</template>
        </el-table-column>
        <el-table-column label="输出大小" width="120">
          <template #default="{ row }">{{ row.output_size }} B</template>
        </el-table-column>
        <el-table-column label="分值" width="80" align="right">
          <template #default="{ row }">{{ row.score }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import DifficultyTag from '@/components/DifficultyTag.vue'
import {
  createMyProblem,
  deleteMyProblem,
  deleteMyProblemTestdata,
  listMyProblemTestdata,
  listMyProblems,
  listTeams,
  updateMyProblem,
  uploadMyProblemTestdata
} from '@/api'

const DIFFICULTIES = ['入门', '普及-', '普及', '普及+', '提高', '提高+', '省选', 'NOI']
const TAGS = ['模拟', '枚举', '排序', '二分', '贪心', '搜索', '动态规划', '图论', '树', '数学', '字符串', '数据结构']

const loading = ref(false)
const items = ref([])
const teams = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const query = ref({ keyword: '', kind: '', team_id: '' })

const dialog = ref(false)
const editing = ref(null)
const saving = ref(false)
const emptyForm = () => ({
  ownership: 'personal',
  team_id: '',
  title: '',
  access: 2,
  difficulty: '',
  tags: [],
  time_limit_ms: 1000,
  memory_limit_mb: 256,
  points: 100,
  description: '',
  input_format: '',
  output_format: '',
  hint: '',
  samples: []
})
const form = ref(emptyForm())

const testdataDialog = ref(false)
const testdataTarget = ref(null)
const testcases = ref([])
const fileList = ref([])
const uploading = ref(false)

const ownerTeams = computed(() => teams.value.filter((t) => t.my_role === 'owner'))
const totalScore = computed(() => testcases.value.reduce((sum, tc) => sum + (tc.score || 0), 0))

const accessText = (access) => (access === 1 ? '公开' : '受限')

async function load() {
  loading.value = true
  try {
    const data = await listMyProblems({
      page: page.value,
      page_size: pageSize,
      keyword: query.value.keyword || undefined,
      kind: query.value.kind || undefined,
      team_id: query.value.team_id || undefined
    })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '加载题目失败')
  } finally {
    loading.value = false
  }
}

async function loadTeams() {
  try {
    const data = await listTeams({ page: 1, page_size: 100 })
    teams.value = data.items || []
  } catch {
    teams.value = []
  }
}

function search() {
  page.value = 1
  load()
}

function openCreate() {
  editing.value = null
  form.value = emptyForm()
  dialog.value = true
}

async function openEdit(row) {
  editing.value = row
  form.value = {
    ...emptyForm(),
    ownership: row.kind === 'team' ? 'team' : 'personal',
    team_id: row.team_id || '',
    title: row.title,
    access: row.access || 1,
    difficulty: row.difficulty || '',
    tags: [...(row.tags || [])],
    time_limit_ms: row.time_limit_ms || 1000,
    memory_limit_mb: row.memory_limit_mb || 256,
    points: row.points || 0
  }
  dialog.value = true
  try {
    const { getMyProblem } = await import('@/api')
    const data = await getMyProblem(row.id)
    const p = data.problem || {}
    form.value.description = p.description || ''
    form.value.input_format = p.input_format || ''
    form.value.output_format = p.output_format || ''
    form.value.hint = p.hint || ''
    form.value.samples = (p.samples || []).map((s) => ({ input: s.input, output: s.output }))
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '加载题目详情失败')
  }
}

function addSample() {
  form.value.samples.push({ input: '', output: '' })
}

async function save() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写题目标题')
    return
  }
  if (!editing.value && form.value.ownership === 'team' && !form.value.team_id) {
    ElMessage.warning('请选择归属团队')
    return
  }
  const payload = {
    title: form.value.title,
    access: form.value.access,
    difficulty: form.value.difficulty,
    tags: form.value.tags,
    time_limit_ms: form.value.time_limit_ms,
    memory_limit_mb: form.value.memory_limit_mb,
    points: form.value.points,
    description: form.value.description,
    input_format: form.value.input_format,
    output_format: form.value.output_format,
    hint: form.value.hint,
    samples: form.value.samples
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateMyProblem(editing.value.id, payload)
      ElMessage.success('题目已更新')
    } else {
      if (form.value.ownership === 'team') {
        payload.team_id = form.value.team_id
      }
      const data = await createMyProblem(payload)
      ElMessage.success(`题目已创建，题号 ${data.problem?.display_id || ''}`)
    }
    dialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '保存题目失败')
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除题目「${row.display_id} ${row.title}」？测试数据会一起删除。`, '删除题目', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await deleteMyProblem(row.id)
    ElMessage.success('题目已删除')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '删除题目失败')
  }
}

async function openTestdata(row) {
  testdataTarget.value = row
  fileList.value = []
  testdataDialog.value = true
  await loadTestdata()
}

async function loadTestdata() {
  if (!testdataTarget.value) return
  try {
    const data = await listMyProblemTestdata(testdataTarget.value.id)
    testcases.value = data.items || []
  } catch (e) {
    testcases.value = []
    ElMessage.error(e?.response?.data?.message || '加载测试数据失败')
  }
}

function onFileChange(file, files) {
  fileList.value = files
}

function onFileRemove(file, files) {
  fileList.value = files
}

async function upload() {
  const files = fileList.value.map((f) => f.raw).filter(Boolean)
  if (!files.length) {
    ElMessage.warning('请选择文件')
    return
  }
  uploading.value = true
  try {
    const data = await uploadMyProblemTestdata(testdataTarget.value.id, files)
    testcases.value = data.items || []
    fileList.value = []
    ElMessage.success('测试数据已更新')
    if (data.sync_failed?.length) {
      ElMessage.warning(`以下评测节点同步失败：${data.sync_failed.join(', ')}`)
    }
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '上传测试数据失败')
  } finally {
    uploading.value = false
  }
}

async function clearTestdata() {
  try {
    await ElMessageBox.confirm('确定清空该题的全部测试数据？', '清空测试数据', { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteMyProblemTestdata(testdataTarget.value.id)
    testcases.value = []
    ElMessage.success('测试数据已清空')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '清空测试数据失败')
  }
}

onMounted(() => {
  load()
  loadTeams()
})
</script>

<style scoped>
.form-hint {
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.sample-row {
  display: grid;
  grid-template-columns: 70px 1fr 1fr 50px;
  gap: 8px;
  align-items: start;
  margin-bottom: 8px;
}

.sample-row__index {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  padding-top: 6px;
}
</style>
