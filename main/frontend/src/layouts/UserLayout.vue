<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="app-header__inner">
        <button class="app-nav-toggle" type="button" aria-label="展开导航" @click="navOpen = !navOpen">
          {{ navOpen ? '×' : '☰' }}
        </button>
        <router-link to="/" class="app-logo" @click="navOpen = false">
          {{ site.app_name }}
          <span class="app-logo__sub">在线评测</span>
        </router-link>
        <nav class="app-nav" :class="{ 'is-open': navOpen }">
          <router-link to="/problems" @click="navOpen = false">题库</router-link>
          <router-link to="/contests" @click="navOpen = false">比赛</router-link>
          <router-link to="/teams" @click="navOpen = false">团队</router-link>
          <router-link v-if="store.user" to="/my/problems" @click="navOpen = false">我的题目</router-link>
          <router-link v-if="store.user" to="/problem-sets" @click="navOpen = false">题单</router-link>
          <router-link to="/ranking" @click="navOpen = false">排行榜</router-link>
          <router-link to="/discussions" @click="navOpen = false">讨论</router-link>
          <router-link to="/submissions" @click="navOpen = false">评测记录</router-link>
        </nav>
        <div class="app-header__right">
          <UserMenu v-if="store.user" />
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
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import UserMenu from '@/components/UserMenu.vue'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()
const route = useRoute()
const site = computed(() => store.site)

// 窄屏折叠导航的展开状态，跳转页面后自动收起
const navOpen = ref(false)

watch(
  () => route.fullPath,
  () => {
    navOpen.value = false
  }
)

onMounted(async () => {
  await store.loadSite()
  if (store.token && !store.user) {
    await store.loadMe()
  }
})
</script>
