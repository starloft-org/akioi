<template>
  <div class="home">
    <el-row :gutter="16">
      <el-col :xs="24" :sm="24" :lg="16">
        <section class="home-hero">
          <div class="home-hero__body">
            <p class="home-hero__eyebrow">{{ store.site.app_name }} · Online Judge</p>
            <h1 class="home-hero__title">{{ heroTitle }}</h1>
            <p class="home-hero__desc">刷题、参加比赛、写题解、和同伴讨论，一切从这里开始。</p>
            <div class="home-hero__actions">
              <el-button type="primary" @click="router.push('/problems')">开始刷题</el-button>
              <el-button v-if="!store.isLogin" text @click="router.push('/login')">登录</el-button>
              <el-button v-else text @click="router.push('/submissions')">我的评测</el-button>
              <el-button
                v-if="!store.isLogin && store.site.register_enabled"
                text
                @click="router.push('/register')"
              >
                注册
              </el-button>
            </div>
          </div>
          <dl class="home-hero__stats">
            <div class="home-hero__stat">
              <dt>{{ stats.problems }}</dt>
              <dd>道题目</dd>
            </div>
            <div class="home-hero__stat">
              <dt>{{ stats.contests }}</dt>
              <dd>场比赛</dd>
            </div>
            <div class="home-hero__stat">
              <dt>{{ stats.users }}</dt>
              <dd>人上榜</dd>
            </div>
          </dl>
        </section>

        <section v-loading="loading.announcement" class="panel">
          <div class="panel-head">
            <h3 class="panel__title">公告</h3>
          </div>
          <el-empty
            v-if="!loading.announcement && announcements.length === 0"
            description="暂无公告"
            :image-size="72"
          />
          <ul v-else class="notice-list">
            <li v-for="item in announcements.slice(0, 6)" :key="item.id">
              <el-link type="primary" :underline="false" @click="openAnnouncement(item)">
                {{ item.title }}
              </el-link>
              <span class="muted">{{ fromNow(item.created_at) }}</span>
            </li>
          </ul>
        </section>

        <section v-loading="loading.problem" class="panel">
          <div class="panel-head">
            <h3 class="panel__title">最新题目</h3>
            <el-link type="primary" :underline="false" @click="router.push('/problems')">
              查看全部
            </el-link>
          </div>
          <el-empty
            v-if="!loading.problem && problems.length === 0"
            description="暂无题目"
            :image-size="72"
          />
          <el-table v-else :data="problems" size="small">
            <el-table-column label="题号" width="96">
              <template #default="{ row }">
                <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
              </template>
            </el-table-column>
            <el-table-column label="标题" min-width="200">
              <template #default="{ row }">
                <router-link :to="`/problems/${row.id}`">{{ row.title }}</router-link>
              </template>
            </el-table-column>
            <el-table-column label="难度" width="96">
              <template #default="{ row }">
                <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
                <span v-else class="muted">—</span>
              </template>
            </el-table-column>
            <el-table-column label="上传" width="104">
              <template #default="{ row }">
                <span class="muted">{{ fromNow(row.created_at) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="通过率" width="90" align="right">
              <template #default="{ row }">
                <span class="muted">{{ acRate(row) }}</span>
              </template>
            </el-table-column>
          </el-table>
        </section>
      </el-col>

      <el-col :xs="24" :sm="24" :lg="8" class="home-aside">
        <section v-loading="loading.contest" class="panel">
          <div class="panel-head">
            <h3 class="panel__title">最近比赛</h3>
            <el-link type="primary" :underline="false" @click="router.push('/contests')">
              全部
            </el-link>
          </div>
          <el-empty
            v-if="!loading.contest && contests.length === 0"
            description="暂无比赛"
            :image-size="72"
          />
          <div v-else class="contest-list">
            <router-link
              v-for="item in contests"
              :key="item.id"
              :to="`/contests/${item.id}`"
              class="contest-card"
              :class="`contest-card--${item.status}`"
            >
              <div class="contest-card__head">
                <span class="contest-card__title">{{ item.title }}</span>
                <el-tag :type="contestStatusType(item.status)" size="small">
                  {{ contestStatusText(item.status) }}
                </el-tag>
              </div>
              <p class="contest-card__meta">
                {{ formatMonthDay(item.start_at) }} 开赛 · {{ durationText(item) }}
              </p>
              <p class="contest-card__meta muted">
                {{ item.problem_count || 0 }} 题 · {{ item.participant_count || 0 }} 人参加
              </p>
            </router-link>
          </div>
        </section>

        <section v-loading="loading.ranking" class="panel">
          <div class="panel-head">
            <h3 class="panel__title">积分榜</h3>
            <el-link type="primary" :underline="false" @click="router.push('/ranking')">
              全部
            </el-link>
          </div>
          <el-empty
            v-if="!loading.ranking && rankings.length === 0"
            description="暂无上榜用户"
            :image-size="72"
          />
          <ol v-else class="rank-list">
            <li v-for="(item, index) in rankings" :key="item.user_id" class="rank-item">
              <span class="rank-no" :class="rankClass(index)">{{ index + 1 }}</span>
              <router-link :to="`/users/${item.user_id}`" class="rank-name">
                {{ item.nickname || item.username }}
              </router-link>
              <span class="rank-points">{{ formatPoints(item.points) }}</span>
            </li>
          </ol>
        </section>
      </el-col>
    </el-row>

    <el-dialog v-model="dialogVisible" :title="current?.title" width="720px">
      <div class="md-body announcement-body" v-html="mdRender ? mdRender(current?.content) : ''"></div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getAnnouncements, listContests, listProblems, listRankings } from '@/api'
import { useUserStore } from '@/store/user'
import { contestStatusText, contestStatusType, formatTime } from '@/utils/format'
import DifficultyTag from '@/components/DifficultyTag.vue'

const store = useUserStore()
const router = useRouter()

const announcements = ref([])
const contests = ref([])
const problems = ref([])
const rankings = ref([])
const loading = reactive({ announcement: true, contest: true, problem: true, ranking: true })
const stats = reactive({ problems: 0, contests: 0, users: 0 })
const dialogVisible = ref(false)
const current = ref(null)

const heroTitle = computed(() => {
  if (!store.isLogin) {
    return `欢迎来到 ${store.site.app_name}`
  }
  return `欢迎回来，${store.user?.nickname || store.user?.username || ''}`
})

const pad = (n) => String(n).padStart(2, '0')

const formatMonthDay = (value) => {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return '-'
  }
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const fromNow = (value) => {
  const t = new Date(value).getTime()
  if (Number.isNaN(t)) {
    return '-'
  }
  const diff = Date.now() - t
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  if (diff < 2_592_000_000) return `${Math.floor(diff / 86_400_000)} 天前`
  return formatTime(value).slice(0, 10)
}

const durationText = (row) => {
  const ms = new Date(row.end_at).getTime() - new Date(row.start_at).getTime()
  if (!Number.isFinite(ms) || ms <= 0) {
    return '-'
  }
  const minutes = ms / 60_000
  if (minutes < 60) {
    return `${Math.round(minutes)} 分钟`
  }
  const hours = minutes / 60
  if (hours < 48) {
    return `${Math.round(hours * 10) / 10} 小时`
  }
  return `${Math.round(hours / 24)} 天`
}

const acRate = (row) => {
  const total = row.submit_count || 0
  if (!total) {
    return '—'
  }
  return `${Math.round(((row.accepted_count || 0) / total) * 100)}%`
}

const formatPoints = (value) => {
  const n = Number(value || 0)
  return Number.isInteger(n) ? String(n) : n.toFixed(2)
}

const rankClass = (index) => (index < 3 ? `rank-no--${index + 1}` : '')

const mdRender = ref(null)

// 公告正文可能带公式，markdown 渲染器按需加载，避免拖慢首页首屏。
const openAnnouncement = async (item) => {
  if (!mdRender.value) {
    const mod = await import('@/utils/markdown')
    mdRender.value = mod.renderMarkdown
  }
  current.value = item
  dialogVisible.value = true
}

const loadAnnouncements = async () => {
  try {
    const data = await getAnnouncements()
    announcements.value = data.items || []
  } catch (e) {
    announcements.value = []
  } finally {
    loading.announcement = false
  }
}

const loadContests = async () => {
  try {
    const data = await listContests({ page: 1, page_size: 8 })
    contests.value = (data.items || [])
      .slice()
      .sort((a, b) => statusWeight(a.status) - statusWeight(b.status))
      .slice(0, 4)
    stats.contests = data.total || contests.value.length
  } catch (e) {
    contests.value = []
  } finally {
    loading.contest = false
  }
}

const statusWeight = (status) => ({ running: 0, pending: 1, ended: 2 }[status] ?? 3)

const loadProblems = async () => {
  try {
    const data = await listProblems({ page: 1, page_size: 8, sort: 'latest' })
    problems.value = data.items || []
    stats.problems = data.total || problems.value.length
  } catch (e) {
    problems.value = []
  } finally {
    loading.problem = false
  }
}

const loadRankings = async () => {
  try {
    const data = await listRankings({ page: 1, page_size: 8 })
    rankings.value = data.items || []
    stats.users = data.total || rankings.value.length
  } catch (e) {
    rankings.value = []
  } finally {
    loading.ranking = false
  }
}

onMounted(() => {
  loadAnnouncements()
  loadContests()
  loadProblems()
  loadRankings()
})
</script>

<style scoped>
.home-hero {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 28px;
  overflow: hidden;
  color: var(--akioi-text);
  background:
    radial-gradient(circle at 100% 0, color-mix(in srgb, var(--akioi-primary) 20%, white), transparent 55%),
    linear-gradient(135deg, color-mix(in srgb, var(--akioi-primary) 12%, white), #fff 65%);
  border: 1px solid color-mix(in srgb, var(--akioi-primary) 26%, white);
  border-radius: var(--akioi-radius-lg);
  box-shadow: var(--akioi-shadow-sm);
}

.home-hero::after {
  position: absolute;
  top: -90px;
  right: -70px;
  width: 240px;
  height: 240px;
  content: '';
  background: color-mix(in srgb, var(--akioi-primary) 16%, white);
  border-radius: 50%;
}

.home-hero__body {
  position: relative;
  z-index: 1;
  min-width: 260px;
}

.home-hero__eyebrow {
  margin: 0 0 6px;
  font-size: 12px;
  letter-spacing: 0.5px;
  color: var(--akioi-text-weak);
}

.home-hero__title {
  margin: 0;
  font-size: 26px;
  font-weight: 600;
  line-height: 1.35;
}

.home-hero__desc {
  margin: 10px 0 18px;
  font-size: 13px;
  color: var(--akioi-text-weak);
}

.home-hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.home-hero__actions :deep(.el-button--primary) {
  --el-button-text-color: #fff;
  --el-button-hover-text-color: #fff;
  --el-button-active-text-color: #fff;
}

.home-hero__actions :deep(.el-button.is-text) {
  color: var(--akioi-text);
}

.home-hero__actions :deep(.el-button.is-text:hover) {
  color: var(--akioi-primary-hover);
  background: color-mix(in srgb, var(--akioi-primary) 14%, white);
}

.home-hero__stats {
  position: relative;
  z-index: 1;
  display: flex;
  gap: 32px;
  margin: 0;
}

.home-hero__stat dt {
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.home-hero__stat dd {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--akioi-text-weak);
}

.home-aside .panel:first-child {
  margin-top: 0;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.panel-head .panel__title {
  margin-bottom: 0;
}

.notice-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.notice-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 0;
  border-bottom: 1px dashed var(--akioi-border);
}

.notice-list li:last-child {
  padding-bottom: 0;
  border-bottom: none;
}

.notice-list .muted {
  flex-shrink: 0;
  font-size: 12px;
}

.contest-card {
  display: block;
  padding: 12px;
  color: inherit;
  border: 1px solid var(--akioi-border);
  border-left: 3px solid var(--akioi-border);
  border-radius: var(--akioi-radius);
  transition: border-color 0.15s ease;
}

.contest-card + .contest-card {
  margin-top: 10px;
}

.contest-card:hover {
  border-color: var(--akioi-primary);
}

.contest-card--running {
  border-left-color: #00a870;
  background: #e3f9e9;
}

.contest-card--pending {
  border-left-color: var(--akioi-primary);
}

.contest-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.contest-card__title {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contest-card__meta {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--akioi-text-weak);
}

.rank-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.rank-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px dashed var(--akioi-border);
}

.rank-item:last-child {
  padding-bottom: 0;
  border-bottom: none;
}

.rank-no {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  font-size: 12px;
  color: var(--akioi-text-weak);
  background: #f2f3f5;
  border-radius: 50%;
}

.rank-no--1 {
  color: #b7791f;
  background: #fff3d6;
}

.rank-no--2 {
  color: #64748b;
  background: #eceef1;
}

.rank-no--3 {
  color: #b45309;
  background: #fbe8dd;
}

.rank-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rank-points {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

@media (max-width: 1199px) {
  .home-aside {
    margin-top: 16px;
  }

  .home-aside .panel:first-child {
    margin-top: 0;
  }
}

@media (max-width: 767px) {
  .home-hero {
    padding: 20px;
  }

  .home-hero__title {
    font-size: 22px;
  }

  .home-hero__stats {
    gap: 24px;
  }
}
</style>
