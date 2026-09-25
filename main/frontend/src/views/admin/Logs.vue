<template>
  <div class="panel">
    <div class="toolbar">
      <el-select v-model="type" style="width: 140px" @change="load">
        <el-option v-for="item in LOG_TYPES" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="lines" style="width: 110px" @change="load">
        <el-option v-for="item in LINE_OPTIONS" :key="item" :label="`${item} 行`" :value="item" />
      </el-select>
      <el-button @click="load">刷新</el-button>
      <el-switch v-model="autoRefresh" active-text="自动刷新（5 秒）" />
    </div>

    <div v-loading="loading" class="log-view">
      <pre v-if="content" class="pre-block">{{ content }}</pre>
      <p v-else class="muted">暂无日志</p>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { adminGetLogs } from '@/api'

const LOG_TYPES = [
  { value: 'business', label: '业务日志' },
  { value: 'access', label: '访问日志' },
  { value: 'admin', label: '管理日志' },
  { value: 'judge', label: '评测日志' },
  { value: 'error', label: '错误日志' }
]

const LINE_OPTIONS = [100, 200, 500, 1000]

const loading = ref(false)
const type = ref('business')
const lines = ref(200)
const logLines = ref([])
const autoRefresh = ref(false)

const content = computed(() => logLines.value.join('\n'))

let timer = null

const stopTimer = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

const load = async () => {
  loading.value = true
  try {
    const data = await adminGetLogs({ type: type.value, lines: lines.value })
    logLines.value = data.lines || []
  } catch (e) {
    logLines.value = []
  } finally {
    loading.value = false
  }
}

watch(autoRefresh, (enabled) => {
  stopTimer()
  if (enabled) {
    timer = setInterval(load, 5000)
  }
})

onUnmounted(stopTimer)

onMounted(load)
</script>

<style scoped>
.log-view {
  min-height: 240px;
  max-height: 70vh;
  overflow: auto;
}
</style>
