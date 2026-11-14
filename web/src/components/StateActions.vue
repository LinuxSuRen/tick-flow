<script setup lang="ts">
// State actions render one button per transition the backend precomputed
// for the current viewer; unauthorized moves never arrive in the first
// place (issue #17/#18). Copy is owned by the frontend.
import type { TransitionOption } from '../api'

defineProps<{ transitions: TransitionOption[]; loading?: boolean }>()
const emit = defineEmits<{ (e: 'action', action: string): void }>()

const actionCopy: Record<string, string> = {
  accept: '受理',
  resolve: '标记解决',
  close: '确认关闭',
  reopen: '重新打开',
  confirm: '确认完成',
  reject: '驳回',
  start: '开始处理',
  submit: '提交验收',
  approve: '同意',
  finish: '完成',
  assign: '安排上门',
}

// Destructive or final moves get the plain style; everything else is
// primary.
const plainActions = new Set(['reopen', 'reject'])
</script>

<template>
  <div class="state-actions">
    <el-button
      v-for="tr in transitions"
      :key="tr.action"
      :type="plainActions.has(tr.action) ? 'default' : 'primary'"
      :loading="loading"
      @click="emit('action', tr.action)"
    >
      {{ actionCopy[tr.action] ?? tr.action }}
    </el-button>
  </div>
</template>

<style scoped>
.state-actions {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
}
</style>
