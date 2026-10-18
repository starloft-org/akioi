<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="340px"
    :close-on-click-modal="false"
    @closed="cleanup"
  >
    <div class="qr-body">
      <el-skeleton v-if="loading" :rows="4" animated />
      <template v-else-if="error">
        <el-empty :description="error" :image-size="72" />
        <el-button size="small" @click="start">重新生成</el-button>
      </template>
      <template v-else>
        <canvas ref="canvasRef" class="qr-canvas"></canvas>
        <p class="qr-tip">{{ tip }}</p>
        <p class="qr-hint">{{ hint }}</p>
      </template>
    </div>
    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
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

const title = computed(() => (props.purpose === 'bind' ? '绑定微信' : '微信登录'))
const tip = computed(() => (props.purpose === 'bind' ? '请用微信扫码完成绑定' : '请用微信扫码登录'))
const hint = '二维码 5 分钟内有效，扫码后在微信里确认授权'

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
      redirect: props.purpose === 'bind' ? '/profile' : '/'
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
    await QRCode.toCanvas(canvasRef.value, data.url, { width: 240, margin: 1 })
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
.qr-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-height: 200px;
}

.qr-canvas {
  width: 240px;
  height: 240px;
}

.qr-tip {
  margin: 4px 0 0;
  font-size: 13px;
  font-weight: 600;
}

.qr-hint {
  margin: 0;
  font-size: 12px;
  color: var(--akioi-text-weak);
}
</style>
