<template>
  <div class="panel">
    <h3 class="panel__title">题单</h3>
    <p class="panel__hint">
      个人题单只有自己能看；团队题单由队长维护，团队成员都能查看。题单本身不公开。
    </p>

    <div class="toolbar">
      <el-radio-group v-model="kindFilter">
        <el-radio-button value="">全部</el-radio-button>
        <el-radio-button value="personal">个人题单</el-radio-button>
        <el-radio-button value="team">团队题单</el-radio-button>
      </el-radio-group>
      <el-button type="primary" @click="openCreate">新建题单</el-button>
    </div>

    <el-table v-loading="loading" :data="shown" size="small">
      <el-table-column label="标题" min-width="220">
        <template #default="{ row }">
          <router-link class="problem-link" :to="detailPath(row)">{{ row.title }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="110">
        <template #default="{ row }">
          <el-tag size="small" :type="row.kind === 'team' ? 'warning' : 'success'">
            {{ row.kind === 'team' ? '团队题单' : '个人题单' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="所属团队" width="160">
        <template #default="{ row }">
          <span v-if="row.kind === 'team'">{{ teamName(row.team_id) }}</span>
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="题目数" width="90" align="right">
        <template #default="{ row }">
          <span class="tabular">{{ row.problem_count }}</span>
        </template>
      </el-table-column>
      <el-table-column label="说明" min-width="200">
        <template #default="{ row }">
          <span class="muted">{{ row.description || '暂无说明' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">查看</el-button>
          <el-button v-if="canManage(row)" link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <el-dialog v-model="dialog" title="新建题单" width="540px">
    <el-form label-width="90px">
      <el-form-item label="归属">
        <el-radio-group v-model="form.kind">
          <el-radio value="personal">个人题单</el-radio>
          <el-radio value="team">团队题单</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="form.kind === 'team'" label="团队">
        <el-select v-model="form.team_id" placeholder="请选择团队" style="width: 100%">
          <el-option v-for="t in ownerTeams" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
        <p class="form-hint">只有你担任队长的团队可以创建团队题单。</p>
      </el-form-item>
      <el-form-item label="标题">
        <el-input v-model="form.title" maxlength="128" />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="3" maxlength="255" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import { createMyProblemSet, deleteMyProblemSet, listMyProblemSets, listTeams } from '@/api'

const router = useRouter()
const loading = ref(false)
const items = ref([])
const teams = ref([])
const kindFilter = ref('')
const dialog = ref(false)
const saving = ref(false)
const form = ref({ kind: 'personal', team_id: '', title: '', description: '' })

const ownerTeams = computed(() => teams.value.filter((t) => t.my_role === 'owner'))
const shown = computed(() =>
  kindFilter.value ? items.value.filter((item) => item.kind === kindFilter.value) : items.value
)

function teamName(id) {
  const team = teams.value.find((t) => t.id === id)
  return team ? team.name : `#${id}`
}

function canManage(row) {
  if (row.kind === 'personal') return true
  const team = teams.value.find((t) => t.id === row.team_id)
  return team?.my_role === 'owner'
}

function detailPath(row) {
  return row.kind === 'team'
    ? `/problem-sets/team/${row.team_id}/${row.id}`
    : `/problem-sets/personal/${row.id}`
}

function openDetail(row) {
  router.push(detailPath(row))
}

async function load() {
  loading.value = true
  try {
    const data = await listMyProblemSets()
    items.value = data.items || []
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '加载题单失败')
  } finally {
    loading.value = false
  }
}

async function loadTeams() {
  try {
    const data = await listTeams({ page: 1, page_size: 100 })
    teams.value = data.items || []
  } catch {
    teams.value = []
  }
}

function openCreate() {
  form.value = { kind: 'personal', team_id: '', title: '', description: '' }
  dialog.value = true
}

async function save() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写题单标题')
    return
  }
  if (form.value.kind === 'team' && !form.value.team_id) {
    ElMessage.warning('请选择团队')
    return
  }
  saving.value = true
  try {
    const data = await createMyProblemSet({
      title: form.value.title,
      description: form.value.description,
      team_id: form.value.kind === 'team' ? form.value.team_id : 0,
      problem_ids: []
    })
    dialog.value = false
    ElMessage.success('题单已创建')
    const id = data.set?.id
    if (id) {
      router.push(
        form.value.kind === 'team'
          ? `/problem-sets/team/${form.value.team_id}/${id}`
          : `/problem-sets/personal/${id}`
      )
    } else {
      load()
    }
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '创建题单失败')
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除题单「${row.title}」？`, '删除题单', { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteMyProblemSet(row.id)
    ElMessage.success('题单已删除')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '删除题单失败')
  }
}

onMounted(() => {
  load()
  loadTeams()
})
</script>

<style scoped>
.form-hint {
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
