<script setup lang="ts">
// Shared ticket detail: grouped display, timeline with dual reply entries,
// state actions, assignment and attachments. The backend already filters
// transitions, internal notes and cross-tenant access per role, so the
// view stays declarative (issues #17/#18).
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  addComment,
  ApiError,
  assignTicket,
  fetchTicket,
  fetchTicketType,
  fetchTimeline,
  listMembers,
  transitionTicket,
  uploadAttachment,
  downloadAttachmentUrl,
  type MemberView,
  type TicketDetail,
  type TicketTypeView,
  type TimelineEntry,
} from '../api'
import { meStore } from '../stores/me'
import TimelineList from './TimelineList.vue'
import StateActions from './StateActions.vue'

const props = defineProps<{ id: number }>()
const router = useRouter()

const ticket = ref<TicketDetail | null>(null)
const type = ref<TicketTypeView | null>(null)
const entries = ref<TimelineEntry[]>([])
const members = ref<MemberView[]>([])
const loading = ref(false)
const acting = ref(false)

const isStaff = computed(() => ['agent', 'tenant_admin'].includes(meStore.role))

const priorityCopy: Record<string, string> = {
  low: '低',
  normal: '普通',
  high: '高',
  urgent: '紧急',
}
const priorityType: Record<string, 'info' | 'success' | 'warning' | 'danger'> = {
  low: 'info',
  normal: 'success',
  high: 'warning',
  urgent: 'danger',
}

async function load() {
  loading.value = true
  try {
    ticket.value = await fetchTicket(props.id)
    type.value = await fetchTicketType(ticket.value.ticket_type.id)
    const page = await fetchTimeline(props.id, 1, 100)
    entries.value = page.data ?? []
    if (isStaff.value) {
      const mp = await listMembers(1, 100)
      members.value = (mp.data ?? []).filter((m) => m.role !== 'requester')
    }
  } catch (err) {
    if (err instanceof ApiError && err.code === 'NOT_FOUND') {
      ElMessage.error('工单不存在或没有权限查看')
      router.replace(meStore.role === 'requester' ? '/mine' : '/console')
      return
    }
    throw err
  } finally {
    loading.value = false
  }
}

// Grouped display resolves builtin fields and custom fields by key.
function fieldLabel(key: string): string {
  const f = type.value?.form_schema?.fields?.find((x) => x.key === key)
  if (f) return f.label
  const builtin: Record<string, string> = {
    number: '工单号',
    title: '标题',
    content: '描述',
    state: '状态',
    priority: '优先级',
    assignee: '处理人',
    creator: '创建人',
    created_at: '创建时间',
    updated_at: '更新时间',
    closed_at: '关闭时间',
  }
  return builtin[key] ?? key
}

function fieldText(key: string): string {
  const t = ticket.value
  if (!t) return ''
  switch (key) {
    case 'number':
      return `#${t.number}`
    case 'title':
      return t.title
    case 'content':
      return t.content || '—'
    case 'state':
      return t.state_name
    case 'priority':
      return priorityCopy[t.priority] ?? t.priority
    case 'assignee':
      return memberName(t.assignee_id)
    case 'creator':
      return memberName(t.creator_id) ?? `用户 ${t.creator_id}`
    case 'created_at':
    case 'updated_at':
    case 'closed_at':
      return t[key] ? new Date(t[key] as string).toLocaleString('zh-CN', { hour12: false }) : '—'
    default: {
      const v = t.custom_fields?.[key]
      if (v === undefined || v === null || v === '') return '—'
      if (Array.isArray(v)) {
        const labels = (type.value?.form_schema?.fields?.find((f) => f.key === key)?.options ?? [])
        return v
          .map((item) => labels.find((o) => o.value === item)?.label ?? String(item))
          .join('、')
      }
      if (typeof v === 'boolean') return v ? '是' : '否'
      const opt = type.value?.form_schema?.fields
        ?.find((f) => f.key === key)?.options
        ?.find((o) => o.value === v)
      return opt?.label ?? String(v)
    }
  }
}

function memberName(id: number | null): string {
  if (!id) return '未分派'
  const known = members.value.find((m) => m.user_id === id)
  if (known) return known.display_name || known.email
  if (ticket.value?.creator_id === id) return '创建人'
  return `成员 ${id}`
}

// ---- actions ----

async function onAction(action: string) {
  acting.value = true
  try {
    ticket.value = await transitionTicket(props.id, action)
    ElMessage.success('操作成功')
    await load()
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message)
    }
  } finally {
    acting.value = false
  }
}

const assignForm = reactive({ visible: false, assignee_id: undefined as number | undefined })

async function doAssign() {
  if (!assignForm.assignee_id) return
  acting.value = true
  try {
    ticket.value = await assignTicket(props.id, assignForm.assignee_id)
    assignForm.visible = false
    ElMessage.success('分派成功')
    await load()
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message)
    }
  } finally {
    acting.value = false
  }
}

// ---- replies ----

const replyTab = ref<'external' | 'internal'>('external')
const replyBody = ref('')
const sending = ref(false)

async function sendReply() {
  if (!replyBody.value.trim()) {
    ElMessage.warning('请输入内容')
    return
  }
  sending.value = true
  try {
    await addComment(props.id, replyBody.value, replyTab.value === 'internal')
    replyBody.value = ''
    ElMessage.success('已发送')
    const page = await fetchTimeline(props.id, 1, 100)
    entries.value = page.data ?? []
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message)
    }
  } finally {
    sending.value = false
  }
}

