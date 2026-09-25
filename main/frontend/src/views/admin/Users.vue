<template>
  <div class="panel">
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="用户名 / 昵称"
        clearable
        style="width: 220px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-select v-model="roleFilter" placeholder="角色" clearable style="width: 120px" @change="search">
        <el-option v-for="item in ROLE_OPTIONS" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-upload action="#" accept=".csv" :show-file-list="false" :http-request="handleImport">
        <el-button :loading="importing">批量导入</el-button>
      </el-upload>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" min-width="120" />
      <el-table-column prop="nickname" label="昵称" min-width="120" />
      <el-table-column prop="email" label="邮箱" min-width="180" />
      <el-table-column label="角色" width="120">
        <template #default="{ row }">
          <el-select :model-value="row.role" size="small" @change="(val) => changeRole(row, val)">
            <el-option v-for="item in ROLE_OPTIONS" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-switch
            :model-value="row.status"
            :active-value="1"
            :inactive-value="2"
            active-text="启用"
            inactive-text="禁用"
            inline-prompt
            @change="(val) => changeStatus(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="submission_count" label="提交数" width="90" />
      <el-table-column label="注册时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="最后登录" width="170">
        <template #default="{ row }">{{ formatTime(row.last_login_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="resetPassword(row)">重置密码</el-button>
          <el-button size="small" @click="renameUser(row)">改昵称</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="load"
      />
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminImportUsers, adminListUsers, adminUpdateUser } from '@/api'
import { formatTime } from '@/utils/format'

const ROLE_OPTIONS = [
  { value: 'admin', label: '管理员' },
  { value: 'setter', label: '出题人' },
  { value: 'user', label: '选手' }
]

const loading = ref(false)
const importing = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const roleFilter = ref('')

const load = async () => {
  loading.value = true
  try {
    const data = await adminListUsers({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      role: roleFilter.value || undefined
    })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const search = () => {
  page.value = 1
  load()
}

const handleImport = async ({ file }) => {
  importing.value = true
  try {
    const data = await adminImportUsers(file)
    ElMessage.success(`导入完成：新增 ${data.imported ?? 0} 条，跳过 ${data.skipped ?? 0} 条`)
    search()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    importing.value = false
  }
}

const changeRole = async (row, role) => {
  try {
    await adminUpdateUser(row.id, { role })
    row.role = role
    ElMessage.success('角色已更新')
  } catch (e) {
    load()
  }
}

const changeStatus = async (row, status) => {
  try {
    await adminUpdateUser(row.id, { status })
    row.status = status
    ElMessage.success(status === 1 ? '已启用' : '已禁用')
  } catch (e) {
    load()
  }
}

const resetPassword = async (row) => {
  try {
    const { value } = await ElMessageBox.prompt(`为用户 ${row.username} 设置新密码`, '重置密码', {
      inputType: 'password',
      inputPlaceholder: '请输入新密码',
      inputValidator: (val) => (val && val.length >= 6 ? true : '密码至少 6 位')
    })
    await adminUpdateUser(row.id, { password: value })
    ElMessage.success('密码已重置')
  } catch (e) {
    // 取消输入或接口失败时不再提示
  }
}

const renameUser = async (row) => {
  try {
    const { value } = await ElMessageBox.prompt('请输入新昵称', '修改昵称', {
      inputValue: row.nickname || '',
      inputValidator: (val) => (val && val.trim() ? true : '昵称不能为空')
    })
    await adminUpdateUser(row.id, { nickname: value.trim() })
    row.nickname = value.trim()
    ElMessage.success('昵称已更新')
  } catch (e) {
    // 取消输入或接口失败时不再提示
  }
}

onMounted(load)
</script>
