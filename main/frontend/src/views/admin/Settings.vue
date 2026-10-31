<template>
  <div v-loading="loading">
    <el-alert
      title="本页设置保存后立即生效，无需重启服务。数据库、Redis 与文件目录仍从 .env 读取；登录密钥与第三方密钥在下方「密钥与集成」中维护，同样保存在数据库中。"
      type="info"
      :closable="false"
      show-icon
      class="settings-alert"
    />

    <div class="panel">
      <h3 class="panel__title">站点与登录</h3>
      <el-form :model="form" label-width="150px">
        <el-row :gutter="24">
          <el-col :xs="24" :md="12">
            <el-form-item label="站点名称">
              <el-input v-model="form.app_name" maxlength="64" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="登录有效期">
              <el-input-number v-model="form.jwt_ttl_hours" :min="1" :max="8760" />
              <span class="unit">小时</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="主题色">
              <el-color-picker v-model="form.theme_color" :predefine="predefineColors" />
              <span class="unit">影响全站按钮、链接与品牌色</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="开放注册">
              <el-switch v-model="form.register_enabled" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="评论区">
              <el-switch v-model="form.comments_enabled" />
              <span class="muted" style="margin-left: 8px">关闭后讨论区暂不可发表与编辑评论</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="微信扫码登录">
              <el-switch v-model="form.wechat_enabled" />
              <secret-tag label="微信密钥" :configured="secretStatus.wechat_credentials_configured" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="微信回调地址">
              <el-input v-model="form.wechat_redirect_uri" placeholder="https://example.com/api/v1/auth/wechat/callback" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="panel">
      <h3 class="panel__title">人机验证码</h3>
      <el-form :model="form" label-width="150px">
        <el-form-item label="启用行为验证码">
          <el-switch v-model="form.captcha_enabled" />
          <secret-tag label="验证码密钥" :configured="secretStatus.captcha_credentials_configured" />
        </el-form-item>
        <p class="muted settings-note">
          启用后在登录、发送短信/邮件验证码时弹出滑动拼图验证。API 密钥在下方「密钥与集成」中填写，留空时复用短信密钥；
          验证码应用 ID 与 JS 地址由平台按场景自动下发。
        </p>
      </el-form>
    </div>

    <div class="panel">
      <h3 class="panel__title">邮件注册与密码重置</h3>
      <el-form :model="form" label-width="150px">
        <el-row :gutter="24">
          <el-col :span="24">
            <el-form-item label="启用邮件服务">
              <el-switch v-model="form.email_enabled" />
              <secret-tag label="SMTP 密钥" :configured="secretStatus.smtp_credentials_configured" />
            </el-form-item>
          </el-col>
        </el-row>
        <p class="muted settings-note">
          SMTP 服务器、端口、账号与发件人在下方「密钥与集成」中填写，保存后立即生效。
          短信与邮件都未开放时，站点不支持注册。
        </p>
      </el-form>
    </div>

    <div class="panel">
      <h3 class="panel__title">短信注册与密码重置</h3>
      <el-form :model="form" label-width="150px">
        <el-row :gutter="24">
          <el-col :xs="24" :md="12">
            <el-form-item label="启用短信服务">
              <el-switch v-model="form.sms_enabled" />
              <secret-tag label="StarLoft 密钥" :configured="secretStatus.sms_credentials_configured" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="接口地址">
              <el-input v-model="form.sms_api_base_url" placeholder="https://api.starloft.cn/v2" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="注册模板 ID">
              <el-input v-model="form.sms_register_template_id" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="重置密码模板 ID">
              <el-input v-model="form.sms_reset_template_id" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="验证码有效期">
              <el-input-number v-model="form.sms_code_ttl_minutes" :min="1" :max="30" />
              <span class="unit">分钟</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="发送间隔">
              <el-input-number v-model="form.sms_send_interval_sec" :min="30" :max="3600" />
              <span class="unit">秒</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="请求超时">
              <el-input-number v-model="form.sms_request_timeout_sec" :min="1" :max="120" />
              <span class="unit">秒</span>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <p class="muted settings-note">两个短信模板均应包含验证码、有效分钟数两个参数。</p>
    </div>

    <div class="panel">
      <h3 class="panel__title">密钥与集成</h3>
      <el-form :model="secrets" label-width="150px">
        <el-alert
          title="密钥保存在数据库中，保存后立即生效。已保存的密钥不会回显，留空表示保持原值；勾选下方复选框可清除整组配置。"
          type="info"
          :closable="false"
          show-icon
          class="settings-inline-alert"
        />
        <el-row :gutter="24">
          <el-col :xs="24" :md="12">
            <el-form-item label="微信 AppID">
              <el-input v-model="secrets.wechat_app_id" placeholder="公众号 AppID，留空表示不使用" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="微信 AppSecret">
              <el-input
                v-model="secrets.wechat_app_secret"
                type="password"
                show-password
                :placeholder="secretStatus.wechat_credentials_configured ? '已配置，留空表示不修改' : '未配置'"
              />
              <el-checkbox v-model="clearGroups.wechat" class="clear-box">清除微信配置</el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="开放平台 AppID">
              <el-input v-model="secrets.wechat_open_app_id" placeholder="PC 扫码登录用，留空表示不支持扫码" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="开放平台 Secret">
              <el-input
                v-model="secrets.wechat_open_app_secret"
                type="password"
                show-password
                :placeholder="secretStatus.wechat_open_configured ? '已配置，留空表示不修改' : '未配置'"
              />
              <el-checkbox v-model="clearGroups.wechat_open" class="clear-box">清除开放平台配置</el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="短信 API Key">
              <el-input v-model="secrets.sms_api_key" placeholder="留空表示不修改" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="短信 API Secret">
              <el-input
                v-model="secrets.sms_api_secret"
                type="password"
                show-password
                :placeholder="secretStatus.sms_credentials_configured ? '已配置，留空表示不修改' : '未配置'"
              />
              <el-checkbox v-model="clearGroups.sms" class="clear-box">清除短信密钥</el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="SMTP 服务器">
              <el-input v-model="secrets.smtp_host" placeholder="smtp.example.com" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="SMTP 端口">
              <el-input-number v-model="secrets.smtp_port" :min="0" :max="65535" />
              <span class="unit">465 走隐式 TLS</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="SMTP 账号">
              <el-input v-model="secrets.smtp_username" placeholder="留空表示不认证" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="SMTP 密码">
              <el-input
                v-model="secrets.smtp_password"
                type="password"
                show-password
                :placeholder="secretStatus.smtp_credentials_configured ? '已配置，留空表示不修改' : '未配置'"
              />
              <el-checkbox v-model="clearGroups.smtp" class="clear-box">清除邮件配置</el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="发件人地址">
              <el-input v-model="secrets.smtp_from" placeholder="noreply@example.com" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="验证码 API Key">
              <el-input v-model="secrets.captcha_api_key" placeholder="留空时复用短信密钥" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="验证码 API Secret">
              <el-input
                v-model="secrets.captcha_api_secret"
                type="password"
                show-password
                :placeholder="secretStatus.captcha_credentials_configured ? '已配置，留空表示不修改' : '未配置'"
              />
              <el-checkbox v-model="clearGroups.captcha" class="clear-box">清除验证码密钥</el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="评测节点同步密钥">
              <el-input
                v-model="secrets.judge_sync_secret"
                type="password"
                show-password
                :placeholder="secretStatus.judge_sync_secret_configured ? '已配置，留空表示不修改' : '未配置'"
              />
              <el-checkbox v-model="clearGroups.judge" class="clear-box">清除同步密钥</el-checkbox>
              <p class="muted settings-note">
                必须与各评测节点 .env 中的 JUDGE_SYNC_SECRET 保持一致，修改后请同步更新评测节点并重启评测服务。
              </p>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="登录密钥">
              <secret-tag label="JWT 密钥" :configured="secretStatus.jwt_secret_configured" />
              <el-button class="settings-inline-button" :disabled="regenerateJWT" @click="confirmRegenerateJWT">
                {{ regenerateJWT ? '保存后重新生成' : '重新生成' }}
              </el-button>
              <span class="muted">重新生成会让所有用户退出登录，请谨慎操作</span>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="panel">
      <h3 class="panel__title">访问与上传限制</h3>
      <el-form :model="form" label-width="150px">
        <el-row :gutter="24">
          <el-col :xs="24" :md="12">
            <el-form-item label="每分钟请求限制">
              <el-input-number v-model="form.rate_limit_per_minute" :min="0" :max="100000" />
              <span class="unit">0 表示关闭</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="同题提交间隔">
              <el-input-number v-model="form.submit_interval_sec" :min="0" :max="3600" />
              <span class="unit">秒，0 表示关闭</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="头像大小上限">
              <el-input-number v-model="form.max_avatar_mb" :min="1" :max="100" />
              <span class="unit">MB</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="测试数据上限">
              <el-input-number v-model="form.max_testdata_mb" :min="1" :max="10240" />
              <span class="unit">MB / 文件</span>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="允许跨域来源">
              <el-input
                v-model="corsText"
                type="textarea"
                :rows="3"
                placeholder="每行一个完整来源，例如 https://oj.example.com；* 表示全部来源"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="panel">
      <h3 class="panel__title">评测与节点同步</h3>
      <el-form :model="form" label-width="150px">
        <el-row :gutter="24">
          <el-col :xs="24" :md="12">
            <el-form-item label="单题测试点上限">
              <el-input-number v-model="form.max_testcases" :min="1" :max="10000" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="节点同步超时">
              <el-input-number v-model="form.judge_sync_timeout_sec" :min="1" :max="3600" />
              <span class="unit">秒</span>
              <secret-tag label="同步密钥" :configured="secretStatus.judge_sync_secret_configured" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="评测节点地址">
              <el-input
                v-model="judgeNodesText"
                type="textarea"
                :rows="4"
                placeholder="每行一个节点，例如 http://192.168.1.10:9000；留空表示不推送"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="settings-actions">
      <el-button type="primary" :loading="saving" @click="save">保存并立即生效</el-button>
      <el-button :disabled="saving" @click="load">放弃修改</el-button>
    </div>
  </div>
