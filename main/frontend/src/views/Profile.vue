<template>
  <div>
    <div class="panel">
      <h3 class="panel__title">基本资料</h3>
      <div class="avatar-row">
        <el-avatar :size="64" :src="store.user?.avatar || undefined">{{ initial }}</el-avatar>
        <el-upload
          :auto-upload="false"
          :show-file-list="false"
          accept=".png,.jpg,.jpeg,.gif,.webp"
          :on-change="handleAvatarChange"
        >
          <el-button :loading="avatarLoading">上传头像</el-button>
        </el-upload>
        <span class="muted">支持 png/jpg/jpeg/gif/webp</span>
      </div>

      <el-form ref="profileFormRef" :model="form" :rules="profileRules" label-width="80px">
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" placeholder="请输入昵称" />
        </el-form-item>
        <el-form-item label="签名" prop="signature">
          <el-input v-model="form.signature" type="textarea" :rows="3" placeholder="介绍一下自己" />
        </el-form-item>
        <el-form-item label="学校" prop="school">
          <el-input v-model="form.school" placeholder="请输入学校" />
        </el-form-item>
        <el-form-item label="年级" prop="grade">
          <el-input v-model="form.grade" placeholder="请输入年级" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input :model-value="store.user?.phone || '未绑定'" disabled />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="panel">
      <h3 class="panel__title">修改密码</h3>
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="100px">
        <el-form-item label="原密码" prop="old_password">
          <el-input v-model="passwordForm.old_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="passwordForm.new_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认新密码" prop="confirm">
          <el-input v-model="passwordForm.confirm" type="password" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="changing" @click="handleChangePassword">修改密码</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { changePassword, updateMe, uploadAvatar } from '@/api'
import { useUserStore } from '@/store/user'

const store = useUserStore()

const profileFormRef = ref(null)
const passwordFormRef = ref(null)
const saving = ref(false)
const changing = ref(false)
const avatarLoading = ref(false)

const form = reactive({ nickname: '', signature: '', school: '', grade: '' })
const passwordForm = reactive({ old_password: '', new_password: '', confirm: '' })

const initial = computed(() => (store.user?.nickname || store.user?.username || '?').charAt(0).toUpperCase())

const validateConfirm = (rule, value, callback) => {
  if (value !== passwordForm.new_password) {
    callback(new Error('两次输入的新密码不一致'))
    return
  }
  callback()
}

const profileRules = {
  nickname: [{ max: 32, message: '昵称长度不能超过 32 个字符', trigger: 'blur' }],
  signature: [{ max: 255, message: '签名长度不能超过 255 个字符', trigger: 'blur' }]
}

const passwordRules = {
  old_password: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 64, message: '新密码长度需在 6 到 64 个字符之间', trigger: 'blur' }
  ],
  confirm: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' }
  ]
}

const handleSave = async () => {
  if (!profileFormRef.value) {
    return
  }
  const valid = await profileFormRef.value.validate().catch(() => false)
  if (!valid) {
    return
  }
  saving.value = true
  try {
    const data = await updateMe({ ...form })
    store.user = data.user
    ElMessage.success('保存成功')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}

const handleAvatarChange = async (uploadFile) => {
  if (!uploadFile?.raw) {
    return
  }
  avatarLoading.value = true
  try {
    const data = await uploadAvatar(uploadFile.raw)
    if (store.user) {
      store.user.avatar = data.avatar
    }
    ElMessage.success('头像已更新')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    avatarLoading.value = false
  }
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) {
    return
  }
  const valid = await passwordFormRef.value.validate().catch(() => false)
  if (!valid) {
    return
  }
  changing.value = true
  try {
		const data = await changePassword({
      old_password: passwordForm.old_password,
      new_password: passwordForm.new_password
    })
		store.setToken(data.token)
    ElMessage.success('密码已修改')
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm = ''
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    changing.value = false
  }
}

onMounted(() => {
  const user = store.user || {}
  form.nickname = user.nickname || ''
  form.signature = user.signature || ''
  form.school = user.school || ''
  form.grade = user.grade || ''
})
</script>

<style scoped>
.avatar-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}
</style>
