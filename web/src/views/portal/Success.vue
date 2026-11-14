<script setup lang="ts">
// Submission success page: copy from the pack view_config plus the ticket
// number and an entry into "my tickets" (issue #17).
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fetchTicket, fetchTicketType, type TicketDetail, type TicketTypeView } from '../../api'

const route = useRoute()
const router = useRouter()

const ticket = ref<TicketDetail | null>(null)
const type = ref<TicketTypeView | null>(null)

onMounted(async () => {
  ticket.value = await fetchTicket(Number(route.params.ticketId))
  type.value = await fetchTicketType(ticket.value.ticket_type.id)
})
</script>

<template>
  <div v-if="ticket" class="success">
    <div class="success__card">
      <div class="success__icon">✅</div>
      <h2 class="success__title">{{ type?.view_config?.portal?.success_text || '已提交，请留意通知' }}</h2>
      <p class="success__number">工单号 <span class="success__num">#{{ ticket.number }}</span></p>
      <div class="success__actions">
        <el-button type="primary" @click="router.push(`/mine/${ticket.id}`)">查看工单</el-button>
        <el-button @click="router.push('/mine')">查看我的工单</el-button>
        <el-button @click="router.push('/')">再提一单</el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.success {
  display: flex;
  justify-content: center;
  padding: var(--space-12) 0;
}

.success__card {
  text-align: center;
  background: var(--color-bg-panel);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-10) var(--space-12);
}

.success__icon {
  font-size: 40px;
}

.success__title {
  margin: var(--space-3) 0 var(--space-1);
}

.success__number {
  color: var(--color-text-secondary);
}

.success__num {
  font-family: var(--font-mono);
  color: var(--color-accent-base);
  font-weight: 600;
}

.success__actions {
  margin-top: var(--space-6);
  display: flex;
  gap: var(--space-2);
  justify-content: center;
}
</style>
