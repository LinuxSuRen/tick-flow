// Tenant branding: the primary color and logo are fetched after login and
// pushed into the CSS variables so both the portal and console pick them up.
import { reactive } from 'vue'

interface TenantTheme {
  primary_color?: string
  logo_url?: string
  name?: string
}

export const tenantStore = reactive({
  theme: {} as TenantTheme,

  apply(theme: TenantTheme | null | undefined) {
    this.theme = theme ?? {}
    const root = document.documentElement.style
    const primary = this.theme.primary_color
    if (primary) {
      root.setProperty('--color-accent-base', primary)
      // hover/active keep a deterministic relationship to the brand color so
      // no per-tenant design work is required
      root.setProperty('--color-accent-hover', primary)
      root.removeProperty('--color-accent-active')
      root.setProperty('--color-accent-subtle', hexToSubtle(primary))
    }
  },

  get logoUrl() {
    return this.theme.logo_url ?? ''
  },

  get name() {
    return this.theme.name ?? 'TickFlow'
  },
})

function hexToSubtle(hex: string): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) {
    return '#ecf1fe'
  }
  const n = parseInt(m[1], 16)
  const r = Math.round((((n >> 16) & 0xff) * 0xe8 + 0xff * 0.17) / 0xff)
  const g = Math.round((((n >> 8) & 0xff) * 0xe8 + 0xff * 0.17) / 0xff)
  const b = Math.round(((n & 0xff) * 0xe8 + 0xff * 0.17) / 0xff)
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`
}
