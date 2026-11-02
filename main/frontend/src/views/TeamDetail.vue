<template>
  <div v-loading="loading">
    <template v-if="team">
      <div class="panel team-head">
        <div class="team-head__main">
          <div class="team-head__title">
            <h2>{{ team.name }}</h2>
            <el-tag v-if="myRole === 'owner'" type="warning" size="small">队长</el-tag>
            <el-tag v-else-if="myRole" type="success" size="small">成员</el-tag>
          </div>
          <p class="muted team-head__desc">{{ team.description || '暂无简介' }}</p>
          <div class="muted team-head__meta">{{ team.member_count }} 名成员</div>
        </div>
        <div class="team-head__actions">
          <el-tag v-if="!isMember && applied" type="info">申请待审批</el-tag>
          <el-button v-else-if="!isMember" type="primary" @click="openApply">申请加入</el-button>
          <el-button v-if="canManage" @click="openEdit">编辑团队</el-button>
          <el-button v-if="isMember && myRole !== 'owner'" @click="quitTeam">退出团队</el-button>
        </div>
      </div>

      <div v-if="isMember" class="panel">
        <el-tabs v-model="tab">
          <el-tab-pane label="成员" name="members">
            <el-table :data="members" size="small">
              <el-table-column label="用户" min-width="160">
                <template #default="{ row }">
                  <router-link :to="`/users/${row.user_id}`">{{ row.nickname || row.username }}</router-link>
                  <el-tag v-if="row.role === 'owner'" type="warning" size="small" style="margin-left: 8px">队长</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="加入时间" width="180">
                <template #default="{ row }">{{ formatTime(row.joined_at) }}</template>
              </el-table-column>
              <el-table-column v-if="canManage" label="操作" width="100" align="right">
                <template #default="{ row }">
                  <el-button
                    v-if="row.role !== 'owner'"
                    size="small"
                    type="danger"
                    plain
                    @click="removeMember(row)"
                  >
                    移除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="题目" name="problems">
            <el-empty v-if="problems.length === 0" description="暂无团队题目，由管理员在后台创建时指定归属团队" />
            <el-table v-else :data="problems" size="small">
              <el-table-column label="题号" width="90">
                <template #default="{ row }">
                  <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
                </template>
              </el-table-column>
              <el-table-column label="标题" min-width="220">
                <template #default="{ row }">
                  <router-link :to="`/problems/${row.id}`">{{ row.title }}</router-link>
                </template>
              </el-table-column>
              <el-table-column label="难度" width="110">
                <template #default="{ row }">
                  <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
                </template>
              </el-table-column>
              <el-table-column label="作者" width="120">
                <template #default="{ row }">
                  <span v-if="row.author_name">{{ row.author_name }}</span>
                  <span v-else class="muted">—</span>
                </template>
              </el-table-column>
              <el-table-column label="测试点" width="90" prop="testcase_count" align="right" />
              <el-table-column label="提交 / 通过" width="130" align="right">
                <template #default="{ row }">{{ row.submit_count }} / {{ row.accepted_count }}</template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="题单" name="sets">
            <div v-if="canManage" class="toolbar">
              <el-button type="primary" @click="openSetDialog()">新建题单</el-button>
            </div>
            <el-empty v-if="sets.length === 0" description="还没有题单" />
            <div v-for="set in sets" :key="set.id" class="set-item">
              <div class="set-item__main">
                <div class="set-item__title">
                  <router-link :to="`/problem-sets/team/${teamId}/${set.id}`">{{ set.title }}</router-link>
                </div>
                <div class="muted set-item__desc">{{ set.description || '暂无说明' }}</div>
