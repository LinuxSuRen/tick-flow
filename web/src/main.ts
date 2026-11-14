import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './style.css'
import App from './App.vue'
import router from './router'
import { meStore } from './stores/me'
import { tenantStore } from './stores/tenant'
import { fetchCurrentTenant, getToken } from './api'

async function boot() {
  if (getToken()) {
    try {
      await meStore.boot()
    } catch {
      // fetchMe already handled 401 (token cleared, redirected to login)
    }
  }
  if (meStore.tenant) {
    tenantStore.apply({ name: meStore.tenant.name })
    try {
      const tenant = await fetchCurrentTenant()
      tenantStore.apply({
        name: tenant.name,
        primary_color: tenant.theme?.primary_color,
        logo_url: tenant.theme?.logo_url ?? tenant.logo_url,
      })
    } catch {
      // branding is cosmetic; the app continues with defaults
    }
  }
}

void boot().finally(() => {
  const app = createApp(App)
  app.use(ElementPlus)
  app.use(router)
  app.mount('#app')
})
