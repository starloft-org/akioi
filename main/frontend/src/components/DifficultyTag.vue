<template>
  <span v-if="difficulty" class="difficulty-tag" :style="tagStyle">{{ difficulty }}</span>
</template>

<script setup>
import { computed } from 'vue'
import { difficultyColor } from '@/utils/format'

const props = defineProps({
  difficulty: { type: String, default: '' }
})

const tagStyle = computed(() => {
  const color = difficultyColor(props.difficulty)
  return {
    color,
    background: `${color}1a`,
    borderColor: `${color}55`
  }
})
</script>

<style scoped>
.difficulty-tag {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
  border: 1px solid transparent;
  border-radius: 4px;
}
</style>
