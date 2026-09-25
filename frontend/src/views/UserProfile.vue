<template>
  <div v-loading="loading">
    <template v-if="user">
      <div class="panel profile-head">
        <el-avatar :size="72" :src="user.avatar || undefined">{{ initial }}</el-avatar>
        <div class="profile-info">
          <div class="profile-name">{{ user.nickname || user.username }}</div>
          <div class="muted">@{{ user.username }}</div>
          <div v-if="user.signature" class="profile-signature">{{ user.signature }}</div>
          <div class="muted profile-extra">
            <span v-if="user.school">学校：{{ user.school }}</span>
            <span v-if="user.grade">年级：{{ user.grade }}</span>
            <span>注册时间：{{ formatTime(user.created_at) }}</span>
          </div>
        </div>
      </div>

      <div class="stat-grid profile-stats">
        <div class="stat-card">
          <div class="stat-card__label">通过题数</div>
          <div class="stat-card__value">{{ solvedCount }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-card__label">提交数</div>
          <div class="stat-card__value">{{ submissionCount }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-card__label">通过次数</div>
          <div class="stat-card__value">{{ acceptedCount }}</div>
        </div>
      </div>

      <div class="panel">
        <h3 class="panel__title">最近提交</h3>
        <el-empty v-if="recentSubmissions.length === 0" description="暂无提交记录" />
        <el-table v-else :data="recentSubmissions" size="small">
          <el-table-column label="状态" width="140">
            <template #default="{ row }">
              <StatusTag :status="row.status" />
            </template>
          </el-table-column>
          <el-table-column label="题目" min-width="220">
            <template #default="{ row }">
              <router-link :to="`/problems/${row.problem_id}`">
                {{ row.problem_display_id }} {{ row.problem_title }}
              </router-link>
            </template>
          </el-table-column>
          <el-table-column label="得分" width="80" prop="score" />
          <el-table-column label="提交时间" min-width="160">
            <template #default="{ row }">
              <router-link :to="`/submissions/${row.id}`">{{ formatTime(row.created_at) }}</router-link>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getUserProfile } from '@/api'
import StatusTag from '@/components/StatusTag.vue'
import { formatTime } from '@/utils/format'

const route = useRoute()

const loading = ref(true)
const user = ref(null)
const solvedCount = ref(0)
const submissionCount = ref(0)
const acceptedCount = ref(0)
const recentSubmissions = ref([])

const initial = computed(() => (user.value?.nickname || user.value?.username || '?').charAt(0).toUpperCase())

const load = async () => {
  loading.value = true
  try {
    const data = await getUserProfile(route.params.id)
    user.value = data.user
    solvedCount.value = data.solved_count || 0
    submissionCount.value = data.submission_count || 0
    acceptedCount.value = data.accepted_count || 0
    recentSubmissions.value = data.recent_submissions || []
  } catch (e) {
    user.value = null
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.profile-head {
  display: flex;
  align-items: center;
  gap: 20px;
}

.profile-name {
  font-size: 20px;
  font-weight: 600;
}

.profile-signature {
  margin: 4px 0;
}

.profile-extra {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 13px;
}

.profile-stats {
  margin: 16px 0;
}
</style>