</template>

<script setup>
import { defineComponent, h, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
import { adminGetSettings, adminUpdateSettings } from '@/api'
import { useUserStore } from '@/store/user'

const SecretTag = defineComponent({
  props: { label: String, configured: Boolean },
  setup(props) {
    return () => h(
      ElTag,
      { type: props.configured ? 'success' : 'warning', size: 'small', class: 'secret-tag' },
      () => `${props.label}${props.configured ? '已配置' : '未配置'}`
    )
  }
})

const defaults = {
  app_name: 'AKIOI', theme_color: '#1CD5C7', jwt_ttl_hours: 168, register_enabled: true,
  comments_enabled: true,
  captcha_enabled: false,
  wechat_enabled: false, wechat_redirect_uri: '', sms_enabled: false,
  sms_api_base_url: 'https://api.starloft.cn/v2', sms_register_template_id: '',
  sms_reset_template_id: '', sms_code_ttl_minutes: 5, sms_send_interval_sec: 60,
  sms_request_timeout_sec: 10, rate_limit_per_minute: 300, submit_interval_sec: 10,
  max_testcases: 100, judge_node_endpoints: [], judge_sync_timeout_sec: 120,
  max_avatar_mb: 2, max_testdata_mb: 512, cors_allow_origins: []
}

const store = useUserStore()
const loading = ref(false)
const saving = ref(false)
const form = reactive({ ...defaults })
const secretStatus = reactive({})
// 常用主题色预设，供快捷选择
const predefineColors = ['#1CD5C7', '#409EFF', '#5470C6', '#00A870', '#F7BA1E', '#F54A45', '#7B61FF', '#34495E']
const judgeNodesText = ref('')
const corsText = ref('')
// 密钥输入框只承载本次提交的新值，已保存的内容不会回显。
const secrets = reactive({
  wechat_app_id: '', wechat_app_secret: '', wechat_open_app_id: '', wechat_open_app_secret: '',
  sms_api_key: '', sms_api_secret: '', smtp_host: '', smtp_port: 465, smtp_username: '',
  smtp_password: '', smtp_from: '', captcha_api_key: '', captcha_api_secret: '', judge_sync_secret: ''
})
const clearGroups = reactive({ wechat: false, wechat_open: false, sms: false, smtp: false, captcha: false, judge: false })
const regenerateJWT = ref(false)

const lines = (value) => value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)

