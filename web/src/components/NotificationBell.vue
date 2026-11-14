<script setup lang="ts">
// Notification bell: unread badge (30s poll via the store), dropdown list,
// mark-read and deep links to the ticket. Agents/admins land in the
// console; requesters in their portal list (issue #16/#18).
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { listNotifications, markNotificationsRead, type NotificationItem } from '../api'
import { notificationsStore } from '../stores/notifications'
import { meStore } from '../stores/me'

const router = useRouter()
const open = ref(false)
const items = ref<NotificationItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const page = await listNotifications(false, 1, 20)
    items.value = page.data ?? []
  } finally {
    loading.value = false
  }
}

async function toggle() {
  open.value = !open.value
  if (open.value) {
    await load()
  }
}

async function markAll() {
  await markNotificationsRead()
  await load()
  await notificationsStore.refresh()
}

async function openItem(item: NotificationItem) {
  if (!item.read_at) {
    await markNotificationsRead([item.id])
    await notificationsStore.refresh()
  }
  open.value = false
  if (item.ticket_id) {
    if (meStore.role === 'requester') {
      router.push(`/mine/${item.ticket_id}`)
    } else {
      router.push(`/console/ticket/${item.ticket_id}`)
    }
  }
}

function timeOf(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => void notificationsStore.refresh())
</script>

<template>
  <el-popover :visible="open" placement="bottom-end" :width="380" @hide="open = false">
    <template #reference>
      <el-badge
        :value="notificationsStore.unreadCount"
        :hidden="notificationsStore.unreadCount === 0"
        :max="99"
        class="bell"
      >
        <el-icon :size="18" class="bell__icon" @click="toggle"><i class="bell-i" /></el-icon>
      </el-badge>
    </template>
    <div class="panel">
      <div class="panel__head">
        <span>通知</span>
        <el-button v-if="notificationsStore.unreadCount > 0" link size="small" @click="markAll">
          全部标记已读
        </el-button>
      </div>
      <div v-if="loading" class="panel__empty">加载中</div>
      <div v-else-if="items.length === 0" class="panel__empty">暂无通知</div>
      <div
        v-for="n in items"
        :key="n.id"
        class="panel__item"
        :class="{ 'is-unread': !n.read_at }"
        @click="openItem(n)"
      >
        <div class="panel__title">{{ n.title }}</div>
        <div v-if="n.body" class="panel__body">{{ n.body }}</div>
        <div class="panel__time">{{ timeOf(n.created_at) }}</div>
      </div>
    </div>
  </el-popover>
</template>

<style scoped>
.bell {
  cursor: pointer;
  display: inline-flex;
  align-items: center;
}

.bell__icon {
  color: var(--color-text-secondary);
}

.bell-i {
  display: inline-block;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 1.5px solid currentColor;
  position: relative;
}

.bell-i::before {
  content: '';
  position: absolute;
  top: -4px;
  left: 50%;
  width: 5px;
  height: 3px;
  background: currentColor;
  border-radius: 2px 2px 0 0;
  transform: translateX(-50%);
}

.panel__head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  padding-bottom: var(--space-2);
}

.panel__empty {
  color: var(--color-text-dim);
  text-align: center;
  padding: var(--space-6) 0;
}

.panel__item {
  padding: var(--space-2) var(--space-1);
  border-top: 1px solid var(--color-border-subtle);
  cursor: pointer;

  &.is-unread .panel__title {
    font-weight: 600;
  }

  &:hover {
    background: var(--color-bg-subtle);
  }
}

.panel__title {
  font-size: 13px;
  color: var(--color-text-primary);
}

.panel__body {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-top: 2px;
}

.panel__time {
  font-size: 12px;
  color: var(--color-text-dim);
  margin-top: 2px;
}
</style>
