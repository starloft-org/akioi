<template>
  <div>
    <div v-loading="loading" class="panel">
      <h3 class="panel__title">数据概览</h3>
      <div class="stat-grid">
        <div v-for="card in cards" :key="card.label" class="stat-card">
          <div class="stat-card__label">{{ card.label }}</div>
          <div class="stat-card__value">{{ card.value }}</div>
        </div>
      </div>
    </div>

    <div class="panel">
      <h3 class="panel__title">快捷入口</h3>
      <el-button type="primary" @click="router.push('/admin/problems/new')">新建题目</el-button>
      <el-button @click="router.push('/admin/contests/new')">新建比赛</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminGetStats } from '@/api'

const router = useRouter()
const loading = ref(false)
const stats = ref(null)

// 通过率小于等于 1 时按比例换算为百分数
const rateText = (rate) => {
  const value = Number(rate) || 0
  return `${(value <= 1 ? value * 100 : value).toFixed(2)}%`
}

const cards = computed(() => {
  const s = stats.value || {}
  return [
    { label: '用户数', value: s.user_count ?? 0 },
    { label: '题目数', value: s.problem_count ?? 0 },
    { label: '提交数', value: s.submission_count ?? 0 },
    { label: '通过数', value: s.accepted_count ?? 0 },
    { label: '通过率', value: rateText(s.acceptance_rate) },
    { label: '今日提交', value: s.today_submission_count ?? 0 },
    { label: '评测队列长度', value: s.queue_length ?? 0 }
  ]
})

const load = async () => {
  loading.value = true
  try {
    stats.value = await adminGetStats()
  } catch (e) {
    stats.value = null
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
