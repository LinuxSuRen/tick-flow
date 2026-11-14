<script setup lang="ts">
// Portal home: one card per enabled ticket type (issue #17). All copy
// except the card labels comes from the pack view_config (tenant data).
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { listTicketTypes, type TicketTypeView } from '../../api'

const router = useRouter()
const types = ref<TicketTypeView[]>([])
const loading = ref(true)

const iconCopy: Record<string, string> = {
  laptop_mac: '💻',
  support_agent: '🎧',
  build: '🔧',
  approval: '📝',
}

onMounted(async () => {
  try {
    const page = await listTicketTypes(1, 100)
    types.value = (page.data ?? []).filter((t) => t.status === 'enabled')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <h2 class="page__title">提交工单</h2>
    <div v-if="loading" class="muted">加载中</div>
    <el-empty v-else-if="types.length === 0" description="暂无可用的工单类型" />
    <div v-else class="cards">
      <div
        v-for="t in types"
        :key="t.id"
        class="cards__item"
        @click="router.push(`/new/${t.id}`)"
      >
        <div class="cards__icon">{{ iconCopy[t.icon] ?? '🎫' }}</div>
        <div class="cards__title">{{ t.view_config?.portal?.title || t.name }}</div>
        <div class="cards__desc">{{ t.view_config?.portal?.description }}</div>
      </div>
    </div>
    <div class="block portal-mine-link">
      <router-link to="/mine">查看我的工单</router-link>
    </div>
  </div>
</template>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: var(--space-4);
  margin-top: var(--space-2);
}

.cards__item {
  background: var(--color-bg-panel);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  cursor: pointer;
  transition: border-color 0.15s ease;

  &:hover {
    border-color: var(--color-accent-base);
  }
}

.cards__icon {
  font-size: 28px;
}

.cards__title {
  font-size: 16px;
  font-weight: 600;
  margin-top: var(--space-2);
  color: var(--color-text-bright);
}

.cards__desc {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-top: var(--space-2);
  line-height: 1.5;
}

.portal-mine-link {
  padding: var(--space-3) var(--space-4);
}
</style>
