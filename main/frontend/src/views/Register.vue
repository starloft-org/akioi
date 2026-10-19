<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-brand">{{ store.site.app_name }}</div>
      <div class="auth-sub">注册新账号</div>

      <el-alert
        v-if="!smsOK && !emailOK"
        type="warning"
        :closable="false"
        show-icon
        title="当前未开放注册"
        style="margin-bottom: 16px"
      />

      <el-form v-else ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item v-if="smsOK && emailOK">
          <el-radio-group v-model="channel">
            <el-radio-button label="sms">手机号注册</el-radio-button>
            <el-radio-button label="email">邮箱注册</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" maxlength="20" placeholder="4-20 位字母、数字、_ 或 -" />
        </el-form-item>

        <el-form-item v-if="channel === 'sms'" label="手机号" prop="phone">
          <el-input v-model="form.phone" maxlength="11" placeholder="中国大陆手机号" />
        </el-form-item>
        <el-form-item v-if="channel === 'sms'" label="短信验证码" prop="sms_code">
          <el-input v-model="form.sms_code" maxlength="6" placeholder="6 位验证码">
            <template #append>
              <el-button :disabled="countdown > 0 || sending" @click="handleSendCode">
                {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
              </el-button>
            </template>
          </el-input>
        </el-form-item>

        <el-form-item v-if="channel === 'email'" label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="用于接收验证码" />
        </el-form-item>
        <el-form-item v-if="channel === 'email'" label="邮件验证码" prop="code">
          <el-input v-model="form.code" maxlength="6" placeholder="6 位验证码">
            <template #append>
              <el-button :disabled="countdown > 0 || sending" @click="handleSendCode">
                {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
              </el-button>
            </template>
          </el-input>
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="6-64 位字符" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" placeholder="选填，默认与用户名相同" />
        </el-form-item>
        <el-button type="primary" class="auth-submit" :loading="loading" @click="handleRegister">注册</el-button>
      </el-form>
      <div class="auth-links">
        <el-link type="primary" :underline="false" @click="router.push('/login')">已有账号，去登录</el-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { sendEmailCode, sendSmsCode } from '@/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()

const formRef = ref(null)
const loading = ref(false)
const sending = ref(false)
const countdown = ref(0)
let timer = null
const form = reactive({ username: '', phone: '', sms_code: '', email: '', code: '', password: '', nickname: '' })
// 渠道由站点配置驱动：两个渠道都开放时允许切换，只开一个时自动锁定
const channel = ref('sms')

const smsOK = computed(() => !!store.site.register_channels?.sms)
const emailOK = computed(() => !!store.site.register_channels?.email)

watch([smsOK, emailOK], () => {
  if (channel.value === 'sms' && !smsOK.value && emailOK.value) {
    channel.value = 'email'
  } else if (channel.value === 'email' && !emailOK.value && smsOK.value) {
    channel.value = 'sms'
  }
})

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { pattern: /^[A-Za-z0-9_-]+$/, message: '只能包含字母、数字、下划线与短横线', trigger: 'blur' },
    { min: 4, max: 20, message: '用户名长度需在 4 到 20 个字符之间', trigger: 'blur' }
  ],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '手机号格式不正确', trigger: 'blur' }
  ],
  sms_code: [
    { required: true, message: '请输入短信验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
  ],
  code: [
    { required: true, message: '请输入邮件验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 64, message: '密码长度需在 6 到 64 个字符之间', trigger: 'blur' }
  ],
  nickname: [{ max: 32, message: '昵称长度不能超过 32 个字符', trigger: 'blur' }]
}

const startCountdown = (seconds = 60) => {
  countdown.value = seconds
  clearInterval(timer)
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) clearInterval(timer)
  }, 1000)
}

const handleSendCode = async () => {
  sending.value = true
  try {
    if (channel.value === 'sms') {
      if (!/^1[3-9]\d{9}$/.test(form.phone)) {
        ElMessage.warning('请先输入有效手机号')
        return
      }
      const data = await sendSmsCode({ phone: form.phone, purpose: 'register' })
      startCountdown(Math.min(Number(data.expires_in || 60), 60))
      ElMessage.success('验证码已发送')
    } else {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
        ElMessage.warning('请先输入有效邮箱')
        return
      }
      const data = await sendEmailCode({ email: form.email, purpose: 'register' })
      startCountdown(Math.min(Number(data.expires_in || 60), 60))
      ElMessage.success('验证码已发送，请查收邮件')
    }
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    sending.value = false
  }
}

const handleRegister = async () => {
  if (!formRef.value) {
    return
  }
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) {
    return
  }
  loading.value = true
  try {
    const payload = {
      username: form.username.trim(),
      channel: channel.value,
      password: form.password,
      nickname: form.nickname
    }
    if (channel.value === 'sms') {
      payload.phone = form.phone
      payload.sms_code = form.sms_code
    } else {
      payload.email = form.email
      payload.code = form.code
    }
    await store.register(payload)
    ElMessage.success('注册成功')
    router.replace('/')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  store.loadSite()
})

onBeforeUnmount(() => clearInterval(timer))
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
</style>
