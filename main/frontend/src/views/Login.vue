<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-brand">{{ store.site.app_name }}</div>
      <div class="auth-sub">信息学奥赛在线评测系统</div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item prop="account">
          <el-input
            v-model="form.account"
            size="large"
            placeholder="手机号或用户名"
            :prefix-icon="UserIcon"
            clearable
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            size="large"
            type="password"
            show-password
            placeholder="请输入密码"
            :prefix-icon="LockIcon"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-button type="primary" size="large" class="auth-submit" :loading="loading" @click="handleLogin">
          登录
        </el-button>
      </el-form>
      <div class="auth-links">
        <template v-if="store.site.register_enabled">
          <span class="muted">还没有账号？</span>
          <el-link type="primary" :underline="false" @click="router.push('/register')">注册账号</el-link>
        </template>
        <el-link type="primary" :underline="false" @click="router.push('/forgot-password')">忘记密码</el-link>
      </div>
      <div v-if="store.site.wechat_enabled" class="wechat-login">
        <div class="wechat-divider"><span>其他登录方式</span></div>
        <el-button class="wechat-btn" size="large" @click="handleWechat">
          <svg class="wechat-icon" viewBox="0 0 24 24" width="20" height="20" fill="currentColor" aria-hidden="true">
            <path d="M8.7 3C4.6 3 1.3 5.8 1.3 9.2c0 1.9 1.1 3.6 2.8 4.8l-.7 2.1 2.4-1.2c.9.3 1.9.4 2.9.4h.4a5.6 5.6 0 0 1-.2-1.5c0-3.2 3.1-5.8 6.9-5.8h.6C15.7 5.2 12.5 3 8.7 3Zm-2.4 3.4a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8Zm4.8 0a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8Z" />
            <path d="M22.7 14.1c0-2.8-2.8-5.1-6.2-5.1s-6.2 2.3-6.2 5.1 2.8 5.1 6.2 5.1c.8 0 1.5-.1 2.2-.3l1.9 1-.5-1.7c1.6-.9 2.6-2.4 2.6-4.1Zm-8.2-1.5a.8.8 0 1 1 0 1.5.8.8 0 0 1 0-1.5Zm4 0a.8.8 0 1 1 0 1.5.8.8 0 0 1 0-1.5Z" />
          </svg>
          {{ wechatLabel }}
        </el-button>
      </div>
      <WechatQrDialog ref="wechatDialog" purpose="login" @login="onWechatLogin" />
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Lock, User } from '@element-plus/icons-vue'
import { getWechatUrl, wechatCallback } from '@/api'
import { verifyCaptcha } from '@/utils/captcha'
import WechatQrDialog from '@/components/WechatQrDialog.vue'
import { useUserStore } from '@/store/user'

// 输入框前缀图标用组件常量，模板里直接绑定
const UserIcon = User
const LockIcon = Lock

const store = useUserStore()
const router = useRouter()
const route = useRoute()

const formRef = ref(null)
const loading = ref(false)
const wechatDialog = ref(null)
const form = reactive({ account: '', password: '' })

// 公众号授权只能微信内置浏览器打开，其余环境走二维码
const isWechatBrowser = /MicroMessenger/i.test(navigator.userAgent)
const wechatLabel = computed(() => (isWechatBrowser ? '微信登录' : '微信扫码登录'))

const rules = {
  account: [{ required: true, message: '请输入手机号或用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

// 微信回调携带 code 与 state：登录流程换 token，绑定流程只确认绑定结果
const handleWechatCallback = async () => {
  try {
    const data = await wechatCallback({ code: route.query.code, state: route.query.state })
    if (data?.bound) {
      ElMessage.success('微信绑定成功')
      await store.loadMe()
      router.replace(data.redirect || '/profile')
      return
    }
    store.setToken(data.token)
    router.replace(data.redirect || '/')
  } catch (e) {
    // 回调失败时停留在登录页，错误提示由请求层处理
  }
}

const handleLogin = async () => {
  if (!formRef.value) {
    return
  }
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) {
    return
  }
  // 登录前做人机验证，拦截脚本撞库；后端未开启时票据为空、照常提交
  let captcha
  try {
    captcha = await verifyCaptcha('account')
  } catch (e) {
    // 用户主动取消时提示；配置/网络错误已由请求层统一提示
    if (e?.message === '已取消人机验证') {
      ElMessage.warning(e.message)
    }
    return
  }
  loading.value = true
  try {
    await store.login({
      account: form.account,
      password: form.password,
      captcha_ticket: captcha.ticket,
      captcha_randstr: captcha.randstr
    })
    ElMessage.success('登录成功')
    router.replace(route.query.redirect || '/')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    loading.value = false
  }
}

// PC 端扫码成功后由对话框带回 token
const onWechatLogin = (data) => {
  store.setToken(data.token)
  if (data.user) {
    store.user = data.user
  }
  ElMessage.success('登录成功')
  router.replace(data.redirect || route.query.redirect || '/')
}

// 微信内直接跳转授权页，非微信环境弹出二维码
const handleWechat = async () => {
  if (!isWechatBrowser) {
    wechatDialog.value.open()
    return
  }
  try {
    const data = await getWechatUrl({
      purpose: 'login',
      redirect: route.query.redirect || '/',
      from: 'h5'
    })
    if (data?.enabled && data.url) {
      window.location.href = data.url
      return
    }
    ElMessage.warning('微信登录未启用')
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

onMounted(() => {
  store.loadSite()
  if (route.query.code && route.query.state) {
    handleWechatCallback()
  }
})
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--akioi-bg);
}

.auth-card {
  width: 100%;
  max-width: 380px;
  padding: 32px 28px;
  background: #fff;
  border: 1px solid var(--akioi-border);
  border-radius: var(--akioi-radius);
}

.auth-brand {
  font-size: 22px;
  font-weight: 600;
  text-align: center;
}

.auth-sub {
  margin: 4px 0 20px;
  text-align: center;
  font-size: 12px;
  color: var(--akioi-text-weak);
}

.auth-submit {
  width: 100%;
}

.auth-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-top: 16px;
  font-size: 13px;
}

/* 其他登录方式：分隔线 + 微信绿图标按钮 */
.wechat-login {
  margin-top: 4px;
}

.wechat-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 20px 0 14px;
  font-size: 12px;
  color: var(--akioi-text-weak);
}

.wechat-divider::before,
.wechat-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--akioi-border);
}

.wechat-btn {
  width: 100%;
}

.wechat-icon {
  margin-right: 8px;
  color: #07c160;
}

.auth-wechat {
  margin-top: 16px;
}

.auth-wechat .el-button {
  width: 100%;
}
</style>
