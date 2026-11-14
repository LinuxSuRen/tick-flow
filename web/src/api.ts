// api.ts is the single HTTP module for the whole frontend. Every backend
// call goes through request() which owns: the 15s timeout, friendly network
// error wrapping, the error envelope mapping, the {data} unwrap and the
// 401-to-login redirect.

export class ApiError extends Error {
  code: string
  fields?: Record<string, string>

  constructor(message: string, code: string, fields?: Record<string, string>) {
    super(message)
    this.code = code
    this.fields = fields
  }
}

// Stable backend codes mapped to user-visible copy. Unknown codes fall back
// to the generic message in request().
const errorCopy: Record<string, string> = {
  UNAUTHORIZED: '登录已过期，请重新登录',
  FORBIDDEN: '没有执行此操作的权限',
  NOT_FOUND: '内容不存在或已被移除',
  VALIDATION_FAILED: '提交的内容有误，请检查后重试',
  STATE_CONFLICT: '当前状态不允许此操作，请刷新后重试',
  DUPLICATED: '内容已存在，请勿重复提交',
  RATE_LIMITED: '操作过于频繁，请稍后重试',
  INTERNAL_ERROR: '服务开小差了，请稍后重试',
}

const TOKEN_KEY = 'tickflow_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

// Paths whose 401 means "wrong credentials" rather than "session expired";
// they must not trigger a login redirect loop.
const authPaths = ['/api/auth/login', '/api/auth/setup', '/api/auth/accept-invitation']

const TIMEOUT_MS = 15000

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS)
  const headers = new Headers(init.headers)
  if (!headers.has('Content-Type') && init.body && typeof init.body === 'string') {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  let resp: Response
  try {
    resp = await fetch(path, { ...init, headers, signal: controller.signal })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError('请求超时，请检查网络后重试', 'timeout')
    }
    throw new ApiError('无法连接后端服务，请确认服务已启动', 'network')
  } finally {
    clearTimeout(timer)
  }

  if (resp.status === 401 && !authPaths.includes(path)) {
    clearToken()
    window.location.href = '/login'
    throw new ApiError('登录已过期，请重新登录', 'UNAUTHORIZED')
  }

  if (!resp.ok) {
    let code = 'UNKNOWN'
    let fields: Record<string, string> | undefined
    try {
      const body = await resp.json()
      code = body?.error?.code ?? code
      // VALIDATION_FAILED messages are "field: reason; field: reason"
      if (code === 'VALIDATION_FAILED' && typeof body?.error?.message === 'string') {
        fields = {}
        for (const part of body.error.message.split(';')) {
          const idx = part.indexOf(': ')
          if (idx > 0) {
            fields[part.slice(0, idx)] = part.slice(idx + 2)
          }
        }
      }
    } catch {
      // non-JSON error body; keep the fallback copy
    }
    throw new ApiError(errorCopy[code] ?? '请求失败，请稍后重试', code, fields)
  }

  if (resp.status === 204) {
    return null as T
  }
  const text = await resp.text()
  if (!text) {
    return null as T
  }
  try {
    const body = JSON.parse(text)
    if (body && typeof body === 'object' && 'data' in body) {
      return body.data as T
    }
    return body as T
  } catch {
    return null as T
  }
}

function post<T>(path: string, payload?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
}

function patch<T>(path: string, payload?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PATCH',
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
}

function put<T>(path: string, payload?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PUT',
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
}

function del<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' })
}

export interface Page<T> {
  data: T[]
  page: number
  page_size: number
  total: number
}

// ---- shared domain shapes (snake_case on the wire) ----

export interface TenantBrief {
  id: number
  name: string
  slug: string
  role: string
}

export interface Me {
  user: { id: number; email: string; display_name: string }
  tenant: { id: number; name: string; slug: string; role: string } | null
  tenants: TenantBrief[]
  unread_count?: number
}

export interface FieldOption {
  value: string
  label: string
}

export interface FormField {
  key: string
  label: string
  type:
    | 'string'
    | 'textarea'
    | 'number'
    | 'select'
    | 'multiselect'
    | 'boolean'
    | 'date'
    | 'datetime'
    | 'attachments'
  placeholder?: string
  required?: boolean
  max_length?: number
  min?: number
  max?: number
  pattern?: string
  options?: FieldOption[]
  max_count?: number
}

