<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="420px"
    align-center
    :close-on-click-modal="false"
    @closed="cleanup"
  >
    <div class="qr-wrap">
      <div class="qr-box">
        <el-skeleton v-if="loading" :rows="2" animated class="qr-skeleton" />
        <div v-else-if="error" class="qr-expired">
          <p>{{ error }}</p>
          <el-button size="small" type="primary" @click="start">刷新二维码</el-button>
        </div>
        <canvas v-else ref="canvasRef" class="qr-canvas"></canvas>
      </div>
      <p class="qr-tip">
        <template v-if="purpose === 'bind'">
          请用微信「扫一扫」扫描二维码，扫码授权后即完成绑定。
        </template>
        <template v-else>
          请用微信「扫一扫」扫描二维码，在手机上确认授权后，本页面将自动登录。
        </template>
      </p>
      <p v-if="!loading && !error" class="qr-status">等待扫码…</p>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import QRCode from 'qrcode'
import { getWechatStatus, getWechatUrl } from '@/api'

const props = defineProps({
  purpose: { type: String, default: 'login' }
})

const emit = defineEmits(['login', 'bound'])

const visible = ref(false)
const loading = ref(false)
const error = ref('')
const state = ref('')
const canvasRef = ref(null)

let timer = null
let deadline = 0

const title = computed(() => (props.purpose === 'bind' ? '微信扫码绑定' : '微信扫码登录'))

const stopTimer = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

const cleanup = () => {
  stopTimer()
  state.value = ''
  error.value = ''
  loading.value = false
}

const poll = async () => {
  if (Date.now() > deadline) {
    stopTimer()
    error.value = '二维码已过期，请重新生成'
    return
  }
  try {
    const data = await getWechatStatus(state.value)
    if (data?.status === 'authorized') {
      stopTimer()
      visible.value = false
      emit('login', data)
      return
    }
    if (data?.status === 'bound') {
      stopTimer()
      visible.value = false
      emit('bound', data)
    }
  } catch (e) {
    // 单次轮询失败不中断，网络抖动后继续
  }
}

const start = async () => {
  cleanup()
  loading.value = true
  try {
    const data = await getWechatUrl({
      purpose: props.purpose,
      redirect: props.purpose === 'bind' ? '/profile' : '/',
      from: 'pc'
    })
    if (!data?.enabled || !data.url) {
      error.value = '微信登录未启用'
      return
    }
    state.value = data.state
    loading.value = false
    // 等 canvas 渲染出来再画二维码；对话框有过渡动画，偶尔需要多等一帧
    await nextTick()
    if (!canvasRef.value) {
      await new Promise((resolve) => setTimeout(resolve, 120))
    }
    await QRCode.toCanvas(canvasRef.value, data.url, { width: 196, margin: 1 })
    deadline = Date.now() + 5 * 60 * 1000
    stopTimer()
    timer = setInterval(poll, 2000)
  } catch (e) {
    error.value = '二维码生成失败'
  } finally {
    loading.value = false
  }
}

const open = () => {
  visible.value = true
  start()
}

defineExpose({ open })

onBeforeUnmount(stopTimer)
</script>

<style scoped>
.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* 二维码外框：与 StarLoft 一致，二维码居中留白 */
.qr-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 220px;
  height: 220px;
  border: 1px solid var(--akioi-border);
  border-radius: 8px;
  background: #fff;
}

.qr-canvas {
  width: 196px;
  height: 196px;
}

.qr-skeleton {
  width: 100%;
  padding: 20px;
  box-sizing: border-box;
}

.qr-expired {
  text-align: center;
  font-size: 13px;
  color: var(--akioi-text-weak);
}

.qr-expired p {
  margin: 0 0 10px;
}

.qr-tip {
  margin: 14px 0 0;
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
  color: var(--akioi-text-weak);
}

.qr-status {
  margin-top: 6px;
  font-size: 12px;
  color: var(--akioi-text-weak);
}
</style>
