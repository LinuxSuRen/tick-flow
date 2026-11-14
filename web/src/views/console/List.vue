<script setup lang="ts">
// Console list: dynamic columns from the selected type's view_config plus
// the full filter set (issue #18).
import { onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  listMembers,
  listTicketTypes,
  listTickets,
  type MemberView,
  type TicketListItem,
  type TicketTypeView,
} from '../../api'
import { meStore } from '../../stores/me'

const router = useRouter()
const items = ref<TicketListItem[]>([])
const types = ref<TicketTypeView[]>([])
const members = ref<MemberView[]>([])
const activeType = ref<TicketTypeView | null>(null)
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  type: undefined as number | undefined,
  state: '',
  assignee: undefined as number | undefined,
  keyword: '',
})
const page = reactive({ page: 1, page_size: 20 })

const priorityCopy: Record<string, string> = { low: '低', normal: '普通', high: '高', urgent: '紧急' }
const priorityType: Record<string, 'info' | 'success' | 'warning' | 'danger'> = {
  low: 'info', normal: 'success', high: 'warning', urgent: 'danger',
}

async function load() {
  loading.value = true
  try {
    const result = await listTickets({
      page: page.page,
      page_size: page.page_size,
      type: filter.type,
      state: filter.state,
      assignee: filter.assignee,
      keyword: filter.keyword || undefined,
    })
    items.value = result.data ?? []
    total.value = result.total
  } finally {
    loading.value = false
  }
}

function onFilterChange() {
  page.page = 1
  activeType.value = types.value.find((t) => t.id === filter.type) ?? null
  void load()
}

// Column model: builtin plus custom fields of the active type, following
// view_config.list_columns when a type is selected.
import { computed } from 'vue'

interface Column {
  key: string
  label: string
  builtin: boolean
}

const builtinLabels: Record<string, string> = {
  number: '工单号',
  title: '标题',
  state: '状态',
  priority: '优先级',
  assignee: '处理人',
  creator: '创建人',
  created_at: '创建时间',
  updated_at: '更新时间',
  closed_at: '关闭时间',
  content: '描述',
}

const columns = computed<Column[]>(() => {
  const type = activeType.value
  if (!type) {
    return [
      { key: 'number', label: '工单号', builtin: true },
      { key: 'title', label: '标题', builtin: true },
      { key: 'state', label: '状态', builtin: true },
      { key: 'priority', label: '优先级', builtin: true },
      { key: 'created_at', label: '创建时间', builtin: true },
    ]
  }
  const labelOf = (key: string): string => {
    if (builtinLabels[key]) return builtinLabels[key]
    return type.form_schema?.fields?.find((f) => f.key === key)?.label ?? key
  }
  const configured = (type.view_config?.list_columns ?? []).map((c) => ({
    key: c.field, label: labelOf(c.field), builtin: !!builtinLabels[c.field],
  }))
  return configured.length > 0 ? configured : [{ key: 'title', label: '标题', builtin: true }]
})

function cellText(row: TicketListItem, key: string): string {
  switch (key) {
    case 'number':
      return `#${row.number}`
    case 'state':
      return row.state_name
    case 'priority':
      return priorityCopy[row.priority] ?? row.priority
    case 'assignee': {
      if (!row.assignee_id) return '未分派'
      return members.value.find((m) => m.user_id === row.assignee_id)?.display_name ?? `成员 ${row.assignee_id}`
    }
    case 'creator':
      return members.value.find((m) => m.user_id === row.creator_id)?.display_name ?? `用户 ${row.creator_id}`
    case 'created_at':
    case 'updated_at':
    case 'closed_at':
      return row[key] ? new Date(row[key] as string).toLocaleString('zh-CN', { hour12: false }) : '—'
    default: {
      const v = row.custom_fields?.[key]
      if (v === undefined || v === null || v === '') return '—'
      const opts = activeType.value?.form_schema?.fields?.find((f) => f.key === key)?.options
      if (Array.isArray(v)) {
        return v.map((x) => opts?.find((o) => o.value === x)?.label ?? String(x)).join('、')
      }
      if (typeof v === 'boolean') return v ? '是' : '否'
      return opts?.find((o) => o.value === v)?.label ?? String(v)
    }
  }
}

const stateOptions = computed(() => {
  const t = types.value.find((x) => x.id === filter.type)
  return t ? t.workflow.states.map((s) => ({ key: s.key, name: s.name })) : []
})

watch(() => filter.type, () => {
  activeType.value = types.value.find((t) => t.id === filter.type) ?? null
})

onMounted(async () => {
  const [typePage, memberPage] = await Promise.all([
    listTicketTypes(1, 100),
    meStore.role === 'requester' ? Promise.resolve({ data: [] as MemberView[] }) : listMembers(1, 100),
  ])
  types.value = typePage.data ?? []
  members.value = (memberPage.data ?? []).filter((m) => m.role !== 'requester')
  await load()
})
</script>

<template>
  <div>
    <h2 class="page__title">处理工作台</h2>
    <div class="block">
      <div class="console-list__filters">
        <el-select
          v-model="filter.type"
          placeholder="全部类型"
          clearable
          style="width: 180px"
          @change="onFilterChange"
        >
          <el-option v-for="t in types" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
        <el-select
          v-model="filter.state"
          placeholder="全部状态"
          clearable
          style="width: 150px"
          @change="onFilterChange"
        >
          <el-option v-for="s in stateOptions" :key="s.key" :label="s.name" :value="s.key" />
        </el-select>
        <el-select
          v-model="filter.assignee"
          placeholder="全部处理人"
          clearable
          style="width: 160px"
          @change="onFilterChange"
        >
          <el-option
            v-for="m in members"
            :key="m.user_id"
            :label="m.display_name || m.email"
            :value="m.user_id"
          />
        </el-select>
        <el-input
          v-model="filter.keyword"
          placeholder="搜索标题或工单号"
          clearable
          style="width: 220px"
          @keyup.enter="onFilterChange"
          @clear="onFilterChange"
        />
        <el-button type="primary" @click="onFilterChange">查询</el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="items"
        @row-click="(row: TicketListItem) => router.push(`/console/ticket/${row.id}`)"
      >
        <el-table-column
          v-for="col in columns"
          :key="col.key"
          :label="col.label"
          :min-width="col.key === 'title' ? 220 : col.builtin ? 130 : 110"
          :show-overflow-tooltip="col.key === 'title'"
        >
          <template #default="{ row }">
            <el-tag
              v-if="col.key === 'state'"
              :type="row.closed_at ? 'info' : 'success'"
              effect="plain"
            >
              {{ cellText(row, col.key) }}
            </el-tag>
            <el-tag v-else-if="col.key === 'priority'" :type="priorityType[row.priority] ?? 'info'" effect="plain">
              {{ cellText(row, col.key) }}
            </el-tag>
            <span v-else :class="{ 'console-list__number': col.key === 'number' }">
              {{ cellText(row, col.key) }}
            </span>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && items.length === 0" description="没有符合条件的工单" />
      <div class="console-list__pager">
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
.console-list__filters {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-3);
  flex-wrap: wrap;
}

.console-list__number {
  font-family: var(--font-mono);
}

.console-list__pager {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-3);
}

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