export interface WorkflowState {
  key: string
  name: string
  type: 'initial' | 'normal' | 'final'
}

export interface WorkflowTransition {
  from: string
  action: string
  to: string
  roles: string[]
}

export interface Workflow {
  states: WorkflowState[]
  transitions: WorkflowTransition[]
}

export interface PortalConfig {
  title: string
  description: string
  success_text: string
}

export interface ViewConfig {
  portal: PortalConfig
  list_columns: { field: string }[]
  detail_groups: { title: string; fields: string[] }[]
}

export interface TicketTypeView {
  id: number
  slug: string
  name: string
  icon: string
  form_schema: { fields: FormField[] }
  workflow: Workflow
  sla_policy: { first_response_minutes?: number; resolve_minutes?: number }
  assignment: { mode: string }
  view_config: ViewConfig
  is_builtin: boolean
  status: string
  created_at: string
  updated_at: string
}

export interface AttachmentBrief {
  id: number
  filename: string
  size_bytes: number
  content_type: string
  created_at: string
}

export interface TransitionOption {
  action: string
  to: string
  to_name: string
}

export interface TicketDetail {
  id: number
  number: string
  ticket_type: { id: number; slug: string; name: string; icon: string }
  title: string
  content: string
  priority: string
  state: string
  state_name: string
  custom_fields: Record<string, unknown>
  creator_id: number
  assignee_id: number | null
  due_at: string | null
  sla_deadlines: { first_response_at?: string; resolve_at?: string }
  closed_at: string | null
  created_at: string
  updated_at: string
  transitions: TransitionOption[]
  attachments: AttachmentBrief[]
}

export interface TicketListItem {
  id: number
  number: string
  ticket_type: { id: number; slug: string; name: string; icon: string }
  title: string
  priority: string
  state: string
  state_name: string
  custom_fields: Record<string, unknown>
  creator_id: number
  assignee_id: number | null
  created_at: string
  updated_at: string
  closed_at: string | null
}

export interface TimelineEntry {
  id: number
  kind: 'comment' | 'internal_note' | 'state_change' | 'assignment' | 'system'
  author_id: number
  author: string
  body: string
  meta: Record<string, unknown>
  created_at: string
}

export interface NotificationItem {
  id: number
  kind: string
  title: string
  body: string
  ticket_id: number | null
  read_at: string | null
  created_at: string
}

export interface MemberView {
  id?: number
  user_id: number
  email: string
  display_name: string
  role: string
  status: string
  created_at?: string
}

export interface TenantView {
  id: number
  name: string
  slug: string
  logo_url: string
  theme: { primary_color?: string; logo_url?: string }
  status: string
}

export interface LoginResult {
  token: string
  me: Me
}

// ---- auth ----

