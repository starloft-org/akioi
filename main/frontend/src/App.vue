<template>
  <router-view />
</template>

<script setup>
import { watch } from 'vue'
import { applyTheme } from '@/utils/theme'
import { useUserStore } from '@/store/user'

const store = useUserStore()

// 登录用户自定义过主题色就用它，否则跟随站点主色；衍生色由 CSS color-mix 自动跟随
watch(
  [() => store.user?.theme_color, () => store.site.theme_color],
  ([mine, site]) => applyTheme(mine || site),
  { immediate: true }
)
</script>
