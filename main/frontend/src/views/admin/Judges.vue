<template>
  <div class="panel">
    <div class="panel-head">
      <h3 class="panel__title">测评机管理</h3>
      <div>
        <el-button :loading="checking" @click="load">检查状态</el-button>
        <el-button type="primary" :loading="syncing" :disabled="!nodes.length" @click="handleSyncAll">
          同步全部题目数据
        </el-button>
      </div>
    </div>

    <el-alert
      v-if="!secretConfigured"
      type="warning"
      :closable="false"
      show-icon
      title="尚未配置 JUDGE_SYNC_SECRET，节点会因为密钥校验失败而全部显示离线"
      style="margin-bottom: 12px"
    />
    <el-alert
      v-if="nodes.length === 0"
      type="info"
      :closable="false"
      show-icon
      title="系统设置里还没有配置评测节点地址"
      style="margin-bottom: 12px"
    />

    <p class="muted">主站共有 {{ totalProblems }} 道题带测试数据</p>

    <el-table v-loading="checking" :data="nodes" size="small">
      <el-table-column label="节点地址" min-width="240">
        <template #default="{ row }">{{ row.endpoint }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.reachable ? 'success' : 'danger'" size="small">
            {{ row.reachable ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="延迟" width="90">
        <template #default="{ row }">
          {{ row.reachable ? row.latency_ms + ' ms' : '—' }}
        </template>
      </el-table-column>
      <el-table-column label="队列长度" width="90" prop="queue_length" />
      <el-table-column label="待评测任务" width="110" prop="pending_jobs" />
      <el-table-column label="数据题目数" width="120">
        <template #default="{ row }">
          <span :class="{ 'diff-warn': row.reachable && row.data_problems !== totalProblems }">
            {{ row.reachable ? row.data_problems + ' / ' + totalProblems : '—' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="异常信息" min-width="220">
        <template #default="{ row }">
          <span class="muted">{{ row.error || '—' }}</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminJudgeNodesStatus, adminJudgeSyncAll } from '@/api'

const checking = ref(false)
const syncing = ref(false)
const nodes = ref([])
const totalProblems = ref(0)
const secretConfigured = ref(false)

const load = async () => {
  checking.value = true
  try {
    const data = await adminJudgeNodesStatus()
    nodes.value = data.nodes || []
    totalProblems.value = data.total_problems || 0
    secretConfigured.value = !!data.secret_configured
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    checking.value = false
  }
}

const handleSyncAll = async () => {
  try {
    await ElMessageBox.confirm(
      `将把全部 ${totalProblems.value} 道题目的测试数据推送到所有评测节点，题目较多时可能需要几分钟，确定继续？`,
      '批量同步',
      { type: 'warning', confirmButtonText: '开始同步', cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }
  syncing.value = true
  try {
    const data = await adminJudgeSyncAll()
    const failedCount = data?.failed?.length || 0
    if (failedCount > 0) {
      ElMessage.warning(
        `同步完成：${data.success}/${data.total} 成功，${failedCount} 道失败，详见下方节点状态`
      )
    } else {
      ElMessage.success(`已同步全部 ${data?.total || 0} 道题目`)
    }
    load()
  } catch (e) {
    // 错误提示由请求层统一处理
  } finally {
    syncing.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.panel-head .panel__title {
  margin: 0;
}

.diff-warn {
  color: #f54a45;
  font-weight: 600;
}
</style>