<div class="muted set-item__meta">{{ set.problem_count }} 道题 · 作者 {{ set.author_name || team?.name || '' }}</div>
                <div v-if="set.problem_ids.length" class="set-item__links">
                  <router-link
                    v-for="pid in set.problem_ids"
                    :key="pid"
                    :to="`/problems/${pid}`"
                    class="set-item__link"
                  >
                    #{{ pid }}
                  </router-link>
                </div>
              </div>
              <div v-if="canManage" class="set-item__actions">
                <el-button size="small" @click="openSetDialog(set)">编辑</el-button>
                <el-button size="small" type="danger" plain @click="removeSet(set)">删除</el-button>
              </div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="比赛" name="contests">
            <el-empty v-if="contests.length === 0" description="暂无团队比赛，由管理员在后台创建时指定归属团队" />
            <el-table v-else :data="contests" size="small">
              <el-table-column label="比赛" min-width="220">
                <template #default="{ row }">
                  <router-link :to="`/contests/${row.id}`">{{ row.title }}</router-link>
                </template>
              </el-table-column>
              <el-table-column label="开始" width="180">
                <template #default="{ row }">{{ formatTime(row.start_at) }}</template>
              </el-table-column>
              <el-table-column label="结束" width="180">
                <template #default="{ row }">{{ formatTime(row.end_at) }}</template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="contestStatusType(row.status)" size="small">{{ contestStatusText(row.status) }}</el-tag>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane v-if="canManage" label="申请审批" name="applications">
            <el-empty v-if="applications.length === 0" description="没有待审批的申请" />
            <el-table v-else :data="applications" size="small">
              <el-table-column label="申请人" min-width="140">
                <template #default="{ row }">
                  <router-link :to="`/users/${row.user_id}`">{{ row.nickname || row.username }}</router-link>
                </template>
              </el-table-column>
              <el-table-column label="留言" min-width="200">
                <template #default="{ row }">{{ row.message || '-' }}</template>
              </el-table-column>
              <el-table-column label="申请时间" width="180">
                <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
              </el-table-column>
              <el-table-column label="操作" width="160" align="right">
                <template #default="{ row }">
                  <el-button size="small" type="primary" @click="review(row, 'approve')">通过</el-button>
                  <el-button size="small" plain @click="review(row, 'reject')">拒绝</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>

      <div v-else class="panel muted">加入团队后可查看团队题目、题单与比赛。</div>
      <!-- 申请加入 -->
      <el-dialog v-model="applyVisible" title="申请加入团队" width="420px">
        <p class="muted apply-tip">提交后需等待队长审批，可在团队页查看进度。</p>
        <el-input v-model="applyMessage" type="textarea" :rows="3" maxlength="255" show-word-limit placeholder="介绍一下自己（选填）" />
        <template #footer>
          <el-button @click="applyVisible = false">取消</el-button>
          <el-button type="primary" :loading="applying" @click="submitApply">提交申请</el-button>
        </template>
      </el-dialog>

      <!-- 编辑团队 -->
      <el-dialog v-model="editVisible" title="编辑团队" width="440px">
        <el-form :model="editForm" label-width="72px">
          <el-form-item label="名称">
            <el-input v-model="editForm.name" maxlength="32" />
          </el-form-item>
          <el-form-item label="简介">
            <el-input v-model="editForm.description" type="textarea" :rows="3" maxlength="255" show-word-limit />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="editVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="submitEdit">保存</el-button>
        </template>
      </el-dialog>

      <!-- 题单编辑 -->
      <el-dialog v-model="setVisible" :title="setForm.id ? '编辑题单' : '新建题单'" width="520px">
        <el-form :model="setForm" label-width="82px">
          <el-form-item label="标题">
            <el-input v-model="setForm.title" maxlength="128" />
          </el-form-item>
          <el-form-item label="说明">
            <el-input v-model="setForm.description" maxlength="255" />
          </el-form-item>
          <el-form-item label="题目 ID">
            <el-input v-model="setForm.problemIds" placeholder="多个 ID 用逗号分隔，如 1,2,3" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="setVisible = false">取消</el-button>
          <el-button type="primary" :loading="setSaving" @click="submitSet">保存</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  applyTeam,
  createTeamSet,
  deleteTeamSet,
  getTeam,
  listTeamApplications,
  listTeamContests,
  listTeamMembers,
  listTeamProblems,
  listTeamSets,
  removeTeamMember,
  reviewTeamApplication,
  updateTeam,
  updateTeamSet
} from '@/api'
import DifficultyTag from '@/components/DifficultyTag.vue'
import { useUserStore } from '@/store/user'
import { formatTime } from '@/utils/format'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const loading = ref(true)
const team = ref(null)
const myRole = ref('')
const applied = ref(false)
const tab = ref('members')

const members = ref([])
const problems = ref([])
const sets = ref([])
const contests = ref([])
const applications = ref([])

const isMember = computed(() => !!myRole.value)
const canManage = computed(() => myRole.value === 'owner')
const teamId = computed(() => Number(route.params.id))

const applyVisible = ref(false)
const applyMessage = ref('')
const applying = ref(false)

const editVisible = ref(false)
const saving = ref(false)
const editForm = reactive({ name: '', description: '' })

const setVisible = ref(false)
const setSaving = ref(false)
const setForm = reactive({ id: 0, title: '', description: '', problemIds: '' })

const contestStatusText = (status) => ({ pending: '未开始', running: '进行中', ended: '已结束' }[status] || status)
const contestStatusType = (status) => ({ pending: 'info', running: 'success', ended: 'info' }[status] || 'info')

