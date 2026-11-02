<template>
  <div class="panel">
    <h3 class="panel__title">我的题目</h3>
    <p class="panel__hint">
      这里只显示你自己上传的个人题目；团队题目请到团队页面查看。题号由题库自动分配，
      访问权限可设为「公开」或「仅自己」。
    </p>

    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        placeholder="题号或标题"
        clearable
        style="width: 200px"
        @keyup.enter="search"
      />
      <el-button type="success" @click="openCreate">新建题目</el-button>
      <el-button type="warning" @click="openImport">批量导入</el-button>
      <el-button type="primary" :disabled="selectedProblems.length === 0" @click="openConvert">
        导入到团队（{{ selectedProblems.length }}）
      </el-button>
    </div>

    <el-table v-loading="loading" :data="items" size="small">
      <el-table-column type="selection" width="44" @selection-change="onSelectionChange" />
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
      <el-table-column label="作者" width="120">
        <template #default="{ row }">
          <span v-if="row.author_name">{{ row.author_name }}</span>
          <span v-else class="muted">—</span>
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
      <el-input
        v-model="mySubtasksInput"
        type="textarea"
        :rows="3"
        placeholder='可选：子任务与按文件名分值（等价于 subtasks.json），例如 {"scores":{"sample":20},"subtasks":[{"name":"小数据","score":30,"cases":["sample"]}]}'
        style="margin-top: 8px"
      />
      <p v-if="mySubtaskSummary" class="muted">{{ mySubtaskSummary }}</p>
      <el-table :data="testcases" size="small" style="margin-top: 12px">
        <el-table-column label="#" prop="index" width="60" />
        <el-table-column label="文件名" prop="name" min-width="140" />
        <el-table-column label="分组" width="100">
          <template #default="{ row }">{{ row.group_id ? `子任务 ${row.group_id}` : '独立' }}</template>
        </el-table-column>
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
    <el-dialog v-model="importDialog" title="批量导入个人题目" width="760px" :close-on-click-modal="false">
      <p class="panel__hint">
        题目包是包含 <code>problem.md</code> 与 <code>*.in</code> / <code>*.out</code> 测试数据的 zip。
        可以一次选一个目录里的所有包，也可以直接选多个 zip；导入成功的包会从待导入列表里移除，失败后可以重试剩下的包。
      </p>

      <div class="toolbar">
        <el-button type="primary" @click="pickDirectory">选择题目包目录</el-button>
        <input ref="dirInputRef" type="file" webkitdirectory multiple class="dir-input" @change="onDirectoryChange" />
        <el-upload :auto-upload="false" :show-file-list="false" multiple accept=".zip" @change="onUploadChange">
          <el-button>选择 zip 文件</el-button>
        </el-upload>
        <span v-if="importDirName" class="muted">目录：{{ importDirName }}</span>
        <el-button v-if="importState" link type="danger" @click="discardImportRecord">丢弃记录</el-button>
      </div>

      <el-alert
        v-if="importHasRemaining && !importing"
        type="warning"
        :closable="false"
        title="还有包没有导入完成"
        class="import-resume"
      >
        <p>共 {{ importTotal || stateFiles.length }} 个包，已完成 {{ stateOkCount }} 个，还剩 {{ importRemaining.length }} 个没有导完。</p>
        <p>重新选择上次的目录后点「开始导入」，已经导进去的包会自动跳过。</p>
      </el-alert>

      <div v-if="pendingEntries.length" class="import-queue">
        <div class="import-queue__title">待导入：{{ pendingEntries.length }} 个包</div>
        <div v-for="entry in pendingEntries.slice(0, 50)" :key="entry.key" class="import-queue__row">
          <span class="muted">{{ entry.index }}.</span>
          <span class="import-queue__name">{{ entry.relPath || entry.name }}</span>
          <el-tag v-if="entry.status === 'failed'" type="danger" size="small">上次失败</el-tag>
          <el-tag v-else-if="entry.status === 'running'" type="warning" size="small">上传中</el-tag>
          <el-tag v-else type="info" size="small">待上传</el-tag>
          <span v-if="entry.error" class="import-queue__error">{{ entry.error }}</span>
        </div>
        <p v-if="pendingEntries.length > 50" class="muted">还有 {{ pendingEntries.length - 50 }} 个未显示</p>
      </div>

      <div v-if="importPhase !== 'idle'" class="import-progress">
        <el-progress :percentage="importCurrentPercent" :status="progressStatus" />
        <p class="muted">
          <template v-if="importPhase === 'importing'">
            {{ importMessage || `正在上传 ${importCurrentName}` }}（第
            {{ Math.min(importDone + 1, importTotal) }} / {{ importTotal }} 个包，已成功
            {{ stateOkCount }} 个）
          </template>
          <template v-else-if="importPhase === 'failed'">
            已停止：{{ importStopReason }}
          </template>
          <template v-else>
            导入完成：成功 {{ stateOkCount }} 个
          </template>
        </p>
      </div>

      <template #footer>
        <el-button @click="importDialog = false">关闭</el-button>
        <el-button v-if="importPhase === 'failed'" type="warning" :loading="importing" @click="retryImport">
          重试剩余 {{ importRemaining.length || pendingEntries.length }} 个包
        </el-button>
        <el-button v-else type="primary" :loading="importing" :disabled="!importTargets.length" @click="handleImport">
          开始导入
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="convertDialog" title="导入到团队" width="440px">
      <p class="muted">题目会变成团队题目，进入团队题库并重新分配题号；只有团队成员能在团队页面看到。</p>
      <el-select v-model="convertTeamId" placeholder="选择团队" style="width: 100%">
        <el-option v-for="team in convertTeams" :key="team.id" :label="team.name" :value="team.id" />
      </el-select>
      <p v-if="convertTeams.length === 0" class="muted" style="margin-top: 8px">你还没有加入任何团队，可以先在团队页面创建或加入团队。</p>
      <template #footer>
        <el-button @click="convertDialog = false">取消</el-button>
        <el-button type="primary" :loading="converting" :disabled="!convertTeamId" @click="confirmConvert">导入</el-button>
      </template>
    </el-dialog>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import DifficultyTag from '@/components/DifficultyTag.vue'
