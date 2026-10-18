<template>
  <div v-loading="loading">
    <template v-if="contest">
      <div class="panel">
        <div class="contest-head">
          <h2 class="contest-title">{{ contest.title }}</h2>
          <div class="contest-actions">
            <el-button size="small" @click="router.push(`/contests/${contest.id}/rank`)">榜单</el-button>
            <el-button size="small" @click="router.push(`/contests/${contest.id}/submissions`)">评测记录</el-button>
            <el-button
              v-if="contest.register_required && !registered"
              type="primary"
              size="small"
              @click="handleRegister"
            >
              报名比赛
            </el-button>
          </div>
        </div>

        <el-descriptions :column="2" border>
          <el-descriptions-item label="状态">
            <el-tag :type="contestStatusType(contest.status)" size="small">
              {{ contestStatusText(contest.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="倒计时">{{ countdown || '-' }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ formatTime(contest.start_at) }}</el-descriptions-item>
          <el-descriptions-item label="结束时间">{{ formatTime(contest.end_at) }}</el-descriptions-item>
          <el-descriptions-item label="封榜时长">{{ contest.freeze_minutes }} 分钟</el-descriptions-item>
          <el-descriptions-item label="是否报名">
            <el-tag v-if="registered" type="success" size="small">已报名</el-tag>
            <span v-else class="muted">未报名</span>
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="contest.frozen" class="frozen-tip">已封榜，榜单不再更新</div>

        <div v-if="contest.description" class="pre-block contest-desc">{{ contest.description }}</div>
      </div>

      <div class="panel">
        <h3 class="panel__title">比赛题目</h3>
        <el-empty v-if="problems.length === 0" description="暂无题目" />
        <el-table v-else :data="problems" size="small">
          <el-table-column label="编号" width="80" prop="label" />
          <el-table-column label="题目" min-width="240">
            <template #default="{ row }">
              <router-link :to="`/problems/${row.problem_id}`">
                <span class="mono">{{ row.display_id }}</span> {{ row.title }}
              </router-link>
            </template>
          </el-table-column>
          <el-table-column label="难度" width="100">
            <template #default="{ row }">
              <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
            </template>
          </el-table-column>
          <el-table-column label="通过/提交" width="120">
            <template #default="{ row }">{{ row.accepted_count }} / {{ row.submit_count }}</template>
          </el-table-column>
          <el-table-column label="我的得分" width="120">
            <template #default="{ row }">{{ row.my_score }} / {{ row.full_score }}</template>
          </el-table-column>
        </el-table>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getContest, registerContest } from '@/api'
import { useUserStore } from '@/store/user'
import { contestStatusText, contestStatusType, formatTime } from '@/utils/format'
import DifficultyTag from '@/components/DifficultyTag.vue'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const contest = ref(null)
const problems = ref([])
const registered = ref(false)
const loading = ref(false)

const current = ref(Date.now())
let timer = null

const countdown = computed(() => {
  if (!contest.value) {
    return ''
  }
  const start = new Date(contest.value.start_at).getTime()
  const end = new Date(contest.value.end_at).getTime()
  let label = ''
  let remain = 0
  if (current.value < start) {
    label = '距离开始'
    remain = start - current.value
  } else if (current.value < end) {
    label = '距离结束'
    remain = end - current.value
  } else {
    return '已结束'
  }
  const total = Math.floor(remain / 1000)
  const h = String(Math.floor(total / 3600)).padStart(2, '0')
  const m = String(Math.floor((total % 3600) / 60)).padStart(2, '0')
  const s = String(total % 60).padStart(2, '0')
  return `${label} ${h}:${m}:${s}`
})

const load = async () => {
  loading.value = true
  try {
    const data = await getContest(route.params.id)
    contest.value = data.contest
    problems.value = data.problems || []
    registered.value = !!data.registered
    // 以服务端时间为基准校准倒计时
    const offset = new Date(data.contest.server_time).getTime() - Date.now()
    current.value = Date.now() + offset
  } catch (e) {
    contest.value = null
  } finally {
    loading.value = false
  }
}

const handleRegister = async () => {
  if (!store.isLogin) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  let password = ''
  if (contest.value?.needs_password) {
    try {
      const res = await ElMessageBox.prompt('请输入比赛口令', '报名比赛', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        inputType: 'password'
      })
      password = res.value || ''
    } catch (e) {
      return
    }
  }
  try {
    await registerContest(route.params.id, { password })
    ElMessage.success('报名成功')
    await load()
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

onMounted(() => {
  load()
  timer = setInterval(() => {
    current.value += 1000
  }, 1000)
})

onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
})
</script>

<style scoped>
.contest-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.contest-title {
  margin: 0;
  font-size: 20px;
}

.contest-actions {
  display: flex;
  gap: 8px;
}

.frozen-tip {
  margin-top: 12px;
  padding: 8px 12px;
  background: #fff7e6;
  border: 1px solid #ffd591;
  border-radius: var(--akioi-radius);
  color: #ad6800;
  font-size: 13px;
}

.contest-desc {
  margin-top: 12px;
}
</style>
