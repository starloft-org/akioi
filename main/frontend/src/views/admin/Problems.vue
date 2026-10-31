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
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="goCreate">新建题目</el-button>
      <el-button @click="importVisible = true">导入题目</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="visibleItems" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="display_id" label="题号" width="100" />
      <el-table-column prop="bank_name" label="题库" width="140" />
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

    <el-dialog v-model="importVisible" title="导入题目" width="640px">
      <div class="import-bank">
        <span class="muted">导入到题库</span>
        <el-select v-model="importBankId" placeholder="默认题库" style="width: 240px">
          <el-option
            v-for="bank in banks"
            :key="bank.id"
            :label="`${bank.name}（${bank.prefix} + ${bank.code_length} 位）`"
            :value="bank.id"
          />
        </el-select>
      </div>
      <el-upload
        v-model:file-list="importFiles"
        drag
        multiple
        :auto-upload="false"
        accept=".zip"
      >
        <div class="el-upload__text">把题目包 zip 拖到这里（支持多选），或<em>点击选择</em></div>
      </el-upload>

      <div v-if="importResults.length" class="import-results">
        <p class="import-results__title">
          导入结果：成功 {{ resultsSummary.success }} 个，失败 {{ resultsSummary.failed }} 个
        </p>
        <div v-for="item in importResults" :key="item.filename" class="import-result-item">
          <span :class="item.ok ? 'result-ok' : 'result-fail'">{{ item.ok ? '✓' : '✗' }}</span>
          <span class="import-result__file">{{ item.filename }}</span>
          <span v-if="item.ok" class="muted">{{ item.display_id }} {{ item.title }}</span>
          <span v-else class="import-result__error">{{ item.error }}</span>
        </div>
      </div>

      <div class="import-hint">
        <p class="muted">每个 zip 一个题目，包内结构：</p>
        <pre class="pre-block">problem.md        题面，可直接用洛谷「复制 Markdown」的内容
config.json       可选：title / difficulty / time_limit_ms / memory_limit_mb
                  / points / tags / source / visibility
1.in  1.out       测试数据，可以有多组；都不带时只导入题面</pre>
        <p class="muted">没有 config.json 时，标题取题面的一级标题；题号由所选题库自动分配，包里不需要写题号。</p>
      </div>

      <template #footer>
        <el-button @click="closeImportDialog">关闭</el-button>
        <el-button type="primary" :loading="importing" :disabled="!importFiles.length" @click="handleImport">
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
  adminImportProblem,
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
const banks = ref([])
const importVisible = ref(false)
const importing = ref(false)
const importFiles = ref([])
const importBankId = ref('')
const importResults = ref([])
const resultsSummary = computed(() => {
  const success = importResults.value.filter((item) => item.ok).length
  return { success, failed: importResults.value.length - success }
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
      bank_id: bankFilter.value || undefined
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
    // 默认选中默认题库，导入时不会误落到别的题库
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

const handleImport = async () => {
  const raws = importFiles.value.map((item) => item.raw).filter(Boolean)
  if (!raws.length) {
    return
  }
  importing.value = true
  try {
    const data = await adminImportProblem(raws, importBankId.value || undefined)
    importResults.value = data?.items || []
    load()
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    importing.value = false
  }
}

const closeImportDialog = () => {
  importVisible.value = false
  importFiles.value = []
  importResults.value = []
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
