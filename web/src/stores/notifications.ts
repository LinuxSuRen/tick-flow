// Unread notification count, polled every 30s while the app is mounted.
// The interval lives here so every layout shares one poller.
import { reactive } from 'vue'
import { listNotifications } from '../api'

export const notificationsStore = reactive({
  unreadCount: 0,
  polling: false,
  _timer: null as ReturnType<typeof setInterval> | null,

  async refresh() {
    try {
      const page = await listNotifications(true, 1, 1)
      this.unreadCount = page.total
    } catch {
      // transient errors keep the last known count; the bell keeps working
    }
  },

  startPolling() {
    if (this.polling) {
      return
    }
    this.polling = true
    void this.refresh()
    this._timer = setInterval(() => void this.refresh(), 30_000)
  },

  stopPolling() {
    this.polling = false
    if (this._timer) {
      clearInterval(this._timer)
      this._timer = null
    }
  },
})
