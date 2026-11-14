<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { meStore } from '../stores/me'
import { tenantStore } from '../stores/tenant'
import { clearToken } from '../api'
import { notificationsStore } from '../stores/notifications'
import NotificationBell from './NotificationBell.vue'
import { CaretBottom } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()

// Console and admin entries are role gated (permission matrix, issue #6).
const showConsole = computed(() => ['agent', 'tenant_admin'].includes(meStore.role))
const showAdmin = computed(() => meStore.role === 'tenant_admin')

const navs = computed(() => {
  const items = [
    { name: 'portal', label: '门户', to: '/' },
    { name: 'portal-mine', label: '我的工单', to: '/mine' },
  ]
  if (showConsole.value) {
    items.push({ name: 'console', label: '工作台', to: '/console' })
  }
  if (showAdmin.value) {
    items.push({ name: 'admin', label: '管理', to: '/admin/members' })
  }
  return items
})

const area = computed(() => {
  const matched = route.matched.map((m) => String(m.name ?? ''))
  if (matched.includes('portal-mine') || matched.includes('portal-ticket')) return 'portal-mine'
  return String(route.matched[0]?.name ?? '')
})

function logout() {
  clearToken()
  meStore.reset()
  notificationsStore.stopPolling()
  router.replace('/login')
}
</script>

<template>
  <header class="app-header">
    <div class="app-header__inner">
      <router-link class="app-header__brand" to="/">
        <img
          v-if="tenantStore.logoUrl"
          :src="tenantStore.logoUrl"
          alt=""
          class="app-header__logo-img"
        />
        <span v-else class="app-header__logo">TF</span>
        <span class="app-header__name">{{ tenantStore.name }}</span>
      </router-link>
      <nav class="app-header__nav">
        <router-link
          v-for="item in navs"
          :key="item.name"
          class="app-header__link"
          :class="{ 'is-active': area === item.name }"
          :to="item.to"
        >
          {{ item.label }}
        </router-link>
      </nav>
      <div class="app-header__right">
        <NotificationBell />
        <el-dropdown v-if="meStore.user" @command="logout">
          <span class="app-header__user">
            {{ meStore.user.display_name }}
            <el-icon :size="12"><CaretBottom /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  background: var(--color-bg-panel);
  border-bottom: 1px solid var(--color-border-subtle);
  position: sticky;
  top: 0;
  z-index: 10;
}

.app-header__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 var(--space-4);
  height: 52px;
  display: flex;
  align-items: center;
  gap: var(--space-6);
}

.app-header__brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--color-text-bright);
}

.app-header__logo {
  width: 26px;
  height: 26px;
  border-radius: var(--radius-sm);
  background: var(--color-accent-base);
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.app-header__logo-img {
  width: 26px;
  height: 26px;
  border-radius: var(--radius-sm);
  object-fit: cover;
}

.app-header__name {
  font-weight: 600;
}

.app-header__nav {
  display: flex;
  gap: var(--space-4);
  flex: 1;
}

.app-header__link {
  color: var(--color-text-secondary);
  padding: var(--space-2) var(--space-1);
  border-bottom: 2px solid transparent;

  &.is-active {
    color: var(--color-accent-base);
    border-bottom-color: var(--color-accent-base);
  }
}

.app-header__right {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.app-header__user {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  cursor: pointer;
  color: var(--color-text-primary);
  outline: none;
}
</style>
