<template>
  <div v-loading="loading">
    <el-alert
      title="本页设置保存后立即生效，无需重启服务。数据库、Redis、文件目录及第三方密钥仍从 .env 读取。"
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
          SMTP 服务器、端口、账号与发件人在 .env 中配置（SMTP_HOST / SMTP_PORT / SMTP_USERNAME / SMTP_PASSWORD / SMTP_FROM），
          修改后需重启服务生效。短信与邮件都未开放时，站点不支持注册。
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
import { ElMessage, ElTag } from 'element-plus'
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

const lines = (value) => value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)

const applyData = (data) => {
  Object.assign(form, defaults, data.settings || {})
  Object.assign(secretStatus, data.secret_status || {})
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
      cors_allow_origins: lines(corsText.value)
    }
    const data = await adminUpdateSettings(payload)
    Object.assign(form, data.settings || payload)
    judgeNodesText.value = (form.judge_node_endpoints || []).join('\n')
    corsText.value = (form.cors_allow_origins || []).join('\n')
    await store.loadSite()
    ElMessage.success('系统设置已保存并立即生效')
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
