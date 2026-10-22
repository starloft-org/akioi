<template>
  <div class="panel">
    <h3 class="panel__title">团队</h3>
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="搜索团队名称或简介"
        clearable
        style="width: 240px"
        @keyup.enter="applyFilter"
      />
      <el-button type="primary" @click="applyFilter">搜索</el-button>
      <div class="toolbar-spacer"></div>
      <el-button v-if="store.isLogin" @click="openCreate">创建团队</el-button>
    </div>

    <div v-loading="loading" class="team-grid">
      <div v-for="team in items" :key="team.id" class="team-card" @click="router.push(`/teams/${team.id}`)">
        <div class="team-card__head">
          <span class="team-card__name">{{ team.name }}</span>
          <el-tag v-if="team.my_role === 'owner'" type="warning" size="small">队长</el-tag>
          <el-tag v-else-if="team.my_role" type="success" size="small">成员</el-tag>
          <el-tag v-else-if="team.applied" type="info" size="small">待审批</el-tag>
        </div>
        <p class="team-card__desc">{{ team.description || '暂无简介' }}</p>
        <div class="team-card__foot muted">{{ team.member_count }} 名成员</div>
      </div>
    </div>

    <el-empty v-if="!loading && items.length === 0" description="暂无团队" />

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        :total="total"
        :page-sizes="[12, 24, 48]"
        layout="total, prev, pager, next, sizes"
        @current-change="load"
        @size-change="applyFilter"
      />
    </div>

    <el-dialog v-model="createVisible" title="创建团队" width="440px">
      <el-form :model="createForm" label-width="72px">
        <el-form-item label="名称">
          <el-input v-model="createForm.name" maxlength="32" placeholder="2-32 个字符" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="createForm.description" type="textarea" :rows="3" maxlength="255" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createTeam, listTeams } from '@/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()

const loading = ref(false)
const items = ref([])
const total = ref(0)
const keyword = ref('')
const query = reactive({ page: 1, page_size: 12 })

const createVisible = ref(false)
const creating = ref(false)
const createForm = reactive({ name: '', description: '' })

const load = async () => {
  loading.value = true
  try {
    const params = { page: query.page, page_size: query.page_size }
    if (keyword.value.trim()) {
      params.keyword = keyword.value.trim()
    }
    const data = await listTeams(params)
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const applyFilter = () => {
  query.page = 1
  load()
}

const openCreate = () => {
  if (!store.isLogin) {
    router.push({ path: '/login', query: { redirect: '/teams' } })
    return
  }
  createForm.name = ''
  createForm.description = ''
  createVisible.value = true
}

const submitCreate = async () => {
  if (!createForm.name.trim()) {
    ElMessage.warning('请填写团队名称')
    return
  }
  creating.value = true
  try {
    const data = await createTeam({ name: createForm.name.trim(), description: createForm.description.trim() })
    createVisible.value = false
    ElMessage.success('团队已创建')
    if (data?.team?.id) {
      router.push(`/teams/${data.team.id}`)
      return
    }
    load()
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    creating.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar-spacer {
  flex: 1;
}

.team-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
  min-height: 60px;
}

.team-card {
  padding: 16px;
  border: 1px solid var(--akioi-border);
  border-radius: var(--akioi-radius);
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.team-card:hover {
  border-color: var(--akioi-primary);
  box-shadow: 0 2px 8px rgba(29, 33, 41, 0.06);
}

.team-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.team-card__name {
  font-size: 15px;
  font-weight: 600;
}

.team-card__desc {
  margin: 8px 0 12px;
  min-height: 40px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--akioi-text-weak);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.team-card__foot {
  font-size: 12px;
}
</style>
