<template>
  <div class="not-found">
    <div class="not-found__code">404</div>
    <div class="not-found__text">页面不存在或已被移除</div>
    <el-button type="primary" @click="router.replace('/')">返回首页</el-button>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<style scoped>
.not-found {
  padding: 80px 16px;
  text-align: center;
}

.not-found__code {
  font-size: 64px;
  font-weight: 600;
  color: var(--akioi-text-weak);
}

.not-found__text {
  margin: 12px 0 24px;
  color: var(--akioi-text-weak);
}
</style>
