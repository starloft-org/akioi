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
          <span v-if="isJudging" class="polling-hint">评测中，结果自动刷新</span>
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
import { getSubmission } from '@/api'
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

const submissionId = computed(() => route.params.id)

// 评测未结束时的轮询间隔
const POLL_INTERVAL_MS = 2000
let pollTimer = null

// 提交处于等待评测或评测中时，页面自动轮询结果，无需手动刷新
const isJudging = computed(() => {
  const status = submission.value?.status
  return status === 'Pending' || status === 'Judging'
})

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(async () => {
    await load(true)
    if (!isJudging.value) {
      stopPolling()
    }
  }, POLL_INTERVAL_MS)
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
</style>
