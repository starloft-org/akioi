<template>
  <div>
    <div v-if="!store.isLogin" class="panel home-welcome">
      <div>
        <div class="home-welcome__title">欢迎来到 {{ store.site.app_name }}</div>
        <div class="muted">登录后可提交代码、参加比赛并查看评测记录。</div>
      </div>
      <div class="home-welcome__actions">
        <el-button type="primary" @click="router.push('/login')">登录</el-button>
        <el-button v-if="store.site.register_enabled" @click="router.push('/register')">注册</el-button>
      </div>
    </div>

    <div v-loading="announcementLoading" class="panel">
      <h3 class="panel__title">公告</h3>
      <el-empty v-if="!announcementLoading && announcements.length === 0" description="暂无公告" />
      <ul v-else class="home-announcements">
        <li v-for="item in announcements" :key="item.id">
          <el-link type="primary" :underline="false" @click="openAnnouncement(item)">{{ item.title }}</el-link>
          <span class="muted">{{ formatTime(item.created_at) }}</span>
        </li>
      </ul>
    </div>

    <div class="home-columns">
      <div v-loading="contestLoading" class="panel">
        <div class="panel-head">
          <h3 class="panel__title">最近比赛</h3>
          <el-link type="primary" :underline="false" @click="router.push('/contests')">查看全部</el-link>
        </div>
        <el-empty v-if="!contestLoading && contests.length === 0" description="暂无比赛" />
        <el-table v-else :data="contests" size="small">
          <el-table-column label="比赛" min-width="180">
            <template #default="{ row }">
              <router-link :to="`/contests/${row.id}`">{{ row.title }}</router-link>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="contestStatusType(row.status)" size="small">
                {{ contestStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="开始时间" width="160">
            <template #default="{ row }">{{ formatTime(row.start_at) }}</template>
          </el-table-column>
        </el-table>
      </div>

      <div v-loading="problemLoading" class="panel">
        <div class="panel-head">
          <h3 class="panel__title">最新题目</h3>
          <el-link type="primary" :underline="false" @click="router.push('/problems')">查看全部</el-link>
        </div>
        <el-empty v-if="!problemLoading && problems.length === 0" description="暂无题目" />
        <el-table v-else :data="problems" size="small">
          <el-table-column label="题号" width="100">
            <template #default="{ row }">
              <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
            </template>
          </el-table-column>
          <el-table-column label="标题" min-width="180">
            <template #default="{ row }">
              <router-link :to="`/problems/${row.id}`">{{ row.title }}</router-link>
            </template>
          </el-table-column>
          <el-table-column label="难度" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.difficulty" size="small">{{ row.difficulty }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="current?.title" width="640px">
      <div class="pre-block">{{ current?.content }}</div>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getAnnouncements, listContests, listProblems } from '@/api'
import { useUserStore } from '@/store/user'
import { contestStatusText, contestStatusType, formatTime } from '@/utils/format'

const store = useUserStore()
const router = useRouter()

const announcements = ref([])
const contests = ref([])
const problems = ref([])
const announcementLoading = ref(true)
const contestLoading = ref(true)
const problemLoading = ref(true)
const dialogVisible = ref(false)
const current = ref(null)

const openAnnouncement = (item) => {
  current.value = item
  dialogVisible.value = true
}

const loadAnnouncements = async () => {
  announcementLoading.value = true
  try {
    const data = await getAnnouncements()
    announcements.value = data.items || []
  } catch (e) {
    announcements.value = []
  } finally {
    announcementLoading.value = false
  }
}

const loadContests = async () => {
  contestLoading.value = true
  try {
    const data = await listContests({ page: 1, page_size: 5 })
    contests.value = data.items || []
  } catch (e) {
    contests.value = []
  } finally {
    contestLoading.value = false
  }
}

const loadProblems = async () => {
  problemLoading.value = true
  try {
    const data = await listProblems({ page: 1, page_size: 10 })
    problems.value = data.items || []
  } catch (e) {
    problems.value = []
  } finally {
    problemLoading.value = false
  }
}

onMounted(() => {
  loadAnnouncements()
  loadContests()
  loadProblems()
})
</script>

<style scoped>
.home-welcome {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.home-welcome__title {
  font-size: 18px;
  font-weight: 600;
}

.home-welcome__actions {
  display: flex;
  gap: 8px;
}

.home-announcements {
  margin: 0;
  padding-left: 0;
  list-style: none;
}

.home-announcements li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 6px 0;
  border-bottom: 1px dashed var(--akioi-border);
}

.home-announcements li:last-child {
  border-bottom: none;
}

.home-columns {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
  gap: 16px;
}

.home-columns .panel {
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
</style>
