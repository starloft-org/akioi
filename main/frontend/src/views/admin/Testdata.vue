<template>
  <div class="panel">
    <div class="testdata-head">
      <h3 class="panel__title">测试数据</h3>
      <el-button size="small" @click="router.push('/admin/problems')">返回题目列表</el-button>
    </div>
    <p class="muted">题目 ID：{{ problemId }}　测试点数量：{{ items.length }}　总分：{{ totalScore }}</p>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="上传会整体替换该题现有的全部测试数据"
      style="margin-bottom: 12px"
    />

    <div class="toolbar">
      <el-upload
        v-model:file-list="fileList"
        action="#"
        multiple
        :auto-upload="false"
        accept=".in,.out,.ans,.zip"
      >
        <el-button>选择文件</el-button>
      </el-upload>
      <el-input
        v-model="scoresInput"
        placeholder="各测试点分值 JSON 数组，留空为均分，例如 [10,10,80]"
        style="width: 340px"
      />
      <el-button type="primary" :loading="uploading" :disabled="!fileList.length" @click="handleUpload">
        上传并替换
      </el-button>
    </div>

    <el-input
      v-model="subtasksInput"
      type="textarea"
      :rows="3"
      placeholder='可选：子任务与按文件名分值（等价于 subtasks.json），例如 {"scores":{"sample":20},"subtasks":[{"name":"小数据","score":30,"cases":["sample"]}]}'
      style="margin-bottom: 8px"
    />
    <p v-if="subtasks.length" class="muted">子任务：{{ subtaskSummary }}</p>

    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="index" label="测试点" width="90" />
      <el-table-column prop="name" label="文件名" min-width="140" />
      <el-table-column label="分组" width="100">
        <template #default="{ row }">{{ row.group_id ? `子任务 ${row.group_id}` : '独立' }}</template>
      </el-table-column>
      <el-table-column prop="score" label="分值" width="80" />
      <el-table-column label="输入大小" width="110">
        <template #default="{ row }">{{ formatFileSize(row.input_size) }}</template>
      </el-table-column>
      <el-table-column label="输出大小" width="110">
        <template #default="{ row }">{{ formatFileSize(row.output_size) }}</template>
      </el-table-column>
      <el-table-column label="输入 SHA256" min-width="160">
        <template #default="{ row }">
          <span class="mono" :title="row.input_hash">{{ shortHash(row.input_hash) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="输出 SHA256" min-width="160">
        <template #default="{ row }">
          <span class="mono" :title="row.output_hash">{{ shortHash(row.output_hash) }}</span>
        </template>
      </el-table-column>
    </el-table>

    <div class="toolbar" style="margin-top: 16px">
      <el-button type="primary" plain :loading="syncing" :disabled="!items.length" @click="handleSync">
        推送到评测节点
      </el-button>
      <el-button @click="downloadPackage">下载数据包</el-button>
      <el-button :disabled="!items.length" @click="clearTestdata">清空测试数据</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  adminDeleteTestdata,
  adminDownloadTestdata,
  adminListTestdata,
  adminSyncTestdata,
  adminUploadTestdata,
  saveBlob
} from '@/api'
import { formatFileSize } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const problemId = computed(() => Number(route.params.id))

const loading = ref(false)
const uploading = ref(false)
const syncing = ref(false)
const items = ref([])
const totalScore = ref(0)
const fileList = ref([])
const scoresInput = ref('')
const subtasks = ref([])
const subtasksInput = ref('')
const subtaskSummary = computed(() =>
  subtasks.value
    .map((st) => `${st.name}（${st.score} 分，${(st.cases || []).length} 个测试点）`)
    .join('、')
)

const shortHash = (hash) => (hash ? `${hash.slice(0, 12)}…` : '-')

// warnSyncFailed 提示未同步成功的评测节点，返回是否已给出提示。
const warnSyncFailed = (payload) => {
  const failed = payload?.sync_failed || []
  if (!failed.length) {
    return false
  }
  ElMessage.warning(`测试数据已保存，但以下评测节点同步失败：${failed.join('、')}`)
  return true
}

const load = async () => {
  loading.value = true
  try {
    const data = await adminListTestdata(problemId.value)
    items.value = data.items || []
    totalScore.value = data.total_score ?? 0
    subtasks.value = data.subtasks || []
    if (!subtasksInput.value && subtasks.value.length) {
      const nameOf = (idx) => items.value.find((it) => it.index === idx)?.name || String(idx)
      subtasksInput.value = JSON.stringify(
        {
          subtasks: subtasks.value.map((st) => ({
            name: st.name,
            score: st.score,
            cases: (st.cases || []).map(nameOf)
          }))
        },
        null,
        2
      )
    }
  } catch (e) {
    items.value = []
    subtasks.value = []
    totalScore.value = 0
  } finally {
    loading.value = false
  }
}

const handleUpload = async () => {
  const files = fileList.value.map((item) => item.raw).filter(Boolean)
  if (!files.length) {
    ElMessage.warning('请先选择文件')
    return
  }
  let scores = null
  const text = scoresInput.value.trim()
  if (text) {
    let parsed
    try {
      parsed = JSON.parse(text)
    } catch (e) {
      ElMessage.error('分值需为 JSON 数字数组，例如 [10,10,80]')
      return
    }
    if (!Array.isArray(parsed) || parsed.some((num) => typeof num !== 'number')) {
      ElMessage.error('分值需为 JSON 数字数组，例如 [10,10,80]')
      return
    }
    scores = parsed
  }
  let subtasksText = null
  const specText = subtasksInput.value.trim()
  if (specText) {
    try {
      JSON.parse(specText)
    } catch (e) {
      ElMessage.error('子任务与分值需为 JSON，例如 {"subtasks":[{"name":"小数据","score":30,"cases":["sample"]}]}')
      return
    }
    subtasksText = specText
  }
  uploading.value = true
  try {
    const res = await adminUploadTestdata(problemId.value, files, scores, subtasksText)
    if (!warnSyncFailed(res)) {
      ElMessage.success('测试数据已替换')
    }
    fileList.value = []
    scoresInput.value = ''
    subtasksInput.value = ''
    load()
  } catch (e) {
    // 错误提示已由请求层处理
  } finally {
    uploading.value = false
  }
}

const downloadPackage = async () => {
  try {
    const blob = await adminDownloadTestdata(problemId.value)
    saveBlob(blob, `problem-${problemId.value}-testdata.zip`)
  } catch (e) {
    // 错误提示已由请求层处理
  }
}

// handleSync 主动把测试数据推送到全部评测节点，用于节点上线后的补推送。
const handleSync = async () => {
  syncing.value = true
  try {
    const data = await adminSyncTestdata(problemId.value)
    const failed = data?.sync_failed || []
    if (failed.length) {
      ElMessage.warning(`以下评测节点同步失败：${failed.join('、')}`)
    } else {
      ElMessage.success('已推送到全部评测节点')
    }
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    syncing.value = false
  }
}

const clearTestdata = async () => {
  try {
    await ElMessageBox.confirm('确认清空该题的全部测试数据？', '清空确认', { type: 'warning' })
  } catch (e) {
    return
  }
  let res
  try {
    res = await adminDeleteTestdata(problemId.value)
  } catch (e) {
    return
  }
  if (!warnSyncFailed(res)) {
    ElMessage.success('已清空')
  }
  load()
}

onMounted(load)
</script>

<style scoped>
.testdata-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.testdata-head .panel__title {
  margin: 0;
}
</style>