// ---- attachments ----

const uploading = ref(false)

async function onUpload(file: File) {
  uploading.value = true
  try {
    await uploadAttachment(props.id, file)
    ElMessage.success('上传成功')
    await load()
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message)
    }
  } finally {
    uploading.value = false
  }
  return false
}

async function download(id: number, filename: string) {
  try {
    const blob = await downloadAttachmentUrl(id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.error('附件下载失败，请稍后重试')
  }
}

function sizeOf(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

onMounted(load)
</script>

<template>
  <div v-if="ticket" class="detail">
    <div class="block detail__head">
      <div class="detail__titleline">
        <h2 class="detail__title">#{{ ticket.number }} {{ ticket.title }}</h2>
        <el-tag :type="ticket.closed_at ? 'info' : 'success'" size="large">{{ ticket.state_name }}</el-tag>
      </div>
      <div class="detail__meta">
        <el-tag :type="priorityType[ticket.priority] ?? 'info'" effect="plain">
          优先级：{{ priorityCopy[ticket.priority] ?? ticket.priority }}
        </el-tag>
        <span class="muted">{{ ticket.ticket_type.name }}</span>
        <span class="muted">创建于 {{ new Date(ticket.created_at).toLocaleString('zh-CN', { hour12: false }) }}</span>
        <span v-if="ticket.sla_deadlines?.resolve_at" class="muted">
          处理时限 {{ new Date(ticket.sla_deadlines.resolve_at).toLocaleString('zh-CN', { hour12: false }) }}
        </span>
      </div>
      <div class="detail__actions">
        <StateActions :transitions="ticket.transitions" :loading="acting" @action="onAction" />
        <el-button v-if="isStaff" @click="assignForm.visible = true">分派处理人</el-button>
      </div>
    </div>

    <div v-for="(group, gi) in type?.view_config?.detail_groups ?? []" :key="gi" class="block">
      <h3 class="detail__group-title">{{ group.title }}</h3>
      <el-descriptions :column="2" border>
        <el-descriptions-item v-for="f in group.fields" :key="f" :label="fieldLabel(f)">
          {{ fieldText(f) }}
        </el-descriptions-item>
      </el-descriptions>
    </div>

    <div class="block">
      <h3 class="detail__group-title">附件</h3>
      <div v-if="ticket.attachments.length === 0" class="muted">暂无附件</div>
      <div v-for="a in ticket.attachments" :key="a.id" class="detail__attachment">
        <span class="detail__attachment-name">{{ a.filename }}</span>
        <span class="muted">{{ sizeOf(a.size_bytes) }}</span>
        <el-button link type="primary" @click="download(a.id, a.filename)">下载</el-button>
      </div>
      <el-upload
        :auto-upload="false"
        :show-file-list="false"
        :on-change="(file: { raw?: File }) => file.raw && onUpload(file.raw)"
      >
        <el-button :loading="uploading" style="margin-top: 8px">上传附件</el-button>
      </el-upload>
    </div>

    <div class="block">
      <h3 class="detail__group-title">沟通记录</h3>
      <TimelineList :entries="entries" />
      <div class="detail__reply">
        <el-tabs v-if="isStaff" v-model="replyTab">
          <el-tab-pane label="对外回复" name="external" />
          <el-tab-pane label="内部备注" name="internal" />
        </el-tabs>
        <el-input
          v-model="replyBody"
          type="textarea"
          :rows="3"
          :placeholder="replyTab === 'internal' ? '仅处理团队可见' : '回复内容，支持 Markdown'"
        />
        <div class="detail__reply-submit">
          <el-button type="primary" :loading="sending" @click="sendReply">
            {{ isStaff && replyTab === 'internal' ? '添加内部备注' : '发送回复' }}
          </el-button>
        </div>
      </div>
    </div>

    <el-dialog v-model="assignForm.visible" title="分派处理人" width="360">
      <el-select
        v-model="assignForm.assignee_id"
        placeholder="选择处理人"
        style="width: 100%"
      >
        <el-option
          v-for="m in members"
          :key="m.user_id"
          :label="`${m.display_name}（${m.email}）`"
          :value="m.user_id"
        />
      </el-select>
      <template #footer>
        <el-button @click="assignForm.visible = false">取消</el-button>
        <el-button type="primary" :loading="acting" @click="doAssign">确定分派</el-button>
      </template>
    </el-dialog>
  </div>
  <div v-else-if="loading" class="block muted">加载中</div>
</template>

<style scoped>
.detail__titleline {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.detail__title {
  margin: 0;
  font-size: 18px;
}

.detail__meta {
  display: flex;
  gap: var(--space-4);
  align-items: center;
  margin-top: var(--space-2);
  flex-wrap: wrap;
}

.detail__actions {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-3);
  align-items: center;
}

.detail__group-title {
  margin: 0 0 var(--space-3);
  font-size: 15px;
}

.detail__attachment {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-1) 0;
}

.detail__attachment-name {
  font-family: var(--font-mono);
  color: var(--color-text-primary);
}

.detail__reply {
  margin-top: var(--space-4);
  border-top: 1px solid var(--color-border-subtle);
  padding-top: var(--space-3);
}

.detail__reply-submit {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-2);
}
</style>
