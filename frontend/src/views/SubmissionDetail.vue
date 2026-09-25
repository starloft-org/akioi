<template>
  <div>
    <div v-if="!store.isLogin" class="panel login-tip">
      <span class="muted">登录后可查看评测详情</span>
      <el-button type="primary" @click="goLogin">登录</el-button>
    </div>

    <template v-else>
      <div v-loading="loading" class="panel">
        <h3 class="panel__title">评测详情</h3>
        <template v-if="submission">
          <el-descriptions :column="3" border>
            <el-descriptions-item label="状态">
              <StatusTag :status="submission.status" />
            </el-descriptions-item>
            <el-descriptions-item label="得分">{{ submission.score }}</el-descriptions-item>
            <el-descriptions-item label="语言">{{ languageName(submission.language) }}</el-descriptions-item>
            <el-descriptions-item label="耗时">{{ formatDuration(submission.time_ms) }}</el-descriptions-item>
            <el-descriptions-item label="内存">{{ formatMemory(submission.memory_kb) }}</el-descriptions-item>
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
            <template #default="{ row }">{{ formatMemory(row.memory_kb) }}</template>
          </el-table-column>
          <el-table-column label="得分" width="80" prop="score" />
          <el-table-column label="提示" min-width="200">
            <template #default="{ row }">{{ row.message || '-' }}</template>
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
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
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

const submissionId = computed(() => route.params.id)

const goLogin = () => {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

const load = async () => {
  loading.value = true
  try {
    const data = await getSubmission(submissionId.value)
    submission.value = data.submission || null
    testcaseResults.value = data.testcase_results || []
    codeVisible.value = !!data.code_visible
    code.value = data.code || ''
    compileError.value = data.compile_error || ''
    judgeMessage.value = data.judge_message || ''
  } catch (e) {
    submission.value = null
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (store.isLogin) {
    load()
  }
})
</script>

<style scoped>
.login-tip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 0;
}

.section-title {
  margin: 16px 0 8px;
  font-size: 14px;
}
</style>
