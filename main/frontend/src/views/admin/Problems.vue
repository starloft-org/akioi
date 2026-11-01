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
      <el-select v-model="bankFilter" placeholder="题库" clearable style="width: 160px" @change="search">
        <el-option v-for="bank in banks" :key="bank.id" :label="bank.name" :value="bank.id" />
      </el-select>
      <el-select v-model="kindFilter" placeholder="归属" clearable style="width: 130px" @change="search">
        <el-option label="公共题目" value="public" />
        <el-option label="团队题目" value="team" />
        <el-option label="个人题目" value="personal" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="goCreate">新建题目</el-button>
      <el-button @click="openImport">导入题目</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="visibleItems" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="display_id" label="题号" width="100" />
      <el-table-column prop="bank_name" label="题库" width="140" />
      <el-table-column label="归属" width="200">
        <template #default="{ row }">
          <el-tag size="small" :type="kindTagType(row.kind)">{{ kindText(row.kind) }}</el-tag>
          <span v-if="row.kind !== 'public'" class="muted" style="margin-left: 6px">{{ ownerText(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="访问权限" width="110">
        <template #default="{ row }">
          <el-tag size="small" :type="row.access === 1 ? 'success' : 'info'">
            {{ row.access === 1 ? '公开' : '受限' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="title" label="标题" min-width="180" />
      <el-table-column label="难度" width="140">
        <template #default="{ row }">
          <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="points" label="积分" width="80" />
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
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="router.push(`/admin/problems/${row.id}/edit`)">编辑</el-button>
          <el-button size="small" @click="router.push(`/admin/problems/${row.id}/testdata`)">测试数据</el-button>
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

    <el-dialog
      v-model="importVisible"
      title="导入题目"
      width="640px"
      :close-on-click-modal="!importing"
      :close-on-press-escape="!importing"
      :show-close="!importing"
    >
      <div class="import-bank">
        <span class="muted">导入到题库</span>
        <el-select v-model="importBankId" placeholder="选择题库" style="width: 240px">
          <el-option
            v-for="bank in banks"
            :key="bank.id"
            :label="`${bank.name}（${bank.prefix} + ${bank.code_length} 位）`"
            :value="bank.id"
          />
        </el-select>
      </div>
      <el-alert
        v-if="importHasRemaining && !importing"
        type="warning"
        :closable="false"
        class="import-resume"
        title="上次导入还没有完成"
      >
        <p>
          共 {{ importTotal || stateFiles.length }} 个包，已完成 {{ stateOkCount }} 个，还剩
          {{ importRemaining.length }} 个未导入。
        </p>
        <p class="import-resume__names">剩余：{{ remainingPreview }}</p>
        <p>重新选择这些 zip 后点「开始导入」，已成功的包会被跳过，失败过的包会重试。</p>
        <el-button size="small" @click="discardImportRecord">丢弃记录</el-button>
      </el-alert>

      <el-upload
        v-model:file-list="importFiles"
        drag
        multiple
        :auto-upload="false"
        accept=".zip"
      >
        <div class="el-upload__text">把题目包 zip 拖到这里（支持多选），或<em>点击选择</em></div>
      </el-upload>

      <div v-if="importPhase !== 'idle'" class="import-progress">
        <el-progress
          :percentage="importPercent"
          :status="importPhase === 'done' ? 'success' : ''"
        />
        <p class="muted">
          <template v-if="importPhase === 'importing'">
            正在导入 {{ importDone }} / {{ importTotal }} 题，成功 {{ resultsSummary.success }} 个，失败
            {{ resultsSummary.failed }} 个
          </template>
          <template v-else>
            导入完成：成功 {{ resultsSummary.success }} 个，失败 {{ resultsSummary.failed }} 个
          </template>
        </p>
      </div>

      <div v-if="importResults.length" class="import-results">
        <p class="import-results__title">
          导入结果：成功 {{ resultsSummary.success }} 个，失败 {{ resultsSummary.failed }} 个
        </p>
        <div v-for="item in importResultView" :key="item.filename" class="import-result-item">
          <span :class="item.ok ? 'result-ok' : 'result-fail'">{{ item.ok ? '✓' : '✗' }}</span>
          <span class="import-result__file">{{ item.filename }}</span>
          <span v-if="item.ok" class="muted">{{ item.display_id }} {{ item.title }}</span>
          <span v-else class="import-result__error">{{ item.error }}</span>
        </div>
        <p v-if="importResults.length > importResultView.length" class="muted">
          仅显示前 {{ importResultView.length }} 条，共 {{ importResults.length }} 条
        </p>
      </div>

      <div class="import-hint">
        <p class="muted">
          导入在服务器后台执行，提交后关闭弹窗或浏览器也会继续，题目会逐包出现在结果里。
        </p>
        <p class="muted">每个 zip 一个题目，包内结构：</p>
        <pre class="pre-block">problem.md        题面，可直接用洛谷「复制 Markdown」的内容
config.json       可选：title / difficulty / time_limit_ms / memory_limit_mb
                  / points / tags / source / visibility
1.in  1.out       测试数据，可以有多组；都不带时只导入题面</pre>
        <p class="muted">没有 config.json 时，标题取题面的一级标题；题号由所选题库自动分配，包里不需要写题号。</p>
      </div>

      <template #footer>
        <el-button :disabled="importing" @click="closeImportDialog">关闭</el-button>
        <el-button
          type="primary"
          :loading="importing"
          :disabled="!importFiles.length || importing"
          @click="handleImport"
        >
          开始导入
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  adminDeleteProblem,
  adminImportProblemOne,
  adminListProblemBanks,
  adminListProblems
} from '@/api'
import DifficultyTag from '@/components/DifficultyTag.vue'

const router = useRouter()
const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const visibilityFilter = ref('')
const bankFilter = ref('')
const kindFilter = ref('')
const banks = ref([])
const importVisible = ref(false)
const importing = ref(false)
const importFiles = ref([])
const importBankId = ref('')
const importResults = ref([])
// 持久化的批次记录：{ bankId, total, files: [{ name, status, display_id, title, error }] }
const importState = ref(null)
// idle：未开始；importing：正在逐包导入；done：完成
// 一次选中的包会在前端拆成多个请求，每个请求只传一个 zip，避免单个请求体过大
const importPhase = ref('idle')
const importDone = ref(0)
const importTotal = ref(0)
const resultsSummary = computed(() => {
  const success = importResults.value.filter((item) => item.ok).length
  return { success, failed: importResults.value.length - success }
})
// 结果最多渲染前 50 条，失败项排在前面，避免一千题时列表卡顿
const importResultView = computed(() => {
  const failed = importResults.value.filter((item) => !item.ok)
  const ok = importResults.value.filter((item) => item.ok)
  return [...failed, ...ok].slice(0, 50)
})
const importPercent = computed(() => {
  if (!importTotal.value) {
    return 0
  }
  return Math.min(100, Math.round((importDone.value / importTotal.value) * 100))
})

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
      keyword: keyword.value || undefined,
      bank_id: bankFilter.value || undefined,
      kind: kindFilter.value || undefined
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

const loadBanks = async () => {
  try {
    const data = await adminListProblemBanks()
    banks.value = data.items || []
    // 默认选中第一个题库（主库），导入时不会误落到别的题库
    if (!importBankId.value && banks.value.length) {
      importBankId.value = banks.value[0].id
    }
  } catch (e) {
    banks.value = []
  }
}

const search = () => {
  page.value = 1
  load()
}

const kindText = (kind) => {
  if (kind === 'team') return '团队题目'
  if (kind === 'personal') return '个人题目'
  return '公共题目'
}

const kindTagType = (kind) => {
  if (kind === 'team') return 'warning'
  if (kind === 'personal') return 'success'
  return 'info'
}

const ownerText = (row) => {
  if (row.kind === 'team') return row.team_name || `团队 #${row.team_id}`
  return row.owner_name || `用户 #${row.owner_id}`
}

// 新建题目时带上当前筛选的题库，避免建到别的题库里
const goCreate = () => {
  router.push({
    path: '/admin/problems/new',
    query: bankFilter.value ? { bank_id: bankFilter.value } : {}
  })
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

// 每包一个请求，最多三个并发；单个包失败不影响其余包
const IMPORT_CONCURRENCY = 3

// 导入批次记录保存在 localStorage，刷新或断网后能看出哪些包还没导入
const IMPORT_STATE_KEY = 'akioi:admin-import-state'

const stateFiles = computed(() => importState.value?.files || [])
const stateOkCount = computed(() => stateFiles.value.filter((item) => item.status === 'ok').length)
const importRemaining = computed(() => stateFiles.value.filter((item) => item.status !== 'ok'))
const importHasRemaining = computed(() => importRemaining.value.length > 0)
const remainingPreview = computed(() => {
  const names = importRemaining.value.map((item) => item.name)
  const head = names.slice(0, 5).join('、')
  return names.length > 5 ? `${head} 等 ${names.length} 个` : head
})

const loadImportState = () => {
  try {
    const raw = localStorage.getItem(IMPORT_STATE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw)
    if (!parsed || !Array.isArray(parsed.files) || !parsed.files.length) {
      return null
    }
    return parsed
  } catch (e) {
    return null
  }
}

const persistImportState = () => {
  try {
    if (importState.value) {
      localStorage.setItem(IMPORT_STATE_KEY, JSON.stringify(importState.value))
    }
  } catch (e) {
    // 存储不可用时只影响刷新后的续传提示
  }
}

const clearImportState = () => {
  try {
    localStorage.removeItem(IMPORT_STATE_KEY)
  } catch (e) {
    // 忽略存储异常
  }
  importState.value = null
}

// 从批次记录恢复弹窗里的结果列表与进度
const restoreImportState = (saved) => {
  importState.value = saved
  const processed = saved.files.filter((item) => item.status !== 'pending')
  importResults.value = processed.map((item) => ({
    filename: item.name,
    ok: item.status === 'ok',
    display_id: item.display_id,
    title: item.title,
    error: item.error
  }))
  importDone.value = processed.length
  importTotal.value = saved.total || saved.files.length
  importPhase.value = importRemaining.value.length ? 'idle' : 'done'
}

const openImport = () => {
  const saved = loadImportState()
  if (saved) {
    restoreImportState(saved)
    if (saved.bankId && banks.value.some((bank) => bank.id === saved.bankId)) {
      importBankId.value = saved.bankId
    }
  } else {
    importState.value = null
  }
  importVisible.value = true
}

const recordImportResult = (fileName, item) => {
  // 重试同一个包时替换掉上一条失败记录，避免结果里出现重复条目
  const previous = importResults.value.findIndex((entry) => entry.filename === fileName && !entry.ok)
  if (previous >= 0) {
    importResults.value.splice(previous, 1, item)
  } else {
    importResults.value.push(item)
  }
  const target = stateFiles.value.find((entry) => entry.name === fileName && entry.status !== 'ok')
  if (target) {
    target.status = item.ok ? 'ok' : 'failed'
    target.display_id = item.display_id
    target.title = item.title
    target.error = item.error
  }
  importDone.value = stateFiles.value.filter((entry) => entry.status !== 'pending').length
  persistImportState()
}

const discardImportRecord = async () => {
  try {
    await ElMessageBox.confirm(
      '丢弃记录后需要重新选择全部题目包（已入库的题目不受影响）。',
      '丢弃导入记录',
      { type: 'warning' }
    )
  } catch (e) {
    return
  }
  clearImportState()
  importResults.value = []
  importDone.value = 0
  importTotal.value = 0
  importPhase.value = 'idle'
  ElMessage.success('已丢弃导入记录')
}

const importOne = async (file) => {
  try {
    const data = await adminImportProblemOne(file, importBankId.value || undefined)
    const item = (data?.items || [])[0]
    if (item) {
      return item
    }
    return { filename: file.name, ok: false, error: '导入没有返回结果' }
  } catch (e) {
    return {
      filename: file.name,
      ok: false,
      error: e?.response?.data?.message || e?.message || '导入失败'
    }
  }
}

const handleImport = async () => {
  const raws = importFiles.value.map((item) => item.raw).filter(Boolean)
  if (!raws.length) {
    ElMessage.warning('请先选择要导入的题目包')
    return
  }

  let targets = raws
  if (importHasRemaining.value) {
    // 续传：只导入上次记录里还没成功的包，按文件名逐个匹配
    const pending = new Map()
    importRemaining.value.forEach((entry) => {
      pending.set(entry.name, (pending.get(entry.name) || 0) + 1)
    })
    const ignored = []
    targets = []
    raws.forEach((file) => {
      const left = pending.get(file.name) || 0
      if (left > 0) {
        targets.push(file)
        pending.set(file.name, left - 1)
      } else {
        ignored.push(file.name)
      }
    })
    if (!targets.length) {
      ElMessage.warning('选中的包与记录的剩余文件不一致，请重新选择剩余的 zip')
      return
    }
    if (ignored.length) {
      ElMessage.info(`已跳过 ${ignored.length} 个不在待导入列表中的包`)
    }
  } else {
    importState.value = {
      bankId: importBankId.value || null,
      total: raws.length,
      createdAt: new Date().toISOString(),
      files: raws.map((file) => ({ name: file.name, status: 'pending' }))
    }
    importResults.value = []
    importDone.value = 0
    importTotal.value = raws.length
    persistImportState()
  }

  importing.value = true
  importPhase.value = 'importing'
  const queue = [...targets]
  const worker = async () => {
    while (queue.length) {
      const file = queue.shift()
      const item = await importOne(file)
      recordImportResult(file.name, item)
    }
  }
  try {
    await Promise.all(
      Array.from({ length: Math.min(IMPORT_CONCURRENCY, targets.length) }, () => worker())
    )
    const remaining = importRemaining.value.length
    importPhase.value = remaining ? 'idle' : 'done'
    if (remaining) {
      ElMessage.warning(`还剩 ${remaining} 个包未导入，记录已保存，重新选择这些 zip 后可以继续`)
    } else {
      const success = stateOkCount.value
      ElMessage.success(`导入完成：成功 ${success} 个，失败 ${importTotal.value - success} 个`)
    }
    load()
  } finally {
    importing.value = false
  }
}

const closeImportDialog = () => {
  importVisible.value = false
  importFiles.value = []
  if (importHasRemaining.value) {
    // 还差包没导，保留记录，下次打开继续
    return
  }
  clearImportState()
  importResults.value = []
  importPhase.value = 'idle'
  importDone.value = 0
  importTotal.value = 0
}

onMounted(() => {
  loadBanks()
  load()
})
</script>

<style scoped>
.import-bank {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.import-hint {
  margin-top: 16px;
}

.import-resume {
  margin-bottom: 12px;
}

.import-resume p {
  margin: 0 0 6px;
  font-size: 12px;
  line-height: 1.6;
}

.import-resume__names {
  word-break: break-all;
  color: var(--akioi-text-weak);
}

.import-progress {
  margin-top: 12px;
}

.import-progress p {
  margin: 6px 0 0;
  font-size: 12px;
}

.import-hint p {
  margin: 0 0 6px;
  font-size: 12px;
}

.import-hint .pre-block {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.7;
}

.import-results {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid var(--akioi-border);
  border-radius: 6px;
  background: #fafbfc;
}

.import-results__title {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
}

.import-result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 0;
  font-size: 13px;
}

.import-result__file {
  min-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-ok {
  color: #00b42a;
  font-weight: 600;
}

.result-fail,
.import-result__error {
  color: #f53f3f;
}
</style>
