<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="app-header__inner">
        <router-link to="/" class="app-logo">
          {{ site.app_name }}
          <span class="app-logo__sub">在线评测</span>
        </router-link>
        <nav class="app-nav">
          <router-link to="/problems">题库</router-link>
          <router-link to="/contests">比赛</router-link>
          <router-link to="/ranking">排行榜</router-link>
          <router-link to="/discussions">讨论</router-link>
          <router-link to="/submissions">评测记录</router-link>
        </nav>
        <div class="app-header__right">
          <template v-if="store.user">
            <router-link to="/profile" class="app-user">
              {{ store.user.nickname || store.user.username }}
            </router-link>
            <el-button v-if="store.isAdmin" size="small" @click="router.push('/admin')">管理后台</el-button>
            <el-button size="small" text @click="handleLogout">退出</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="router.push('/login')">登录</el-button>
            <el-button v-if="site.register_enabled" size="small" type="primary" @click="router.push('/register')">
              注册
            </el-button>
          </template>
        </div>
      </div>
    </header>

    <main class="app-main" :class="{ 'app-main--wide': route.meta.wide }">
      <router-view />
    </main>

    <footer class="app-footer">
      {{ site.app_name }} · 信息学奥赛在线评测系统
    </footer>
  </div>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()
const route = useRoute()
const site = computed(() => store.site)

onMounted(async () => {
  await store.loadSite()
  if (store.token && !store.user) {
    await store.loadMe()
  }
})

const handleLogout = async () => {
  await store.logout()
  ElMessage.success('已退出登录')
  router.push('/')
}
</script>
