<template>
  <router-view />
</template>

<script setup>
import { watch } from 'vue'
import { applyTheme } from '@/utils/theme'
import { useUserStore } from '@/store/user'

const store = useUserStore()

// 站点主色变化时立即应用到根节点；衍生色由 CSS color-mix 自动跟随
watch(
  () => store.site.theme_color,
  (color) => applyTheme(color),
  { immediate: true }
)
</script>
