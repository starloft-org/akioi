<template>
  <div class="panel">
    <h3 class="panel__title">管理员管理</h3>
    <el-alert
      class="admin-tip"
      type="info"
      :closable="false"
      show-icon
      title="超级管理员可以把已有账号设为管理员，并按后台模块分配权限；管理员登录后台后只能看到并被允许操作被分配的模块。"
    />
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">添加管理员</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="username" label="用户名" min-width="130" />
      <el-table-column prop="nickname" label="昵称" min-width="120" />
      <el-table-column label="角色" width="120">
        <template #default="{ row }">
          <el-tag v-if="row.role === 'superadmin'" type="danger">超级管理员</el-tag>
          <el-tag v-else type="warning">管理员</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="权限" min-width="240">
        <template #default="{ row }">
          <el-tag v-if="row.role === 'superadmin'" type="danger" size="small">全部权限</el-tag>
          <template v-else>
            <el-tag v-for="key in row.permissions || []" :key="key" size="small" class="permission-tag">
              {{ labelOf(key) }}
            </el-tag>
            <span v-if="!(row.permissions || []).length" class="muted">未分配权限</span>
          </template>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最后登录" width="170">
        <template #default="{ row }">{{ row.last_login_at ? formatTime(row.last_login_at) : '—' }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <template v-if="row.role === 'superadmin'">
            <span class="muted">不可修改</span>
          </template>
          <template v-else>
            <el-button size="small" @click="openEdit(row)">编辑权限</el-button>
            <el-button size="small" type="danger" plain @click="removeAdmin(row)">取消管理员</el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" title="添加管理员" width="520px" :close-on-click-modal="false">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="选择用户">
          <el-select
            v-model="createForm.user_id"
            filterable
            remote
            reserve-keyword
            :remote-method="searchUsers"
            :loading="searching"
            placeholder="输入用户名或昵称搜索"
            style="width: 100%"
            @focus="searchUsers('')"
          >
            <el-option
              v-for="item in userOptions"
              :key="item.id"
              :label="item.nickname ? `${item.username}（${item.nickname}）` : item.username"
              :value="item.id"
            />
          </el-select>
          <p class="muted admin-note">把已有账号设为管理员，不会新建账号；账号的昵称、密码仍在用户管理中维护。</p>
        </el-form-item>
        <el-form-item label="权限">
          <div class="permission-groups">
            <div v-for="group in groups" :key="group.name" class="permission-group">
              <div class="permission-group__title">{{ group.name }}</div>
              <el-checkbox-group v-model="createForm.permissions">
                <el-checkbox v-for="option in group.options" :key="option.key" :value="option.key">
                  {{ option.label }}
                </el-checkbox>
              </el-checkbox-group>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitCreate">设为管理员</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editVisible" :title="`编辑管理员 ${editForm.username}`" width="520px" :close-on-click-modal="false">
      <el-form :model="editForm" label-width="90px">
        <el-form-item label="昵称">
          <el-input v-model="editForm.nickname" placeholder="留空表示不修改" />
        </el-form-item>
        <el-form-item label="重设密码">
          <el-input v-model="editForm.password" type="password" show-password placeholder="留空表示不修改，至少 6 位" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch
            v-model="editForm.status"
            :active-value="1"
            :inactive-value="2"
            active-text="启用"
            inactive-text="禁用"
            inline-prompt
          />
        </el-form-item>
        <el-form-item label="权限">
          <div class="permission-groups">
            <div v-for="group in groups" :key="group.name" class="permission-group">
              <div class="permission-group__title">{{ group.name }}</div>
              <el-checkbox-group v-model="editForm.permissions">
                <el-checkbox v-for="option in group.options" :key="option.key" :value="option.key">
                  {{ option.label }}
                </el-checkbox>
              </el-checkbox-group>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminCreateAdmin, adminDeleteAdmin, adminListAdmins, adminListUsers, adminUpdateAdmin } from '@/api'
import { formatTime } from '@/utils/format'

