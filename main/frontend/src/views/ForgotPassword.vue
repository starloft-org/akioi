<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-brand">重置密码</div>
      <div class="auth-sub">验证码仅用于找回密码，不支持短信登录</div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="form.phone" maxlength="11" />
        </el-form-item>
        <el-form-item label="短信验证码" prop="sms_code">
          <el-input v-model="form.sms_code" maxlength="6">
            <template #append>
              <el-button :disabled="countdown > 0 || sending" @click="sendCode">
                {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
              </el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="form.new_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认新密码" prop="confirm">
          <el-input v-model="form.confirm" type="password" show-password @keyup.enter="submit" />
        </el-form-item>
        <el-button type="primary" class="auth-submit" :loading="loading" @click="submit">重置密码</el-button>
      </el-form>
      <div class="auth-links">
        <el-link type="primary" :underline="false" @click="router.push('/login')">返回登录</el-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { resetPasswordBySms, sendSmsCode } from '@/api'

const router = useRouter()
const formRef = ref(null)
const loading = ref(false)
const sending = ref(false)
const countdown = ref(0)
let timer = null
const form = reactive({ phone: '', sms_code: '', new_password: '', confirm: '' })

const rules = {
  phone: [{ required: true, pattern: /^1[3-9]\d{9}$/, message: '请输入有效手机号', trigger: 'blur' }],
  sms_code: [{ required: true, pattern: /^\d{6}$/, message: '请输入 6 位验证码', trigger: 'blur' }],
  new_password: [{ required: true, min: 6, max: 64, message: '密码长度需在 6 到 64 个字符之间', trigger: 'blur' }],
  confirm: [{ validator: (_, value, callback) => value === form.new_password ? callback() : callback(new Error('两次密码不一致')), trigger: 'blur' }]
}

const startCountdown = () => {
  countdown.value = 60
  clearInterval(timer)
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) clearInterval(timer)
  }, 1000)
}

const sendCode = async () => {
  if (!/^1[3-9]\d{9}$/.test(form.phone)) {
    ElMessage.warning('请先输入有效手机号')
    return
  }
  sending.value = true
  try {
    await sendSmsCode({ phone: form.phone, purpose: 'reset_password' })
    startCountdown()
    ElMessage.success('如手机号已注册，验证码将发送到该号码')
  } finally {
    sending.value = false
  }
}

const submit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await resetPasswordBySms({ phone: form.phone, sms_code: form.sms_code, new_password: form.new_password })
    ElMessage.success('密码已重置，请重新登录')
    router.replace('/login')
  } finally {
    loading.value = false
  }
}

onBeforeUnmount(() => clearInterval(timer))
</script>

<style scoped>
.auth-page { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 24px; background: var(--akioi-bg); }
.auth-card { width: 100%; max-width: 380px; padding: 32px 28px; background: #fff; border: 1px solid var(--akioi-border); border-radius: var(--akioi-radius); }
.auth-brand { font-size: 22px; font-weight: 600; text-align: center; }
.auth-sub { margin: 4px 0 20px; text-align: center; font-size: 12px; color: var(--akioi-text-weak); }
.auth-submit { width: 100%; }
.auth-links { margin-top: 12px; text-align: center; }
</style>
