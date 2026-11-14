import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev proxy forwards API calls to the Go backend so the browser only ever
// sees same-origin requests; in production the backend embeds this build.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
