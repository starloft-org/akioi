<template>
  <div v-loading="loading" class="problem-layout">
    <template v-if="problem">
      <div class="problem-pane panel">
        <div class="problem-statement">
          <h1 class="problem-title">
            <span class="mono">{{ problem.display_id }}</span>
            {{ problem.title }}
          </h1>
          <div class="problem-meta muted">
            <span>时间限制 {{ problem.time_limit_ms }} ms</span>
            <span>内存限制 {{ problem.memory_limit_mb }} MB</span>
            <span>积分 {{ problem.points }}</span>
            <DifficultyTag v-if="problem.difficulty" :difficulty="problem.difficulty" />
            <el-tag v-if="problem.contest_only" type="warning" size="small">比赛专用</el-tag>
            <span>通过率 {{ rateText(problem.acceptance_rate) }}</span>
            <span>提交 {{ problem.submit_count }} · 通过 {{ problem.accepted_count }}</span>
            <span v-if="problem.source">来源 {{ problem.source }}</span>
          </div>
          <div class="toolbar" style="margin-top: 16px">
            <el-button @click="router.push(`/problems/${problem.id}/solutions`)">查看题解</el-button>
            <el-button @click="router.push(`/discussions?problem_id=${problem.id}`)">相关讨论</el-button>
          </div>
          <div v-if="problem.tags && problem.tags.length" class="problem-tags">
            <el-tag v-for="tag in problem.tags" :key="tag" type="info" size="small">{{ tag }}</el-tag>
          </div>

          <h3>题目描述</h3>
          <div class="md-body" v-html="renderMarkdown(problem.description)"></div>
          <h3>输入格式</h3>
          <div class="md-body" v-html="renderMarkdown(problem.input_format)"></div>
          <h3>输出格式</h3>
          <div class="md-body" v-html="renderMarkdown(problem.output_format)"></div>

          <template v-if="problem.samples && problem.samples.length">
            <h3>样例</h3>
            <div v-for="(sample, index) in problem.samples" :key="index" class="sample">
              <table class="sample-table">
                <thead>
                  <tr>
                    <th>输入</th>
                    <th>输出</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td><pre class="pre-block">{{ sample.input }}</pre></td>
                    <td><pre class="pre-block">{{ sample.output }}</pre></td>
                  </tr>
                </tbody>
              </table>
              <div v-if="sample.explanation" class="muted sample-explain">说明：{{ sample.explanation }}</div>
            </div>
          </template>

          <template v-if="problem.hint">
            <h3>提示</h3>
            <div class="md-body" v-html="renderMarkdown(problem.hint)"></div>
          </template>
        </div>
      </div>

      <div class="submit-pane">
        <div class="panel">
          <h3 class="panel__title">提交代码</h3>
          <template v-if="store.isLogin">
            <div class="toolbar">
              <el-select v-model="language" style="width: 140px">
                <el-option v-for="item in languageOptions" :key="item.key" :label="item.name" :value="item.key" />
              </el-select>
              <span class="muted">按 Ctrl + Enter 可快速提交</span>
            </div>
            <CodeEditor v-model="code" :language="language" @submit="handleSubmit" />
            <div class="submit-bar">
              <el-button type="primary" :loading="submitting" @click="handleSubmit">提交</el-button>
            </div>
          </template>
          <div v-else class="submit-login">
            <span class="muted">请先登录后再提交代码</span>
            <el-button type="primary" @click="goLogin">登录</el-button>
          </div>
        </div>

        <div v-if="store.isLogin" class="panel">
          <h3 class="panel__title">我的提交记录</h3>
          <el-empty v-if="!myLoading && mySubmissions.length === 0" description="暂无提交记录" />
          <el-table v-else v-loading="myLoading" :data="mySubmissions" size="small">
            <el-table-column label="状态" width="140">
              <template #default="{ row }">
                <StatusTag :status="row.status" />
              </template>
            </el-table-column>
            <el-table-column label="得分" width="80" prop="score" />
            <el-table-column label="耗时" width="100">
              <template #default="{ row }">{{ formatDuration(row.time_ms) }}</template>
            </el-table-column>
            <el-table-column label="内存" width="100">
              <template #default="{ row }">{{ formatMemory(row.memory_kb) }}</template>
            </el-table-column>
            <el-table-column label="语言" width="90">
              <template #default="{ row }">{{ languageName(row.language) }}</template>
            </el-table-column>
            <el-table-column label="提交时间" min-width="160">
              <template #default="{ row }">
                <router-link :to="`/submissions/${row.id}`">{{ formatTime(row.created_at) }}</router-link>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createSubmission, getProblem, listSubmissions } from '@/api'
