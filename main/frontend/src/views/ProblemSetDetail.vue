<template>
  <div class="panel" v-loading="loading">
    <div class="set-head">
      <div>
        <h3 class="panel__title">
          {{ set.title || '题单' }}
          <el-tag size="small" :type="isTeam ? 'warning' : 'success'" style="margin-left: 8px">
            {{ isTeam ? '团队题单' : '个人题单' }}
          </el-tag>
        </h3>
        <p class="panel__hint" v-if="isTeam">所属团队：{{ teamName }}</p>
        <p class="panel__hint">{{ set.description || '暂无说明' }}</p>
      </div>
      <div v-if="editable" class="set-head__actions">
        <el-button size="small" @click="openEdit">编辑题单</el-button>
        <el-button size="small" type="danger" @click="remove">删除题单</el-button>
      </div>
    </div>

    <el-table :data="problems" size="small">
      <el-table-column label="题号" width="120">
        <template #default="{ row }">
          <router-link class="mono" :to="`/problems/${row.id}`">{{ row.display_id }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="标题" min-width="240">
        <template #default="{ row }">
          <router-link class="problem-link" :to="`/problems/${row.id}`">{{ row.title }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="难度" width="110">
        <template #default="{ row }">
          <DifficultyTag v-if="row.difficulty" :difficulty="row.difficulty" />
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="标签" min-width="160">
        <template #default="{ row }">
          <el-tag v-for="tag in row.tags || []" :key="tag" size="small" class="tag">{{ tag }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag v-if="row.my_status === 'solved'" size="small" type="success">已通过</el-tag>
          <el-tag v-else-if="row.my_status === 'attempted'" size="small" type="warning">尝试过</el-tag>
          <span v-else class="muted">未提交</span>
        </template>
      </el-table-column>
    </el-table>

    <el-empty v-if="!problems.length && !loading" description="题单里还没有题目" />

    <el-dialog v-model="dialog" title="编辑题单" width="620px">
      <el-form label-width="90px">
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="128" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="255" />
        </el-form-item>
        <el-form-item label="题目">
          <el-select
            v-model="form.problem_ids"
            multiple
            filterable
            placeholder="从可管理的题目中选择"
            style="width: 100%"
          >
            <el-option
              v-for="item in candidates"
              :key="item.id"
              :label="`${item.display_id} ${item.title}`"
              :value="item.id"
            />
          </el-select>
          <p class="form-hint">列表来自「我的题目」，可输入题号或标题搜索。</p>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import DifficultyTag from '@/components/DifficultyTag.vue'
import {
  deleteMyProblemSet,
  deleteTeamSet,
  getMyProblemSet,
  getTeamSet,
  listMyProblems,
  listTeams,
  updateMyProblemSet,
  updateTeamSet
} from '@/api'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const saving = ref(false)
const dialog = ref(false)
const set = ref({})
const problems = ref([])
const candidates = ref([])
const teams = ref([])
const form = ref({ title: '', description: '', problem_ids: [] })

const isTeam = computed(() => route.name === 'problem-set-team' || Boolean(route.params.teamId))
const teamId = computed(() => Number(route.params.teamId) || 0)
const setId = computed(() => Number(route.params.setId || route.params.id) || 0)
const teamName = computed(() => {
  const team = teams.value.find((t) => t.id === teamId.value)
  return team ? team.name : `#${teamId.value}`
})
const editable = computed(() => {
  if (!isTeam.value) return true
  const team = teams.value.find((t) => t.id === teamId.value)
  return team?.my_role === 'owner'
})

async function loadTeams() {
  try {
    const data = await listTeams({ page: 1, page_size: 100 })
    teams.value = data.items || []
  } catch {
    teams.value = []
  }
}

async function load() {
  loading.value = true
  try {
    const data = isTeam.value
      ? await getTeamSet(teamId.value, setId.value)
      : await getMyProblemSet(setId.value)
    set.value = data.set || {}
    problems.value = data.problems || []
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '加载题单失败')
  } finally {
    loading.value = false
  }
}

async function loadCandidates() {
  try {
    const data = await listMyProblems({ page: 1, page_size: 200 })
    candidates.value = data.items || []
  } catch {
    candidates.value = []
  }
}

function openEdit() {
  form.value = {
    title: set.value.title || '',
    description: set.value.description || '',
    problem_ids: problems.value.map((item) => item.id)
  }
  dialog.value = true
  if (!candidates.value.length) {
    loadCandidates()
  }
}

async function save() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写题单标题')
    return
  }
  if (!editable.value) {
    ElMessage.warning(isTeam.value ? '只有团队队长可以修改题单' : '只有题单创建者可以修改题单')
    return
  }
  saving.value = true
  try {
    const payload = {
      title: form.value.title,
      description: form.value.description,
      problem_ids: form.value.problem_ids
    }
    // 团队题单由队长通过团队接口修改，个人题单走个人接口
    if (isTeam.value) {
      await updateTeamSet(teamId.value, setId.value, payload)
    } else {
      await updateMyProblemSet(setId.value, payload)
    }
    ElMessage.success('题单已更新')
    dialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '保存题单失败')
  } finally {
    saving.value = false
  }
}

async function remove() {
  try {
    await ElMessageBox.confirm(`确定删除题单「${set.value.title || ''}」？`, '删除题单', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    if (isTeam.value) {
      await deleteTeamSet(teamId.value, setId.value)
    } else {
      await deleteMyProblemSet(setId.value)
    }
    ElMessage.success('题单已删除')
    router.push('/problem-sets')
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '删除题单失败')
  }
}

onMounted(async () => {
  if (isTeam.value) {
    await loadTeams()
  }
  load()
})
</script>

<style scoped>
.set-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.set-head__actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.tag {
  margin-right: 4px;
}

.form-hint {
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
