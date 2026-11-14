import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { copyFileSync, existsSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = dirname(fileURLToPath(import.meta.url))

// Single logo source: docs/logo.svg is the one true file; the favicon is a
// build-time copy (an artifact, not a second source).
const logoSrc = resolve(root, '../docs/logo.svg')
const faviconDst = resolve(root, 'public/favicon.svg')
if (existsSync(logoSrc)) {
  mkdirSync(dirname(faviconDst), { recursive: true })
  copyFileSync(logoSrc, faviconDst)
}

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