const load = async () => {
  loading.value = true
  try {
    const data = await getTeam(teamId.value)
    team.value = data.team || null
    myRole.value = data.my_role || ''
    applied.value = !!data.applied
    if (isMember.value) {
      await Promise.all([loadMembers(), loadProblems(), loadSets(), loadContests()])
      if (canManage.value) {
        await loadApplications()
      }
    }
  } catch (e) {
    team.value = null
  } finally {
    loading.value = false
  }
}

const loadMembers = async () => {
  const data = await listTeamMembers(teamId.value).catch(() => ({ items: [] }))
  members.value = data.items || []
}
const loadProblems = async () => {
  const data = await listTeamProblems(teamId.value).catch(() => ({ items: [] }))
  problems.value = data.items || []
}
const loadSets = async () => {
  const data = await listTeamSets(teamId.value).catch(() => ({ items: [] }))
  sets.value = data.items || []
}
const loadContests = async () => {
  const data = await listTeamContests(teamId.value).catch(() => ({ items: [] }))
  contests.value = data.items || []
}
const loadApplications = async () => {
  const data = await listTeamApplications(teamId.value, { status: 'pending' }).catch(() => ({ items: [] }))
  applications.value = data.items || []
}

const openApply = () => {
  if (!store.isLogin) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  applyMessage.value = ''
  applyVisible.value = true
}

const submitApply = async () => {
  applying.value = true
  try {
    await applyTeam(teamId.value, { message: applyMessage.value.trim() })
    applyVisible.value = false
    applied.value = true
    ElMessage.success('申请已提交，请等待队长审批')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    applying.value = false
  }
}

const openEdit = () => {
  editForm.name = team.value?.name || ''
  editForm.description = team.value?.description || ''
  editVisible.value = true
}

const submitEdit = async () => {
  saving.value = true
  try {
    await updateTeam(teamId.value, { name: editForm.name.trim(), description: editForm.description.trim() })
    editVisible.value = false
    ElMessage.success('已保存')
    load()
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}

const quitTeam = async () => {
  try {
    await ElMessageBox.confirm('确定退出该团队？退出后需重新申请才能加入。', '退出团队', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await removeTeamMember(teamId.value, store.user?.id)
    ElMessage.success('已退出团队')
    router.push('/teams')
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

const removeMember = async (row) => {
  try {
    await ElMessageBox.confirm(`确定将 ${row.nickname || row.username} 移出团队？`, '移除成员', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await removeTeamMember(teamId.value, row.user_id)
    ElMessage.success('已移除')
    load()
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

const review = async (row, action) => {
  try {
    await reviewTeamApplication(teamId.value, row.id, { action })
    ElMessage.success(action === 'approve' ? '已通过' : '已拒绝')
    loadApplications()
    loadMembers()
    load()
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

const openSetDialog = (set) => {
  setForm.id = set?.id || 0
  setForm.title = set?.title || ''
  setForm.description = set?.description || ''
  setForm.problemIds = (set?.problem_ids || []).join(',')
  setVisible.value = true
}

const submitSet = async () => {
  if (!setForm.title.trim()) {
    ElMessage.warning('请填写题单标题')
    return
  }
  const ids = setForm.problemIds
    .split(/[,，\s]+/)
    .map((item) => Number(item))
    .filter((n) => Number.isInteger(n) && n > 0)
  setSaving.value = true
  try {
    const payload = { title: setForm.title.trim(), description: setForm.description.trim(), problem_ids: ids }
    if (setForm.id) {
      await updateTeamSet(teamId.value, setForm.id, payload)
    } else {
      await createTeamSet(teamId.value, payload)
    }
    setVisible.value = false
    ElMessage.success('已保存')
    loadSets()
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    setSaving.value = false
  }
}

const removeSet = async (set) => {
  try {
    await ElMessageBox.confirm(`确定删除题单「${set.title}」？`, '删除题单', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await deleteTeamSet(teamId.value, set.id)
    ElMessage.success('已删除')
    loadSets()
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

watch(tab, (value) => {
  if (value === 'applications' && canManage.value) {
    loadApplications()
  }
})

onMounted(load)
</script>

<style scoped>
.team-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.team-head__title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.team-head__title h2 {
  margin: 0;
  font-size: 20px;
}

.team-head__desc {
  margin: 8px 0 6px;
  font-size: 13px;
}

.team-head__meta {
  font-size: 12px;
}

.team-head__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.apply-tip {
  margin: 0 0 12px;
  font-size: 13px;
}

.set-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid var(--akioi-border);
}

.set-item:last-child {
  border-bottom: none;
}

.set-item__title {
  font-weight: 600;
}

.set-item__desc {
  margin: 4px 0;
  font-size: 13px;
}

.set-item__meta {
  font-size: 12px;
}

.set-item__links {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}

.set-item__link {
  font-size: 12px;
  font-family: Consolas, Monaco, monospace;
}

.set-item__actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
</style>
