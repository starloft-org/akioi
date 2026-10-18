<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-brand">{{ store.site.app_name }}</div>
      <div class="auth-sub">信息学奥赛在线评测系统</div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item label="账号" prop="account">
          <el-input v-model="form.account" placeholder="手机号或用户名" @keyup.enter="handleLogin" />
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
        <el-link type="primary" :underline="false" @click="router.push('/forgot-password')">
          忘记密码
        </el-link>
      </div>
      <div v-if="store.site.wechat_enabled" class="auth-wechat">
        <el-button @click="wechatDialog.open()">微信扫码登录</el-button>
      </div>
      <WechatQrDialog ref="wechatDialog" purpose="login" @login="onWechatLogin" />
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { wechatCallback } from '@/api'
import WechatQrDialog from '@/components/WechatQrDialog.vue'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()
const route = useRoute()

const formRef = ref(null)
const loading = ref(false)
const wechatDialog = ref(null)
const form = reactive({ account: '', password: '' })

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
  loading.value = true
  try {
    await store.login({ account: form.account, password: form.password })
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
