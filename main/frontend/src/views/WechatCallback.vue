<template>
  <div class="auth-page">
    <div class="auth-card callback-card">
      <div class="status-icon" :class="`is-${phase}`">
        <span v-if="phase === 'processing'" class="spinner"></span>
        <span v-else-if="phase === 'error'">!</span>
        <span v-else>✓</span>
      </div>
      <h2 class="status-title">{{ title }}</h2>
      <p class="status-desc">{{ desc }}</p>
      <div class="status-actions">
        <el-button v-if="phase === 'error'" type="primary" @click="router.replace('/login')">
          返回登录
        </el-button>
        <el-button v-else-if="phase !== 'processing' && from === 'pc'" @click="router.replace('/')">
          在电脑上继续
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { wechatCallback } from '@/api'
import { useUserStore } from '@/store/user'

const route = useRoute()
const router = useRouter()
const store = useUserStore()

const phase = ref('processing')
const message = ref('')
const from = ref('h5')

const title = computed(() => {
  switch (phase.value) {
    case 'processing':
      return '正在完成授权'
    case 'bound':
      return '微信绑定成功'
    case 'login':
      return '登录成功'
    default:
      return '授权失败'
  }
})

const desc = computed(() => {
  if (phase.value === 'processing') {
    return '正在与微信确认授权信息，请稍候'
  }
  if (phase.value === 'error') {
    return message.value || '请重新发起授权'
  }
  if (from.value === 'pc') {
    return phase.value === 'bound' ? '请回到电脑，绑定已经完成' : '请回到电脑，登录已完成'
  }
  return phase.value === 'bound' ? '即将返回个人中心' : '即将进入站点'
})

onMounted(async () => {
  const code = route.query.code
  const state = route.query.state
  if (!code || !state) {
    phase.value = 'error'
    message.value = '缺少授权参数，请重新发起'
    return
  }
  try {
    const data = await wechatCallback({ code, state })
    from.value = data?.from || 'h5'

    if (data?.bound) {
      phase.value = 'bound'
      store.wechatBound = true
      if (from.value !== 'pc') {
        setTimeout(() => router.replace(data.redirect || '/profile'), 1200)
      }
      return
    }

    phase.value = 'login'
    if (from.value !== 'pc') {
      store.setToken(data.token)
      if (data.user) {
        store.user = data.user
      }
      setTimeout(() => router.replace(data.redirect || '/'), 800)
    }
  } catch (e) {
    phase.value = 'error'
    message.value = e?.response?.data?.message || '授权失败，请重新发起'
  }
})
</script>

<style scoped>
.auth-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
  background: var(--akioi-bg);
}

.auth-card {
  width: 100%;
  max-width: 380px;
  padding: 36px 28px;
  text-align: center;
  background: #fff;
  border: 1px solid var(--akioi-border);
  border-radius: var(--akioi-radius);
}

.status-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin: 0 auto 16px;
  font-size: 26px;
  font-weight: 600;
  color: #fff;
  border-radius: 50%;
}

.status-icon.is-processing {
  background: var(--akioi-primary-light);
}

.status-icon.is-login,
.status-icon.is-bound {
  background: #00a870;
}

.status-icon.is-error {
  background: #f54a45;
}

.status-icon.is-error span {
  font-size: 24px;
}

.spinner {
  width: 22px;
  height: 22px;
  border: 2px solid var(--akioi-primary);
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.status-title {
  margin: 0 0 8px;
  font-size: 18px;
  font-weight: 600;
}

.status-desc {
  margin: 0 0 20px;
  font-size: 13px;
  color: var(--akioi-text-weak);
}

.status-actions {
  min-height: 32px;
}
</style>