const FALLBACK_OPTIONS = [
  { key: 'users', label: '用户管理', group: '运营' },
  { key: 'announcements', label: '公告管理', group: '运营' },
  { key: 'problems', label: '题目管理', group: '内容' },
  { key: 'problem_banks', label: '题库管理', group: '内容' },
  { key: 'contests', label: '比赛管理', group: '内容' },
  { key: 'submissions', label: '评测记录', group: '内容' },
  { key: 'judges', label: '测评机管理', group: '内容' },
  { key: 'settings', label: '系统设置', group: '系统' },
  { key: 'backups', label: '数据备份', group: '系统' },
  { key: 'logs', label: '日志查看', group: '系统' }
]

const loading = ref(false)
const saving = ref(false)
const items = ref([])
const options = ref(FALLBACK_OPTIONS)

const groups = computed(() => {
  const order = []
  const map = new Map()
  options.value.forEach((option) => {
    const name = option.group || '其他'
    if (!map.has(name)) {
      map.set(name, [])
      order.push(name)
    }
    map.get(name).push(option)
  })
  return order.map((name) => ({ name, options: map.get(name) }))
})

const labelOf = (key) => options.value.find((option) => option.key === key)?.label || key

const load = async () => {
  loading.value = true
  try {
    const data = await adminListAdmins()
    items.value = data.items || []
    if (Array.isArray(data.permission_options) && data.permission_options.length) {
      options.value = data.permission_options
    }
  } catch (e) {
    items.value = []
  } finally {
    loading.value = false
  }
}

const createVisible = ref(false)
const createForm = reactive({ user_id: null, permissions: [] })
const userOptions = ref([])
const searching = ref(false)

// 只在普通用户里搜索候选账号，管理员已经在列表里
const searchUsers = async (keyword) => {
  searching.value = true
  try {
    const data = await adminListUsers({ keyword: keyword || undefined, role: 'user', page: 1, page_size: 20 })
    userOptions.value = (data.items || []).filter((item) => item.role === 'user')
  } catch (e) {
    userOptions.value = []
  } finally {
    searching.value = false
  }
}

const openCreate = () => {
  createForm.user_id = null
  createForm.permissions = options.value.map((option) => option.key)
  userOptions.value = []
  createVisible.value = true
  searchUsers('')
}

const submitCreate = async () => {
  if (!createForm.user_id) {
    ElMessage.warning('请选择要设为管理员的用户')
    return
  }
  saving.value = true
  try {
    await adminCreateAdmin({
      user_id: createForm.user_id,
      permissions: createForm.permissions
    })
    ElMessage.success('已设为管理员')
    createVisible.value = false
    load()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    saving.value = false
  }
}

const editVisible = ref(false)
const editForm = reactive({ id: 0, username: '', nickname: '', password: '', status: 1, permissions: [] })

const openEdit = (row) => {
  editForm.id = row.id
  editForm.username = row.username
  editForm.nickname = ''
  editForm.password = ''
  editForm.status = row.status
  editForm.permissions = [...(row.permissions || [])]
  editVisible.value = true
}

const submitEdit = async () => {
  if (editForm.password && editForm.password.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  const payload = {
    permissions: editForm.permissions,
    status: editForm.status
  }
  if (editForm.nickname.trim()) {
    payload.nickname = editForm.nickname.trim()
  }
  if (editForm.password) {
    payload.password = editForm.password
  }
  saving.value = true
  try {
    await adminUpdateAdmin(editForm.id, payload)
    ElMessage.success('管理员已更新')
    editVisible.value = false
    load()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    saving.value = false
  }
}

const removeAdmin = async (row) => {
  try {
    await ElMessageBox.confirm(`确定取消 ${row.username} 的管理员身份吗？该账号将变为普通用户。`, '取消管理员', {
      type: 'warning'
    })
    await adminDeleteAdmin(row.id)
    ElMessage.success('已取消管理员')
    load()
  } catch (e) {
    // 取消操作或接口失败时不再提示
  }
}

onMounted(load)
</script>

<style scoped>
.admin-tip {
  margin-bottom: 12px;
}

.admin-note {
  margin: 6px 0 0;
  font-size: 12px;
}

.permission-groups {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.permission-group__title {
  font-size: 12px;
  color: var(--akioi-text-weak);
  margin-bottom: 2px;
}

.permission-tag {
  margin: 2px 6px 2px 0;
}
</style>
