// Module-level reactive stores, no state library (frontend convention).

import { reactive } from 'vue'
import { fetchMe, type Me } from '../api'

// me: the logged-in user, the active tenant and the role. booted() is called
// once from main.ts before mount.
export const meStore = reactive({
  user: null as Me['user'] | null,
  tenant: null as Me['tenant'] | null,
  tenants: [] as Me['tenants'],
  booted: false,

  async boot() {
    try {
      const me = await fetchMe()
      this.apply(me)
    } finally {
      this.booted = true
    }
  },

  apply(me: Me) {
    this.user = me.user
    this.tenant = me.tenant
    this.tenants = me.tenants ?? []
  },

  reset() {
    this.user = null
    this.tenant = null
    this.tenants = []
  },

  get role() {
    return this.tenant?.role ?? ''
  },
})
