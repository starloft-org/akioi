<template>
  <!-- 用户名悬停菜单：把个人主页、账号设置、管理后台、退出登录收进一处，导航栏只留栏目 -->
  <el-dropdown
    v-if="store.user"
    ref="menuRef"
    trigger="hover"
    placement="bottom-end"
    :show-timeout="60"
    :hide-timeout="150"
    popper-class="app-user-menu"
    @command="handleCommand"
    @visible-change="menuVisible = $event"
  >
    <button
      type="button"
      class="app-user"
      :class="{ 'is-open': menuVisible }"
      aria-haspopup="menu"
      :aria-expanded="menuVisible"
      @click="openMenu"
      @keydown.enter.prevent="openMenu"
      @keydown.space.prevent="openMenu"
      @keydown.down.prevent="openMenu"
    >
      <el-avatar :size="28" :src="store.user.avatar || undefined" class="app-user__avatar">
        {{ initial }}
      </el-avatar>
      <span class="app-user__name">{{ displayName }}</span>
      <el-icon class="app-user__caret"><CaretBottom /></el-icon>
    </button>

    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="home" :icon="UserIcon">个人主页</el-dropdown-item>
        <el-dropdown-item command="profile" :icon="SettingIcon">账号设置</el-dropdown-item>
        <el-dropdown-item v-if="store.isAdmin" command="admin" :icon="MonitorIcon" divided>
          管理后台
        </el-dropdown-item>
        <el-dropdown-item command="logout" :icon="SwitchButtonIcon" divided>退出登录</el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { CaretBottom, Monitor, Setting, SwitchButton, User } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'

// 图标以组件常量传入，模板里直接绑定
const UserIcon = User
const SettingIcon = Setting
const MonitorIcon = Monitor
const SwitchButtonIcon = SwitchButton

const store = useUserStore()
const router = useRouter()
const menuRef = ref(null)
const menuVisible = ref(false)

const displayName = computed(() => store.user?.nickname || store.user?.username || '')
const initial = computed(() => (displayName.value || '?').charAt(0).toUpperCase())

// 悬停展开已由 el-dropdown 负责，这里补上点击与键盘触发
const openMenu = () => {
  menuRef.value?.handleOpen()
}

const handleCommand = async (command) => {
  if (command === 'logout') {
    await store.logout()
    ElMessage.success('已退出登录')
    router.push('/')
    return
  }
  if (command === 'admin') {
    router.push('/admin')
    return
  }
  if (command === 'profile') {
    router.push('/profile')
    return
  }
  if (command === 'home') {
    // 未拿到用户 id 时退回账号设置页，避免跳到无效地址
    router.push(store.user?.id ? `/users/${store.user.id}` : '/profile')
  }
}
</script>