// clearSecretInputs 清空密钥输入框与清除勾选，避免已提交的密钥在表单里残留。
const clearSecretInputs = () => {
  Object.assign(secrets, {
    wechat_app_secret: '', wechat_open_app_secret: '', sms_api_key: '', sms_api_secret: '',
    smtp_password: '', captcha_api_key: '', captcha_api_secret: '', judge_sync_secret: ''
  })
  Object.keys(clearGroups).forEach((key) => { clearGroups[key] = false })
  regenerateJWT.value = false
}

// confirmRegenerateJWT 先确认再标记，保存时由后端重新生成登录密钥。
const confirmRegenerateJWT = async () => {
  try {
    await ElMessageBox.confirm('重新生成后所有用户都需要重新登录，确定继续？', '重新生成登录密钥', {
      type: 'warning',
      confirmButtonText: '重新生成',
      cancelButtonText: '取消'
    })
    regenerateJWT.value = true
    ElMessage.info('已标记，点击「保存并立即生效」后生效')
  } catch (e) {
    // 取消时不改变任何状态
  }
}

const applyData = (data) => {
  Object.assign(form, defaults, data.settings || {})
  Object.assign(secretStatus, data.secret_status || {})
  const view = data.secret_view || {}
  Object.assign(secrets, {
    wechat_app_id: view.wechat_app_id || '',
    wechat_open_app_id: view.wechat_open_app_id || '',
    smtp_host: view.smtp_host || '',
    smtp_port: view.smtp_port || 465,
    smtp_username: view.smtp_username || '',
    smtp_from: view.smtp_from || ''
  })
  clearSecretInputs()
  judgeNodesText.value = (form.judge_node_endpoints || []).join('\n')
  corsText.value = (form.cors_allow_origins || []).join('\n')
}

const load = async () => {
  loading.value = true
  try {
    applyData(await adminGetSettings())
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    const payload = {
      ...form,
      judge_node_endpoints: lines(judgeNodesText.value),
      cors_allow_origins: lines(corsText.value),
      secrets: {
        ...secrets,
        clear: Object.keys(clearGroups).filter((key) => clearGroups[key]),
        regenerate_jwt: regenerateJWT.value
      }
    }
    const data = await adminUpdateSettings(payload)
    applyData(data)
    await store.loadSite()
    if (data.notice) {
      ElMessage.warning(data.notice)
    } else {
      ElMessage.success('系统设置已保存并立即生效')
    }
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.settings-alert { margin-bottom: 16px; }
.settings-inline-alert { margin-bottom: 12px; }
.settings-inline-button { margin-left: 10px; }
.clear-box { margin-top: 6px; }
.unit { margin-left: 8px; color: var(--akioi-text-weak); }
.secret-tag { margin-left: 10px; }
.settings-note { margin: -4px 0 0 150px; }
.settings-actions {
  position: sticky;
  bottom: 0;
  z-index: 2;
  margin-top: 16px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid var(--akioi-border);
  border-radius: var(--akioi-radius);
}
</style>
