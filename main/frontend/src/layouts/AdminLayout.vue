<template>
  <el-container class="admin-shell">
    <div v-if="sidebarOpen" class="admin-mask" @click="sidebarOpen = false" />
    <el-aside width="200px" class="admin-aside" :class="{ 'is-open': sidebarOpen }">
      <div class="admin-brand">{{ site.app_name }} 管理后台</div>
      <el-menu :default-active="activePath" router class="admin-menu" @select="sidebarOpen = false">
        <el-menu-item index="/admin">概览</el-menu-item>
        <el-menu-item-group title="运营">
          <el-menu-item v-if="store.hasPermission('users')" index="/admin/users">用户管理</el-menu-item>
          <el-menu-item v-if="store.hasPermission('announcements')" index="/admin/announcements">公告管理</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="内容">
          <el-menu-item v-if="store.hasPermission('problems')" index="/admin/problems">题目管理</el-menu-item>
          <el-menu-item v-if="store.hasPermission('problem_banks')" index="/admin/problem-banks">题库管理</el-menu-item>
          <el-menu-item v-if="store.hasPermission('contests')" index="/admin/contests">比赛管理</el-menu-item>
          <el-menu-item v-if="store.hasPermission('submissions')" index="/admin/submissions">评测记录</el-menu-item>
          <el-menu-item v-if="store.hasPermission('solutions')" index="/admin/solutions">题解审核</el-menu-item>
          <el-menu-item v-if="store.hasPermission('judges')" index="/admin/judges">测评机管理</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="系统">
          <el-menu-item v-if="store.isSuperAdmin" index="/admin/admins">管理员管理</el-menu-item>
          <el-menu-item v-if="store.hasPermission('settings')" index="/admin/settings">系统设置</el-menu-item>
          <el-menu-item v-if="store.hasPermission('backups')" index="/admin/backups">数据备份</el-menu-item>
          <el-menu-item v-if="store.hasPermission('logs')" index="/admin/logs">日志查看</el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="admin-header">
        <div class="admin-header__left">
          <button class="admin-menu-toggle" type="button" aria-label="展开菜单" @click="sidebarOpen = !sidebarOpen">
            {{ sidebarOpen ? '×' : '☰' }}
          </button>
          <span class="admin-header__title">{{ pageTitle }}</span>
        </div>
        <div class="admin-header__actions">
          <el-button size="small" text @click="router.push('/')">返回前台</el-button>
          <el-button size="small" text @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-main class="admin-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const site = computed(() => store.site)
const activePath = computed(() => '/' + route.path.split('/').slice(1, 3).join('/'))
const pageTitle = computed(() => route.name || '管理后台')

// 窄屏侧栏抽屉的展开状态，切换页面后自动收起
const sidebarOpen = ref(false)

watch(
  () => route.fullPath,
  () => {
    sidebarOpen.value = false
  }
)

const handleLogout = async () => {
  await store.logout()
  router.push('/login')
}
</script>

<style scoped>
.admin-header__left {
  display: flex;
  align-items: center;
  min-width: 0;
}

.admin-header__actions {
  display: flex;
  align-items: center;
  flex: none;
}

.admin-menu :deep(.el-menu-item-group__title) {
  padding: 14px 20px 4px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--akioi-text-weak);
}
</style>
