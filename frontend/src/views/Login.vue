<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-brand">{{ store.site.app_name }}</div>
      <div class="auth-sub">信息学奥赛在线评测系统</div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="请输入用户名" @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="请输入密码"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-button type="primary" class="auth-submit" :loading="loading" @click="handleLogin">登录</el-button>
      </el-form>
      <div class="auth-links">
        <el-link
          v-if="store.site.register_enabled"
          type="primary"
          :underline="false"
          @click="router.push('/register')"
        >
          注册账号
        </el-link>
      </div>
      <div v-if="store.site.wechat_enabled" class="auth-wechat">
        <el-button :loading="wechatLoading" @click="handleWechat">微信登录</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getWechatUrl, wechatCallback } from '@/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()
const route = useRoute()

const formRef = ref(null)
const loading = ref(false)
const wechatLoading = ref(false)
const form = reactive({ username: '', password: '' })

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

// 微信回调携带 code 与 state，换取凭证后跳转
const handleWechatCallback = async () => {
  try {
    const data = await wechatCallback({ code: route.query.code, state: route.query.state })
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
  loading.value = true
  try {
    await store.login({ username: form.username, password: form.password })
    ElMessage.success('登录成功')
    router.replace(route.query.redirect || '/')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    loading.value = false
  }
}

const handleWechat = async () => {
  wechatLoading.value = true
  try {
    const data = await getWechatUrl(window.location.href)
    if (data?.enabled && data.url) {
      window.location.href = data.url
      return
    }
    ElMessage.warning('微信登录未启用')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    wechatLoading.value = false
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
  margin-top: 12px;
  text-align: center;
}

.auth-wechat {
  margin-top: 16px;
}

.auth-wechat .el-button {
  width: 100%;
}
</style>