import {
  convertMyProblems,
  createMyProblem,
  deleteMyProblem,
  deleteMyProblemTestdata,
  listMyProblemTestdata,
  listMyProblems,
  listMyTeams,
  listTeams,
  myGetImportTask,
  myImportProblemsOne,
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
const query = ref({ keyword: '' })

const selectedProblems = ref([])
const convertDialog = ref(false)
const convertTeams = ref([])
const convertTeamId = ref('')
const converting = ref(false)

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
const subtasks = ref([])
const mySubtasksInput = ref('')
const fileList = ref([])
const uploading = ref(false)

const ownerTeams = computed(() => teams.value.filter((t) => t.my_role === 'owner'))
const totalScore = computed(() =>
  testcases.value.reduce((sum, tc) => sum + (tc.score || 0), 0) +
  subtasks.value.reduce((sum, st) => sum + (st.score || 0), 0)
)
const mySubtaskSummary = computed(() =>
  subtasks.value
    .map((st) => `${st.name}（${st.score} 分，${(st.cases || []).length} 个测试点）`)
    .join('、')
)

const accessText = (access) => (access === 1 ? '公开' : '受限')

async function load() {
  loading.value = true
  try {
    const data = await listMyProblems({
      page: page.value,
      page_size: pageSize,
      keyword: query.value.keyword || undefined
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

function onSelectionChange(rows) {
  selectedProblems.value = rows
}

async function openConvert() {
  if (selectedProblems.value.length === 0) {
    return
  }
  try {
    const data = await listMyTeams()
    convertTeams.value = data.items || []
  } catch {
    convertTeams.value = []
  }
  convertTeamId.value = convertTeams.value.length > 0 ? convertTeams.value[0].id : ''
  convertDialog.value = true
}

async function confirmConvert() {
  if (!convertTeamId.value) {
    ElMessage.warning('请选择团队')
    return
  }
  converting.value = true
  try {
    const data = await convertMyProblems({
      problem_ids: selectedProblems.value.map((row) => row.id),
      target: 'team',
      team_id: convertTeamId.value
    })
    ElMessage.success(`已导入团队：${data.converted_count} 道题目，题号已重新分配`)
    convertDialog.value = false
    selectedProblems.value = []
    await load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '导入团队失败')
  } finally {
    converting.value = false
  }
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
ElMessage.success(form.value.ownership === 'team' ? '团队题目已创建，请到团队页面查看' : `题目已创建，题号 ${data.problem?.display_id || ''}`)
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
    subtasks.value = data.subtasks || []
    if (!mySubtasksInput.value && subtasks.value.length) {
      const nameOf = (idx) => testcases.value.find((it) => it.index === idx)?.name || String(idx)
      mySubtasksInput.value = JSON.stringify(
        {
          subtasks: subtasks.value.map((st) => ({
            name: st.name,
            score: st.score,
            cases: (st.cases || []).map(nameOf)
          }))
        },
        null,
        2
      )
    }
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
  let subtasksText = null
  const specText = mySubtasksInput.value.trim()
  if (specText) {
    try {
      JSON.parse(specText)
    } catch (e) {
      ElMessage.error('子任务与分值需为 JSON')
      return
    }
    subtasksText = specText
  }
  const files = fileList.value.map((f) => f.raw).filter(Boolean)
  if (!files.length) {
    ElMessage.warning('请选择文件')
    return
  }
  uploading.value = true
  try {
    const data = await uploadMyProblemTestdata(testdataTarget.value.id, files, null, subtasksText)
    testcases.value = data.items || []
    subtasks.value = data.subtasks || []
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

// 个人题目批量导入：记录里只保留没导完的包，导成功的包直接丢掉
const IMPORT_STATE_KEY = 'akioi:my-import-state'

const importDialog = ref(false)
const importing = ref(false)
const importFiles = ref([])
const importDirFiles = ref([])
const importDirName = ref('')
const dirInputRef = ref(null)
const importState = ref(null)
const importPhase = ref('idle')
const importDone = ref(0)
const importTotal = ref(0)
const importCurrentPercent = ref(0)
const importCurrentName = ref('')
const importMessage = ref('')
const importStopReason = ref('')
const importOkCount = ref(0)

const entryKey = (entry) => entry.relPath || entry.name

const loadImportState = () => {
  try {
    const raw = localStorage.getItem(IMPORT_STATE_KEY)
    if (!raw) {
      return null
    }
    const saved = JSON.parse(raw)
    return saved && Array.isArray(saved.files) ? saved : null
  } catch {
    return null
  }
}

const persistImportState = () => {
  if (!importState.value) {
    return
  }
  try {
    localStorage.setItem(IMPORT_STATE_KEY, JSON.stringify(importState.value))
  } catch {
    // 配额满了就不记了，不影响本次导入
  }
}

const clearImportState = () => {
  importState.value = null
  try {
    localStorage.removeItem(IMPORT_STATE_KEY)
  } catch {
    // 忽略
  }
}

const stateFiles = computed(() => importState.value?.files || [])
const stateOkCount = computed(() => importOkCount.value)
const importRemaining = computed(() => stateFiles.value)
const importHasRemaining = computed(() => importRemaining.value.length > 0)
const progressStatus = computed(() => {
  if (importPhase.value === 'failed') {
    return 'exception'
  }
  return importPhase.value === 'done' ? 'success' : undefined
})

// 目录导入时优先用目录里选到的 zip，否则用 el-upload 里选中的包
const importTargets = computed(() => {
  if (importDirFiles.value.length) {
    return importDirFiles.value
  }
  return importFiles.value
    .filter((item) => item.raw)
    .map((item) => ({
      name: item.name,
      relPath: item.webkitRelativePath || item.name,
      raw: item.raw
    }))
})

const pendingEntries = computed(() => {
  const failed = new Map()
  stateFiles.value.forEach((entry) => failed.set(entryKey(entry), entry))
  return importTargets.value.map((file, index) => {
    const key = file.relPath || file.name
    const entry = failed.get(key)
    const running = importing.value && importCurrentName.value === file.name && !entry
    return {
      key,
      index: index + 1,
      name: file.name,
      relPath: file.relPath && file.relPath !== file.name ? file.relPath : '',
      status: entry?.status === 'failed' ? 'failed' : running ? 'running' : 'waiting',
      error: entry?.error || ''
    }
  })
})

const topDirName = (entries) => {
  const first = entries[0]?.relPath || ''
  const index = first.indexOf('/')
  return index > 0 ? first.slice(0, index) : ''
}

const restoreImportState = (saved) => {
  importState.value = saved
  importDirName.value = saved.dir || ''
  importOkCount.value = saved.done || 0
  importDone.value = saved.done || 0
  importTotal.value = saved.total || (saved.files || []).length
  importPhase.value = importHasRemaining.value ? 'idle' : 'done'
}

const openImport = () => {
  const saved = loadImportState()
  if (saved) {
    restoreImportState(saved)
    if (importHasRemaining.value) {
      ElMessage.info(`上次还有 ${saved.files.length} 个包没有导入完成，重新选择目录后可以接着导`)
    }
  }
  importDialog.value = true
}

const pickDirectory = () => {
  dirInputRef.value?.click()
}

const onDirectoryChange = (event) => {
  const picked = Array.from(event.target.files || [])
  // 允许再次选择同一个目录：先清空，否则浏览器不会再触发 change
  event.target.value = ''
  if (!picked.length) {
    return
  }
  const zips = picked.filter((file) => /\.zip$/i.test(file.name))
  if (!zips.length) {
    ElMessage.warning('选中的目录里没有 zip 题目包')
    return
  }
  importDirFiles.value = zips.map((file) => ({
    name: file.name,
    relPath: file.webkitRelativePath || file.name,
    raw: file
  }))
  importFiles.value = []
  importDirName.value = topDirName(importDirFiles.value)
  const savedDir = importState.value?.dir
  if (savedDir && importDirName.value && savedDir !== importDirName.value) {
    ElMessage.warning(`这次选的是「${importDirName.value}」，和上次的「${savedDir}」不一样，续传可能对不上`)
  }
  importPhase.value = 'idle'
  importMessage.value = ''
  importStopReason.value = ''
  ElMessage.success(`已选择「${importDirName.value || '当前目录'}」里的 ${zips.length} 个题目包`)
}

const onUploadChange = (file, files) => {
  importDirFiles.value = []
  importDirName.value = ''
  importFiles.value = files
}

const discardImportRecord = async () => {
  try {
    await ElMessageBox.confirm(
      '丢弃记录后需要重新选择全部题目包（已经导入的题目不受影响）。',
      '丢弃导入记录',
      { type: 'warning' }
    )
  } catch {
    return
  }
  clearImportState()
  importDone.value = 0
  importTotal.value = 0
  importOkCount.value = 0
  importPhase.value = 'idle'
  ElMessage.success('已丢弃导入记录')
}

// 轮询后台导入任务：任务只存在服务端内存里，重启或过期后返回 404，要明确失败
const pollImportTask = (taskId, onProgress) =>
  new Promise((resolve, reject) => {
    let timer = null
    let misses = 0
    const stop = () => {
      if (timer) {
        clearTimeout(timer)
        timer = null
      }
    }
    const tick = async () => {
      try {
        const task = await myGetImportTask(taskId)
        misses = 0
        onProgress?.(task)
        if (task.status === 'done') {
          stop()
          resolve(task)
          return
        }
        if (task.status === 'failed') {
          stop()
          reject(new Error(task.message || '导入失败'))
          return
        }
      } catch (e) {
        if (e?.response?.status === 404) {
          stop()
          reject(new Error('导入任务已不存在（后端可能重启过），请重新上传这个包'))
          return
        }
        if (!e?.response) {
          stop()
          reject(e)
          return
        }
        misses += 1
        if (misses >= 10) {
          stop()
          reject(new Error(e?.response?.data?.message || e?.message || '查询导入进度失败'))
          return
        }
      }
      timer = setTimeout(tick, 1000)
    }
    tick()
  })

const importOne = async (file, onProgress) => {
  try {
    importCurrentName.value = file.name
    importCurrentPercent.value = 0
    importMessage.value = `正在上传 ${file.name}`
    const data = await myImportProblemsOne(file, (percent) => {
      importCurrentPercent.value = Math.min(30, Math.round(percent * 0.3))
      importMessage.value = `正在上传 ${file.name} ${percent}%`
    })
    if (Array.isArray(data?.items)) {
      importCurrentPercent.value = 100
      return { items: data.items, container: false }
    }
    if (data?.task_id) {
      importCurrentPercent.value = Math.max(importCurrentPercent.value, 30)
      importMessage.value = '上传完成，正在校验并导入…'
      const task = await pollImportTask(data.task_id, onProgress)
      importCurrentPercent.value = 100
      return { items: task.items || [], container: true, total: task.total || 0 }
    }
    return { items: [{ filename: file.name, ok: false, error: '导入没有返回结果' }], container: false }
  } catch (e) {
    importCurrentPercent.value = 100
    return {
      items: [{ filename: file.name, ok: false, error: e?.response?.data?.message || e?.message || '导入失败' }],
      container: false
    }
  }
}

// 导完的包从待导入列表和记录里一起删掉
const removeImportedFile = (key) => {
  importDirFiles.value = importDirFiles.value.filter((entry) => (entry.relPath || entry.name) !== key)
  importFiles.value = importFiles.value.filter((entry) => (entry.webkitRelativePath || entry.name) !== key)
  const saved = importState.value
  if (!saved) {
    return
  }
  const done = saved.files.find((entry) => entryKey(entry) === key)
  if (!done) {
    return
  }
  saved.files = saved.files.filter((entry) => entryKey(entry) !== key)
  saved.done = (saved.done || 0) + 1
  importOkCount.value += 1
  persistImportState()
}

const markOuterFile = (key, ok) => {
  const target = stateFiles.value.find((entry) => entryKey(entry) === key)
  if (!target) {
    return
  }
  target.status = ok ? 'ok' : 'failed'
  persistImportState()
}

const retryImport = () => {
  if (!importTargets.value.length) {
    ElMessage.info('请重新选择上次的目录或需要重试的 zip，然后点「开始导入」')
    return
  }
  handleImport()
}

const handleImport = async () => {
  let targets = importTargets.value
  if (!targets.length) {
    ElMessage.warning('请先选择题目包目录或 zip 文件')
    return
  }
  if (importState.value && importHasRemaining.value) {
    const pending = new Map()
    stateFiles.value.forEach((entry) => {
      const key = entryKey(entry)
      pending.set(key, (pending.get(key) || 0) + 1)
    })
    const ignored = []
    const picked = []
    targets.forEach((file) => {
      const key = file.relPath || file.name
      const left = pending.get(key) || 0
      if (left > 0) {
        picked.push(file)
        pending.set(key, left - 1)
      } else {
        ignored.push(key)
      }
    })
    if (!picked.length) {
      ElMessage.warning('选中的包和记录里剩余的不一致，请重新选择上次的目录')
      return
    }
    if (ignored.length) {
      ElMessage.info(`已跳过 ${ignored.length} 个已经导入过的包`)
    }
    targets = picked
  } else {
    importState.value = {
      dir: importDirName.value || '',
      total: targets.length,
      done: 0,
      createdAt: new Date().toISOString(),
      files: targets.map((file) => ({
        name: file.name,
        relPath: file.relPath || '',
        status: 'pending'
      }))
    }
    importOkCount.value = 0
    importDone.value = 0
    importTotal.value = targets.length
    persistImportState()
  }

  importing.value = true
  importPhase.value = 'importing'
  importStopReason.value = ''
  importMessage.value = ''
  try {
    for (const file of targets) {
      const key = file.relPath || file.name
      const base = importDone.value
      let expanded = false
      const res = await importOne(file.raw || file, (task) => {
        if (!expanded && task?.total) {
          importTotal.value += Math.max(0, task.total - 1)
          expanded = true
        }
        const packageTotal = task?.package_total || 0
        if (packageTotal > 0) {
          const ratio = Math.min(1, (task?.package_done || 0) / packageTotal)
          importCurrentPercent.value = Math.max(importCurrentPercent.value, 30 + Math.round(ratio * 70))
        } else if (task?.message) {
          importCurrentPercent.value = Math.max(importCurrentPercent.value, 35)
        }
        if (task?.message) {
          importMessage.value = task.message
        }
      })
      const failures = res.items.filter((item) => !item.ok)
      if (res.container) {
        importDone.value = base + (res.total || res.items.length)
        importTotal.value = Math.max(importTotal.value, importDone.value)
        markOuterFile(key, failures.length === 0)
      } else {
        importDone.value = base + 1
      }
      if (failures.length) {
        const failedItem = failures[0]
        const target = stateFiles.value.find((entry) => entryKey(entry) === key)
        if (target) {
          target.status = 'failed'
          target.error = failedItem.error || '包内题目未通过校验'
        }
        importStopReason.value = `${file.name}：${failedItem.error || '包内题目未通过校验'}`
        persistImportState()
        break
      }
      removeImportedFile(key)
    }
    importCurrentName.value = ''
    if (importStopReason.value) {
      importPhase.value = 'failed'
      importMessage.value = ''
      ElMessage.error(`导入已停止：${importStopReason.value}`)
    } else {
      importPhase.value = 'done'
      importMessage.value = ''
      if (!importHasRemaining.value) {
        clearImportState()
      }
      ElMessage.success(`导入完成：成功 ${importOkCount.value} 个`)
    }
    load()
  } catch (e) {
    importCurrentName.value = ''
    importStopReason.value = e?.message || '导入失败，请重试'
    importPhase.value = 'failed'
    importMessage.value = ''
    ElMessage.error(`导入已停止：${importStopReason.value}`)
  } finally {
    importing.value = false
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

@media (max-width: 640px) {
  .sample-row {
    grid-template-columns: 1fr;
  }

  .sample-row__index {
    padding-top: 0;
  }
}
.dir-input {
  display: none;
}

.import-resume {
  margin-top: 8px;
}

.import-resume p {
  margin: 4px 0;
  font-size: 13px;
}

.import-queue {
  margin-top: 10px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 8px 10px;
  max-height: 220px;
  overflow: auto;
}

.import-queue__title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 6px;
}

.import-queue__row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  padding: 3px 0;
}

.import-queue__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.import-queue__error {
  color: var(--el-color-danger);
  font-size: 12px;
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.import-progress {
  margin-top: 10px;
}

.import-progress p {
  margin: 6px 0 0;
  font-size: 13px;
}
</style>
