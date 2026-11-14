// Unread notification count, polled every 30s while the app is mounted.
// The interval lives here so every layout shares one poller.
import { reactive } from 'vue'

export const notificationsStore = reactive({
  unreadCount: 0,
  polling: false,
  _timer: null as ReturnType<typeof setInterval> | null,

  async refresh() {
    // wired to GET /api/notifications in the notification batch; the
    // skeleton keeps the store shape so layouts can already bind to it
    this.unreadCount = 0
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
