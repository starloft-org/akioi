<template>
  <el-container class="admin-shell">
    <el-aside width="200px" class="admin-aside">
      <div class="admin-brand">{{ site.app_name }} 管理后台</div>
      <el-menu :default-active="activePath" router class="admin-menu">
        <el-menu-item index="/admin">概览</el-menu-item>
        <el-menu-item-group title="内容">
          <el-menu-item index="/admin/problems">题目管理</el-menu-item>
          <el-menu-item index="/admin/contests">比赛管理</el-menu-item>
          <el-menu-item index="/admin/submissions">评测记录</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="运营">
          <el-menu-item v-if="store.isAdmin" index="/admin/users">用户管理</el-menu-item>
          <el-menu-item index="/admin/announcements">公告管理</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="系统">
          <el-menu-item index="/admin/settings">系统设置</el-menu-item>
          <el-menu-item index="/admin/logs">日志查看</el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="admin-header">
        <span class="admin-header__title">{{ pageTitle }}</span>
        <div>
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
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const site = computed(() => store.site)
const activePath = computed(() => '/' + route.path.split('/').slice(1, 3).join('/'))
const pageTitle = computed(() => route.name || '管理后台')

const handleLogout = async () => {
  await store.logout()
  router.push('/login')
}
</script>

<style scoped>
.admin-menu :deep(.el-menu-item-group__title) {
  padding: 14px 20px 4px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--akioi-text-weak);
}
</style>
