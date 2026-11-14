<script setup lang="ts">
// The timeline list renders every kind distinctly: conversations as
// bubbles (internal notes visually flagged), state changes / assignments /
// system events as compact system rows. Markdown rendering with a DOMPurify
// allowlist stays inside this component (issue #14).
import { computed } from 'vue'
import { Marked } from 'marked'
import DOMPurify from 'dompurify'
import type { TimelineEntry } from '../api'
import { meStore } from '../stores/me'

const props = defineProps<{ entries: TimelineEntry[] }>()

const marked = new Marked({ gfm: false, breaks: true })

function renderMarkdown(src: string): string {
  if (!src) return ''
  const html = marked.parse(src, { async: false })
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'code', 'pre', 'ul', 'ol', 'li', 'blockquote', 'a'],
    ALLOWED_ATTR: ['href', 'title'],
  })
}

const actionCopy: Record<string, string> = {
  accept: '受理',
  resolve: '标记解决',
  close: '关闭',
  reopen: '重新打开',
  confirm: '确认完成',
  reject: '驳回',
  start: '开始处理',
  submit: '提交验收',
  approve: '同意',
  assign: '安排上门',
  finish: '完成',
}

function metaOf(e: TimelineEntry, key: string): string {
  const v = e.meta?.[key]
  return typeof v === 'string' ? v : ''
}

function systemText(e: TimelineEntry): string {
  switch (e.kind) {
    case 'state_change': {
      const action = actionCopy[metaOf(e, 'action')] ?? metaOf(e, 'action')
      return `${e.author} 执行了「${action}」`
    }
    case 'assignment':
      return `${e.author} 分派了工单`
    case 'system':
      return e.body || `${e.author} 更新了工单`
    default:
      return ''
  }
}

function timeOf(e: TimelineEntry): string {
  return new Date(e.created_at).toLocaleString('zh-CN', { hour12: false })
}

const isMine = (e: TimelineEntry) => meStore.user?.id === e.author_id
</script>

<template>
  <div class="timeline">
    <div
      v-for="e in props.entries"
      :key="e.id"
      class="timeline__item"
      :class="{ 'timeline__item--internal': e.kind === 'internal_note' }"
    >
      <template v-if="e.kind === 'comment' || e.kind === 'internal_note'">
        <div class="timeline__head">
          <span class="timeline__author" :class="{ 'is-me': isMine(e) }">{{ e.author || '用户' }}</span>
          <span v-if="e.kind === 'internal_note'" class="timeline__badge">内部备注</span>
          <span class="timeline__time">{{ timeOf(e) }}</span>
        </div>
        <div class="timeline__body" v-html="renderMarkdown(e.body)" />
      </template>
      <template v-else>
        <div class="timeline__sys">
          <span class="timeline__dot" />
          <span>{{ systemText(e) }}</span>
          <span class="timeline__time">{{ timeOf(e) }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.timeline {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.timeline__item {
  padding: var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-bg-subtle);

  &--internal {
    background: #fdf6ec;
    border: 1px dashed #e6a23c66;
  }
}

.timeline__head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-1);
}

.timeline__author {
  font-weight: 600;
  color: var(--color-text-primary);

  &.is-me {
    color: var(--color-accent-base);
  }
}

.timeline__badge {
  font-size: 12px;
  color: #b88230;
  background: #faecd8;
  border-radius: var(--radius-sm);
  padding: 0 var(--space-1);
}

.timeline__time {
  font-size: 12px;
  color: var(--color-text-dim);
  margin-left: auto;
}

.timeline__body {
  color: var(--color-text-primary);
  line-height: 1.6;
  word-break: break-word;

  :deep(p) {
    margin: 0 0 var(--space-1);
  }

  :deep(a) {
    color: var(--color-accent-base);
  }
}

.timeline__sys {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: 13px;
  color: var(--color-text-secondary);
  padding: var(--space-1) var(--space-2);
}

.timeline__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-border-subtle);
  flex: none;
}
</style>
