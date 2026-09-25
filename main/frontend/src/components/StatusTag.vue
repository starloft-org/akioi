<template>
  <el-tag :type="statusType(status)">{{ statusText(status) }}</el-tag>
</template>

<script setup>
import { statusText, statusType } from '@/utils/format'

defineProps({
  status: { type: String, default: '' }
})
</script>