export function setupFirstTenant(payload: {
  tenant_name: string
  slug: string
  admin_email: string
  admin_password: string
  admin_name: string
}) {
  return request<LoginResult>('/api/auth/setup', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function login(email: string, password: string) {
  return post<LoginResult>('/api/auth/login', { email, password })
}

export function selectTenant(tenant_id: number) {
  return post<LoginResult>('/api/auth/select-tenant', { tenant_id })
}

export function acceptInvitation(token: string, password: string) {
  return post<never>('/api/auth/accept-invitation', { token, password })
}

export function fetchMe() {
  return request<Me>('/api/me')
}

// ---- tenant & members ----

export function fetchCurrentTenant() {
  return request<TenantView>('/api/tenants/current')
}

export function updateTenantBranding(payload: {
  name?: string
  logo_url?: string
  theme?: { primary_color?: string; logo_url?: string }
}) {
  return patch<TenantView>('/api/tenants/current', payload)
}

export function listMembers(page = 1, page_size = 20) {
  return request<Page<MemberView>>(
    `/api/tenants/current/members?page=${page}&page_size=${page_size}`,
  )
}

export function inviteMember(payload: { email: string; role: string; display_name?: string }) {
  return post<{ member: MemberView; invite_token: string }>('/api/tenants/current/members', payload)
}

export function updateMemberRole(id: number, role: string) {
  return patch<MemberView>(
    `/api/tenants/current/members/${encodeURIComponent(id)}`,
    { role },
  )
}

export function removeMember(id: number) {
  return del<never>(`/api/tenants/current/members/${encodeURIComponent(id)}`)
}

// ---- ticket types ----

export function listTicketTypes(page = 1, page_size = 100) {
  return request<Page<TicketTypeView>>(`/api/ticket-types?page=${page}&page_size=${page_size}`)
}

export function fetchTicketType(id: number) {
  return request<TicketTypeView>(`/api/ticket-types/${encodeURIComponent(id)}`)
}

export function createTicketType(payload: unknown) {
  return post<TicketTypeView>('/api/ticket-types', payload)
}

export function updateTicketType(id: number, payload: unknown) {
  return put<TicketTypeView>(`/api/ticket-types/${encodeURIComponent(id)}`, payload)
}

export function setTicketTypeStatus(id: number, status: 'enabled' | 'disabled') {
  return post<TicketTypeView>(
    `/api/ticket-types/${encodeURIComponent(id)}/disable`,
    { status },
  )
}

// ---- tickets ----

export interface TicketQuery {
  page?: number
  page_size?: number
  type?: number
  state?: string
  assignee?: number
  priority?: string
  keyword?: string
}

export function listTickets(query: TicketQuery) {
  const params = new URLSearchParams()
  params.set('page', String(query.page ?? 1))
  params.set('page_size', String(query.page_size ?? 20))
  if (query.type) params.set('type', String(query.type))
  if (query.state) params.set('state', query.state)
  if (query.assignee) params.set('assignee', String(query.assignee))
  if (query.priority) params.set('priority', query.priority)
  if (query.keyword) params.set('keyword', query.keyword)
  return request<Page<TicketListItem>>(`/api/tickets?${params.toString()}`)
}

export function createTicket(payload: {
  ticket_type_id: number
  title: string
  content?: string
  priority?: string
  custom_fields?: Record<string, unknown>
}) {
  return post<TicketDetail>('/api/tickets', payload)
}

export function fetchTicket(id: number) {
  return request<TicketDetail>(`/api/tickets/${encodeURIComponent(id)}`)
}

export function transitionTicket(id: number, action: string, comment?: string) {
  return post<TicketDetail>(
    `/api/tickets/${encodeURIComponent(id)}/transitions`,
    { action, comment },
  )
}

export function assignTicket(id: number, assignee_id: number) {
  return post<TicketDetail>(
    `/api/tickets/${encodeURIComponent(id)}/assignments`,
    { assignee_id },
  )
}

// ---- timeline ----

export function addComment(id: number, body: string, is_internal = false) {
  return post<TimelineEntry>(`/api/tickets/${encodeURIComponent(id)}/comments`, {
    body,
    is_internal,
  })
}

export function fetchTimeline(id: number, page = 1, page_size = 50) {
  return request<Page<TimelineEntry>>(
    `/api/tickets/${encodeURIComponent(id)}/timeline?page=${page}&page_size=${page_size}`,
  )
}

// ---- attachments ----

export function uploadAttachment(ticketId: number, file: File) {
  const form = new FormData()
  form.append('file', file)
  return request<AttachmentBrief>(`/api/tickets/${encodeURIComponent(ticketId)}/attachments`, {
    method: 'POST',
    body: form,
  })
}

export async function downloadAttachmentUrl(id: number): Promise<Blob> {
  const headers = new Headers()
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const resp = await fetch(`/api/attachments/${encodeURIComponent(id)}/content`, { headers })
  if (!resp.ok) {
    throw new ApiError('附件下载失败，请稍后重试', 'ATTACHMENT')
  }
  return resp.blob()
}

// ---- notifications ----

export function listNotifications(unreadOnly = false, page = 1, page_size = 20) {
  return request<Page<NotificationItem>>(
    `/api/notifications?unread_only=${unreadOnly}&page=${page}&page_size=${page_size}`,
  )
}

export function markNotificationsRead(ids?: number[]) {
  return post<never>('/api/notifications/read', ids ? { ids } : {})
}
