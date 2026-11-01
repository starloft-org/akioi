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
      <h3 class="panel__title">个性化</h3>
      <el-form label-width="100px">
        <el-form-item label="主题色">
          <div class="appearance-row">
            <el-color-picker
              :model-value="appearance.theme_color"
              :predefine="predefineColors"
              @change="handleThemeColor"
            />
            <el-button link type="primary" :disabled="!appearance.theme_color" @click="resetThemeColor">
              跟随站点主题色
            </el-button>
            <span class="muted">只影响你自己的界面，管理员修改站点主题色时未被自定义的页面照常跟随</span>
          </div>
        </el-form-item>
        <el-form-item label="主页背景">
          <div class="cover-field">
            <div class="cover-preview" :style="coverStyle">
              <span v-if="!appearance.cover_image" class="muted">未设置</span>
            </div>
            <div class="cover-actions">
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                accept=".png,.jpg,.jpeg,.webp"
                :on-change="handleCoverChange"
              >
                <el-button :loading="coverLoading">{{ appearance.cover_image ? '更换背景图' : '上传背景图' }}</el-button>
              </el-upload>
              <el-button v-if="appearance.cover_image" :loading="coverLoading" @click="handleRemoveCover">
                移除背景
              </el-button>
              <div class="muted">支持 png/jpg/jpeg/webp，尺寸建议 1200×300，展示在你个人主页顶部</div>
            </div>
          </div>
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

    <div v-if="store.site.wechat_enabled" class="panel">
      <h3 class="panel__title">第三方账号</h3>
      <div class="bind-row">
        <div>
          <div class="bind-name">微信</div>
          <div class="muted">
            {{ store.wechatBound ? '已绑定，可用微信扫码登录' : '未绑定，绑定后可用微信扫码直接登录' }}
          </div>
        </div>
        <el-button v-if="!store.wechatBound" type="primary" plain @click="handleBindWechat">
          绑定微信
        </el-button>
        <el-button v-else :loading="unbinding" @click="handleUnbindWechat">解除绑定</el-button>
      </div>
      <WechatQrDialog ref="wechatDialog" purpose="bind" @bound="onWechatBound" />
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  changePassword,
  deleteCover,
  getWechatUrl,
  unbindWechat,
  updateMe,
  uploadAvatar,
  uploadCover,
  wechatCallback
} from '@/api'
import WechatQrDialog from '@/components/WechatQrDialog.vue'
import { useUserStore } from '@/store/user'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const profileFormRef = ref(null)
const passwordFormRef = ref(null)
const saving = ref(false)
const changing = ref(false)
const avatarLoading = ref(false)
const unbinding = ref(false)
const wechatDialog = ref(null)

const form = reactive({ nickname: '', signature: '', school: '', grade: '' })
const appearance = reactive({ theme_color: '', cover_image: '' })
const coverLoading = ref(false)
const predefineColors = ['#1CD5C7', '#409EFF', '#5470C6', '#00A870', '#F7BA1E', '#F54A45', '#7B61FF', '#34495E']
const coverStyle = computed(() =>
  appearance.cover_image
    ? { backgroundImage: `url(${appearance.cover_image})` }
    : {}
)
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

const saveThemeColor = async (value) => {
  try {
    const data = await updateMe({ theme_color: value })
    store.user = data.user
    ElMessage.success(value ? '主题色已更新' : '已跟随站点主题色')
  } catch (e) {
    // 失败时回退到服务端保存的值
    appearance.theme_color = store.user?.theme_color || ''
  }
}

const handleThemeColor = (value) => {
  appearance.theme_color = value || ''
  saveThemeColor(appearance.theme_color)
}

const resetThemeColor = () => {
  appearance.theme_color = ''
  saveThemeColor('')
}

const handleCoverChange = async (uploadFile) => {
  if (!uploadFile?.raw) {
    return
  }
  coverLoading.value = true
  try {
    const data = await uploadCover(uploadFile.raw)
    appearance.cover_image = data.cover_image
    if (store.user) {
      store.user.cover_image = data.cover_image
    }
    ElMessage.success('背景图已更新')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    coverLoading.value = false
  }
}

const handleRemoveCover = async () => {
  coverLoading.value = true
  try {
    await deleteCover()
    appearance.cover_image = ''
    if (store.user) {
      store.user.cover_image = ''
    }
    ElMessage.success('已移除背景图')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    coverLoading.value = false
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

const onWechatBound = () => {
  store.wechatBound = true
  ElMessage.success('微信绑定成功')
}

const isWechatBrowser = /MicroMessenger/i.test(navigator.userAgent)

// 微信内直接跳转授权，其余环境弹二维码
const handleBindWechat = async () => {
  if (!isWechatBrowser) {
    wechatDialog.value.open()
    return
  }
  try {
    const data = await getWechatUrl({ purpose: 'bind', redirect: '/profile', from: 'h5' })
    if (data?.enabled && data.url) {
      window.location.href = data.url
      return
    }
    ElMessage.warning('微信登录未启用')
  } catch (e) {
    // 错误提示由请求层统一处理
  }
}

const handleUnbindWechat = async () => {
  try {
    await ElMessageBox.confirm(
      '解除后将不能再用微信扫码登录。若该账号由微信创建且尚未设置密码，解绑后可能无法登录，请先绑定手机号或修改密码。',
      '解除微信绑定',
      {
        confirmButtonText: '解除绑定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
  } catch (e) {
    return
  }
  unbinding.value = true
  try {
    await unbindWechat()
    store.wechatBound = false
    ElMessage.success('已解除微信绑定')
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    unbinding.value = false
  }
}

// 微信授权完成后会回到本页并带上 code 与 state，这里完成绑定
const handleWechatCallback = async () => {
  try {
    const data = await wechatCallback({ code: route.query.code, state: route.query.state })
    if (data?.bound) {
      store.wechatBound = true
      ElMessage.success('微信绑定成功')
    }
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    router.replace('/profile')
  }
}

onMounted(() => {
  const user = store.user || {}
  form.nickname = user.nickname || ''
  form.signature = user.signature || ''
  form.school = user.school || ''
  form.grade = user.grade || ''
  appearance.theme_color = user.theme_color || ''
  appearance.cover_image = user.cover_image || ''
  if (route.query.code && route.query.state) {
    handleWechatCallback()
  }
})
</script>

<style scoped>
.avatar-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.bind-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 4px 0;
}

.appearance-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.cover-field {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  width: 100%;
}

.cover-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 240px;
  height: 72px;
  border: 1px dashed var(--akioi-border);
  border-radius: 8px;
  background-color: var(--akioi-bg-soft);
  background-size: cover;
  background-position: center;
  font-size: 12px;
}

.cover-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

@media (max-width: 640px) {
  .cover-field {
    flex-direction: column;
  }

  .cover-preview {
    width: 100%;
  }
}

.bind-name {
  margin-bottom: 2px;
  font-weight: 600;
}
</style>
