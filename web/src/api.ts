// api.ts is the single HTTP module for the whole frontend. Every backend
// call goes through request() which owns: the 15s timeout, friendly network
// error wrapping, the error envelope mapping, the {data} unwrap and the
// 401-to-login redirect.

export class ApiError extends Error {
  code: string

  constructor(message: string, code: string) {
    super(message)
    this.code = code
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
const authPaths = ['/api/auth/login', '/api/auth/setup']

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
    try {
      const body = await resp.json()
      code = body?.error?.code ?? code
    } catch {
      // non-JSON error body; keep the fallback copy
    }
    throw new ApiError(errorCopy[code] ?? `请求失败，请稍后重试`, code)
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

export interface Page<T> {
  data: T[]
  page: number
  page_size: number
  total: number
}

export interface TenantBrief {
  id: number
  name: string
  slug: string
  role: string
}

export interface Me {
  user: { id: number; email: string; display_name: string }
  tenant: { id: number; name: string; role: string } | null
  tenants: TenantBrief[]
}

// ---- auth ----

export function setup(payload: {
  tenant_name: string
  slug: string
  admin_email: string
  admin_password: string
  admin_name: string
}) {
  return post<never>('/api/auth/setup', payload)
}

export interface LoginResult {
  token: string
  me: Me
}

export function login(email: string, password: string) {
  return post<LoginResult>('/api/auth/login', { email, password })
}

export function selectTenant(tenant_id: number) {
  return post<LoginResult>('/api/auth/select-tenant', { tenant_id })
}

export function fetchMe() {
  return request<Me>('/api/me')
}
