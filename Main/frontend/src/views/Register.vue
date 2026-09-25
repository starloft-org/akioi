<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-brand">{{ store.site.app_name }}</div>
      <div class="auth-sub">注册新账号</div>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="3-20 位字母、数字、_ 或 -" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="6-64 位字符" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" placeholder="选填，默认与用户名相同" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="选填" />
        </el-form-item>
        <el-form-item v-if="store.site.register_invite_code_required" label="邀请码" prop="invite_code">
          <el-input v-model="form.invite_code" placeholder="请输入邀请码" />
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
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const router = useRouter()

const formRef = ref(null)
const loading = ref(false)
const form = reactive({ username: '', password: '', nickname: '', email: '', invite_code: '' })

// 邮箱选填，仅在填写时校验格式
const validateEmail = (rule, value, callback) => {
  if (!value) {
    callback()
    return
  }
  if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
    callback()
    return
  }
  callback(new Error('邮箱格式不正确'))
}

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度需在 3 到 20 个字符之间', trigger: 'blur' },
    { pattern: /^[A-Za-z0-9_-]+$/, message: '只能包含字母、数字、下划线与短横线', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 64, message: '密码长度需在 6 到 64 个字符之间', trigger: 'blur' }
  ],
  nickname: [{ max: 32, message: '昵称长度不能超过 32 个字符', trigger: 'blur' }],
  email: [{ validator: validateEmail, trigger: 'blur' }],
  invite_code: [{ required: true, message: '请输入邀请码', trigger: 'blur' }]
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
    await store.register({
      username: form.username,
      password: form.password,
      nickname: form.nickname,
      email: form.email,
      invite_code: form.invite_code
    })
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