import CodeEditor from '@/components/CodeEditor.vue'
import DifficultyTag from '@/components/DifficultyTag.vue'
import StatusTag from '@/components/StatusTag.vue'
import { useUserStore } from '@/store/user'
import { LANGUAGES, formatDuration, formatMemory, formatTime, languageName } from '@/utils/format'
import { renderMarkdown } from '@/utils/markdown'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const problemId = computed(() => Number(route.params.id))
const loading = ref(true)
const problem = ref(null)
const languageOptions = ref(LANGUAGES)
const language = ref(LANGUAGES[0].key)
const code = ref('')
const submitting = ref(false)
const myLoading = ref(true)
const mySubmissions = ref([])

const rateText = (value) => `${Number(value || 0).toFixed(1)}%`

const goLogin = () => {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

const loadProblem = async () => {
  loading.value = true
  try {
    const data = await getProblem(problemId.value)
    problem.value = data.problem
    if (data.languages && data.languages.length) {
      languageOptions.value = data.languages
      language.value = data.languages[0].key
    }
  } catch (e) {
    problem.value = null
  } finally {
    loading.value = false
  }
}

const loadMySubmissions = async () => {
  if (!store.isLogin) {
    return
  }
  myLoading.value = true
  try {
    const data = await listSubmissions({ problem_id: problemId.value, page: 1, page_size: 10 })
    mySubmissions.value = data.items || []
  } catch (e) {
    mySubmissions.value = []
  } finally {
    myLoading.value = false
  }
}

const handleSubmit = async () => {
  if (submitting.value) {
    return
  }
  if (!code.value.trim()) {
    ElMessage.warning('代码不能为空')
    return
  }
  submitting.value = true
  try {
    const data = await createSubmission({
      problem_id: problemId.value,
      language: language.value,
      code: code.value
    })
    ElMessage.success('提交成功')
    router.push(`/submissions/${data.submission_id}`)
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadProblem()
  loadMySubmissions()
})
</script>

<style scoped>
/* 左右分栏：题面与编译器各自独立滚动 */
.problem-layout {
  display: flex;
  gap: 16px;
  align-items: stretch;
  /* 56px 顶部导航 + 上下各 16px 内边距 */
  height: calc(100vh - 56px - 32px);
  min-height: 480px;
}

.problem-pane {
  flex: 0 0 42%;
  overflow-y: auto;
  padding: 20px;
}

.submit-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  min-width: 0;
}

.submit-pane .panel {
  flex-shrink: 0;
}

.problem-title {
  margin: 0 0 8px;
  font-size: 20px;
}

.problem-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.problem-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}

.sample {
  margin-bottom: 12px;
}

.sample-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

.sample-table th,
.sample-table td {
  padding: 6px;
  border: 1px solid var(--akioi-border);
  vertical-align: top;
}

.sample-table th {
  background: #fafafa;
  font-weight: 600;
}

.sample-explain {
  margin-top: 4px;
  font-size: 13px;
}

.submit-bar {
  margin-top: 12px;
  text-align: right;
}

.submit-login {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 窄屏（移动端/小窗）时退回上下堆叠 */
@media (max-width: 960px) {
  .problem-layout {
    flex-direction: column;
    height: auto;
  }

  .problem-pane {
    flex: none;
    overflow-y: visible;
  }

  .submit-pane {
    overflow-y: visible;
  }
}
</style>
