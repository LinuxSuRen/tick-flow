<script setup lang="ts">
// My tickets: the requester's own list (the backend enforces the scope)
// with type/state filters; filters reset the page to 1 (issue #17).
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { listTicketTypes, listTickets, type TicketListItem, type TicketTypeView } from '../../api'

const router = useRouter()
const items = ref<TicketListItem[]>([])
const types = ref<TicketTypeView[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({ type: undefined as number | undefined, state: '' })
const page = reactive({ page: 1, page_size: 20 })

const stateOptions = ref<{ key: string; name: string }[]>([])

async function load() {
  loading.value = true
  try {
    const result = await listTickets({
      page: page.page,
      page_size: page.page_size,
      type: filter.type,
      state: filter.state,
    })
    items.value = result.data ?? []
    total.value = result.total
  } finally {
    loading.value = false
  }
}

function onFilterChange() {
  page.page = 1
  syncStateOptions()
  void load()
}

function syncStateOptions() {
  const t = types.value.find((x) => x.id === filter.type)
  stateOptions.value = t ? t.workflow.states.map((s) => ({ key: s.key, name: s.name })) : []
}

onMounted(async () => {
  const page = await listTicketTypes(1, 100)
  types.value = page.data ?? []
  await load()
})
</script>

<template>
  <div>
    <h2 class="page__title">我的工单</h2>
    <div class="block">
      <div class="mine__filters">
        <el-select
          v-model="filter.type"
          placeholder="全部类型"
          clearable
          style="width: 200px"
          @change="onFilterChange"
        >
          <el-option v-for="t in types" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
        <el-select
          v-model="filter.state"
          placeholder="全部状态"
          clearable
          style="width: 160px"
          @change="onFilterChange"
        >
          <el-option v-for="s in stateOptions" :key="s.key" :label="s.name" :value="s.key" />
        </el-select>
      </div>
      <el-table v-loading="loading" :data="items" @row-click="(row: TicketListItem) => router.push(`/mine/${row.id}`)">
        <el-table-column label="工单号" width="100">
          <template #default="{ row }">
            <span class="mine__number">#{{ row.number }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
        <el-table-column prop="ticket_type.name" label="类型" width="130" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.closed_at ? 'info' : 'success'" effect="plain">{{ row.state_name }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            {{ new Date(row.created_at).toLocaleString('zh-CN', { hour12: false }) }}
          </template>
        </el-table-column>
        <el-table-column width="90">
          <template #default>
            <el-button link type="primary">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && items.length === 0" description="还没有工单，去门户提交第一单" />
      <div class="mine__pager">
        <el-pagination
          v-model:current-page="page.page"
          :page-size="page.page_size"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="load"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.mine__filters {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-3);
}

.mine__number {
  font-family: var(--font-mono);
  color: var(--color-accent-base);
  cursor: pointer;
}

.mine__pager {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-3);
}

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
