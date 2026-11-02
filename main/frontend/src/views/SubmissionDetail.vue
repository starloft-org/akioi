<template>
  <div>
    <div v-if="!store.isLogin" class="panel login-tip">
      <span class="muted">登录后可查看评测详情</span>
      <el-button type="primary" @click="goLogin">登录</el-button>
    </div>

    <template v-else>
      <div v-loading="loading" class="panel">
        <h3 class="panel__title">
          评测详情
          <span v-if="isJudging" class="polling-hint">{{ judgeHint }}</span>
        </h3>
        <template v-if="submission">
          <el-descriptions :column="3" border>
            <el-descriptions-item label="状态">
              <StatusTag :status="submission.status" />
            </el-descriptions-item>
            <el-descriptions-item label="得分">{{ submission.score }}</el-descriptions-item>
            <el-descriptions-item label="语言">{{ languageName(submission.language) }}</el-descriptions-item>
            <el-descriptions-item label="耗时">{{ formatDuration(submission.time_ms) }}</el-descriptions-item>
            <el-descriptions-item label="内存">{{ memoryText(submission.memory_kb) }}</el-descriptions-item>
            <el-descriptions-item label="代码长度">{{ submission.code_length }} B</el-descriptions-item>
            <el-descriptions-item label="题目">
              <router-link :to="`/problems/${submission.problem_id}`">
                {{ submission.problem_display_id }} {{ submission.problem_title }}
              </router-link>
            </el-descriptions-item>
            <el-descriptions-item label="提交时间">{{ formatTime(submission.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="评测时间">{{ formatTime(submission.judged_at) }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="isJudging" class="judge-progress">
            <el-progress
              :percentage="progressPercent"
              :stroke-width="10"
              :text-inside="true"
              :status="progressTotal > 0 ? undefined : 'warning'"
            />
          </div>

          <template v-if="judgeMessage">
            <h4 class="section-title">评测信息</h4>
            <div class="pre-block">{{ judgeMessage }}</div>
          </template>
        </template>
      </div>

      <div v-if="submission" class="panel">
        <h3 class="panel__title">测试点结果</h3>
        <el-empty v-if="testcaseResults.length === 0" description="暂无测试点结果" />
        <el-table v-else :data="testcaseResults" size="small">
          <el-table-column label="测试点" width="90">
            <template #default="{ row }">#{{ row.index }}</template>
          </el-table-column>
          <el-table-column label="状态" width="150">
            <template #default="{ row }">
              <StatusTag :status="row.status" />
            </template>
          </el-table-column>
          <el-table-column label="耗时" width="100">
            <template #default="{ row }">{{ formatDuration(row.time_ms) }}</template>
          </el-table-column>
          <el-table-column label="内存" width="100">
            <template #default="{ row }">{{ memoryText(row.memory_kb) }}</template>
          </el-table-column>
          <el-table-column label="得分" width="80" prop="score" />
          <el-table-column label="提示" min-width="260">
            <template #default="{ row }">
              <div class="hint-cell">
                <span class="hint-text">{{ row.message || '-' }}</span>
                <el-button v-if="hasDetail(row)" link type="primary" size="small" @click="openDetail(row)">
                  详情
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div v-if="submission" class="panel">
        <h3 class="panel__title">源代码</h3>
        <template v-if="compileError">
          <h4 class="section-title">编译错误</h4>
          <div class="pre-block">{{ compileError }}</div>
        </template>
        <CodeEditor v-if="codeVisible" :model-value="code" readonly />
        <el-empty v-else description="比赛进行中，暂不能查看源码或他人代码" />
      </div>

      <!-- 单个测试点的完整错误信息，表格里只放摘要 -->
      <el-dialog v-model="detailVisible" :title="`测试点 #${detailRow?.index ?? ''} 详情`" width="640px">
        <template v-if="detailRow">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="状态">
              <StatusTag :status="detailRow.status" />
            </el-descriptions-item>
            <el-descriptions-item label="退出码">{{ detailRow.exit_code ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="耗时">{{ formatDuration(detailRow.time_ms) }}</el-descriptions-item>
            <el-descriptions-item label="内存">{{ memoryText(detailRow.memory_kb) }}</el-descriptions-item>
          </el-descriptions>
          <h4 class="section-title">提示</h4>
          <div class="pre-block">{{ detailRow.message || '-' }}</div>
          <template v-if="detailRow.stderr">
            <h4 class="section-title">错误输出（stderr）</h4>
            <div class="pre-block stderr-block">{{ detailRow.stderr }}</div>
          </template>
        </template>
        <template #footer>
          <el-button @click="detailVisible = false">关闭</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getSubmission, getSubmissionStatus } from '@/api'
import CodeEditor from '@/components/CodeEditor.vue'
import StatusTag from '@/components/StatusTag.vue'
import { useUserStore } from '@/store/user'
import { formatDuration, formatMemory, formatTime, languageName } from '@/utils/format'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submission = ref(null)
const testcaseResults = ref([])
const codeVisible = ref(false)
const code = ref('')
const compileError = ref('')
const judgeMessage = ref('')
const detailVisible = ref(false)
const detailRow = ref(null)

// 评测进度：已测测试点、是否在排队、已等待秒数
const progressDone = ref(0)
const progressTotal = ref(0)
const pendingAhead = ref(0)
const waitingSeconds = ref(0)

const submissionId = computed(() => route.params.id)

// 刚提交时轮询得勤一些，超过 30 秒后退到低频，避免长时间占着请求
const POLL_FAST_MS = 700
const POLL_SLOW_MS = 1500
const POLL_FAST_WINDOW_MS = 30000
let pollTimer = null
let pollStartedAt = 0

// 提交处于等待评测或评测中时，页面自动轮询结果，无需手动刷新
const isJudging = computed(() => {
  const status = submission.value?.status
  return status === 'Pending' || status === 'Judging'
})

const progressPercent = computed(() => {
  if (!isJudging.value) {
    return 100
  }
  if (progressTotal.value > 0) {
    return Math.min(99, Math.round((progressDone.value / progressTotal.value) * 100))
  }
  return 5
})

const judgeHint = computed(() => {
  if (!isJudging.value) {
    return ''
  }
  const seconds = waitingSeconds.value
  const judging = submission.value?.status === 'Judging' || progressTotal.value > 0
  if (judging) {
    return progressTotal.value > 0
      ? `评测中：已测 ${progressDone.value} / ${progressTotal.value} 个测试点，已等待 ${seconds} 秒`
      : `评测中，已等待 ${seconds} 秒`
  }
  return pendingAhead.value > 0
    ? `排队中：前面还有 ${pendingAhead.value} 份提交，已等待 ${seconds} 秒`
    : `等待评测，已等待 ${seconds} 秒`
})

const stopPolling = () => {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
}

// 轮询轻量状态接口，状态变化后再拉一次详情，避免每次都拿源码和大段测试点信息
const poll = async () => {
  pollTimer = null
  try {
    const status = await getSubmissionStatus(submissionId.value)
    if (submission.value && status) {
      submission.value.status = status.status
      submission.value.score = status.score
      submission.value.time_ms = status.time_ms
      submission.value.memory_kb = status.memory_kb
      submission.value.judged_at = status.judged_at
    }
    progressDone.value = status?.progress?.done ?? 0
    progressTotal.value = status?.progress?.total ?? 0
    pendingAhead.value = status?.pending_ahead ?? 0
    waitingSeconds.value = Math.max(0, Math.round((Date.now() - pollStartedAt) / 1000))
    if (status.status !== 'Pending' && status.status !== 'Judging') {
      await load(true)
      return
    }
  } catch (e) {
    // 轮询失败（断网、后端重启）不打断页面，下一轮继续
  }
  const elapsed = Date.now() - pollStartedAt
  pollTimer = setTimeout(poll, elapsed < POLL_FAST_WINDOW_MS ? POLL_FAST_MS : POLL_SLOW_MS)
}

const startPolling = () => {
  stopPolling()
  pollStartedAt = Date.now()
  waitingSeconds.value = 0
  poll()
}

const goLogin = () => {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

// 内存采集失败时显示为 —，避免把「未知」误读成 0KB
const memoryText = (kb) => (kb > 0 ? formatMemory(kb) : '—')

// 运行时错误、系统错误或带 stderr 的测试点才值得展开完整信息
const hasDetail = (row) =>
  !!row.stderr || !!row.exit_code || row.status === 'RuntimeError' || row.status === 'SystemError'

const openDetail = (row) => {
  detailRow.value = row
  detailVisible.value = true
}

// silent 为 true 时不显示 loading 遮罩，避免轮询期间页面闪烁
const load = async (silent = false) => {
  if (!silent) {
    loading.value = true
  }
  try {
    const data = await getSubmission(submissionId.value)
    submission.value = data.submission || null
    testcaseResults.value = data.testcase_results || []
    codeVisible.value = !!data.code_visible
    code.value = data.code || ''
    compileError.value = data.compile_error || ''
    judgeMessage.value = data.judge_message || ''
  } catch (e) {
    if (!silent) {
      submission.value = null
    }
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

onMounted(async () => {
  if (!store.isLogin) {
    return
  }
  await load()
  if (isJudging.value) {
    startPolling()
  }
})

onUnmounted(stopPolling)
</script>

<style scoped>
.login-tip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 0;
}

/* 评测中的自动刷新提示，带一个呼吸小圆点 */
.polling-hint {
  margin-left: 8px;
  font-size: 12px;
  font-weight: 400;
  color: var(--akioi-primary);
}

.polling-hint::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 6px;
  border-radius: 50%;
  background: var(--akioi-primary);
  animation: poll-pulse 1.2s ease-in-out infinite;
}

@keyframes poll-pulse {
  0%,
  100% {
    opacity: 0.3;
  }
  50% {
    opacity: 1;
  }
}

.section-title {
  margin: 16px 0 8px;
  font-size: 14px;
}

.hint-cell {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

/* 表格里只展示前两行摘要，完整内容放在详情弹窗 */
.hint-text {
  display: -webkit-box;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: pre-wrap;
  word-break: break-word;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.stderr-block {
  max-height: 320px;
  overflow: auto;
}

/* 评测中的进度条与说明文字 */
.judge-progress {
  margin-top: 12px;
}

.judge-progress__text {
  margin-top: 6px;
  font-size: 12px;
  color: var(--akioi-primary);
}
</style>
