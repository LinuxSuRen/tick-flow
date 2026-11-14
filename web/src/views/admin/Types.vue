<script setup lang="ts">
// Ticket type management with the P0 JSON editor fallback (issue #5):
// validate on the backend; the editor only formats JSON.
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ApiError,
  createTicketType,
  listTicketTypes,
  setTicketTypeStatus,
  updateTicketType,
  type TicketTypeView,
} from '../../api'

const items = ref<TicketTypeView[]>([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 50 })

async function load() {
  loading.value = true
  try {
    const result = await listTicketTypes(page.page, page.page_size)
    items.value = result.data ?? []
    total.value = result.total
  } finally {
    loading.value = false
  }
}

const editor = reactive({
  visible: false,
  editingId: 0,
  draft: '',
  saving: false,
})

function openCreate() {
  editor.editingId = 0
  editor.draft = JSON.stringify(
    {
      slug: 'my-flow',
      name: '新流程',
      form_schema: {
        fields: [{ key: 'remark', label: '备注', type: 'textarea' }],
      },
      workflow: {
        states: [
          { key: 'pending', name: '待受理', type: 'initial' },
          { key: 'closed', name: '已关闭', type: 'final' },
        ],
        transitions: [
          { from: 'pending', action: 'close', to: 'closed', roles: ['agent', 'tenant_admin'] },
        ],
      },
      sla_policy: {},
      assignment: { mode: 'manual' },
      view_config: {
        portal: { title: '新流程', description: '', success_text: '已提交' },
        list_columns: [{ field: 'number' }, { field: 'title' }, { field: 'state' }],
        detail_groups: [{ title: '基础信息', fields: ['number', 'title', 'state'] }],
      },
    },
    null,
    2,
  )
  editor.visible = true
}

function openEdit(t: TicketTypeView) {
  editor.editingId = t.id
  editor.draft = JSON.stringify(
    {
      slug: t.slug,
      name: t.name,
      icon: t.icon || undefined,
      form_schema: t.form_schema,
      workflow: t.workflow,
      sla_policy: t.sla_policy,
      assignment: t.assignment,
      view_config: t.view_config,
    },
    null,
    2,
  )
  editor.visible = true
}

async function save() {
  let payload: unknown
  try {
    payload = JSON.parse(editor.draft)
  } catch {
    ElMessage.error('JSON 格式有误，请检查后重试')
    return
  }
  editor.saving = true
  try {
    if (editor.editingId) {
      await updateTicketType(editor.editingId, payload)
    } else {
      await createTicketType(payload)
    }
    editor.visible = false
    ElMessage.success('已保存')
    await load()
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.fields ? Object.values(err.fields).join('；') : err.message)
    }
  } finally {
    editor.saving = false
  }
}

async function toggleStatus(t: TicketTypeView) {
  const next = t.status === 'enabled' ? 'disabled' : 'enabled'
  const verb = next === 'enabled' ? '启用' : '停用'
  await ElMessageBox.confirm(`确定${verb}「${t.name}」吗？`, '确认', { type: 'warning' })
  try {
    await setTicketTypeStatus(t.id, next)
    ElMessage.success(`已${verb}`)
    await load()
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page__title-line">
      <h2 class="page__title">工单类型</h2>
      <el-button type="primary" @click="openCreate">新建类型</el-button>
    </div>
    <div class="block">
      <el-table v-loading="loading" :data="items">
        <el-table-column prop="name" label="名称" width="160" />
        <el-table-column prop="slug" label="标识" width="160" />
        <el-table-column label="来源" width="90">
          <template #default="{ row }">
            <el-tag :type="row.is_builtin ? 'success' : 'info'" effect="plain" size="small">
              {{ row.is_builtin ? '内置' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'enabled' ? 'success' : 'danger'" effect="plain" size="small">
              {{ row.status === 'enabled' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="字段数" width="90">
          <template #default="{ row }">{{ row.form_schema?.fields?.length ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" :type="row.status === 'enabled' ? 'danger' : 'success'" plain @click="toggleStatus(row)">
              {{ row.status === 'enabled' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog
      v-model="editor.visible"
      :title="editor.editingId ? '编辑类型' : '新建类型'"
      width="760"
      top="6vh"
    >
      <el-alert
        v-if="!editor.editingId"
        type="info"
        :closable="false"
        show-icon
        title="结构说明见 docs/openapi.yaml 与种子模板 seed/，保存时后端逐字段校验"
        style="margin-bottom: 12px"
      />
      <el-input
        v-model="editor.draft"
        type="textarea"
        :rows="22"
        spellcheck="false"
        class="type-editor"
      />
      <template #footer>
        <el-button @click="editor.visible = false">取消</el-button>
        <el-button type="primary" :loading="editor.saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page__title-line {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.type-editor :deep(textarea) {
  font-family: var(--font-mono);
  font-size: 13px;
}
</style>
