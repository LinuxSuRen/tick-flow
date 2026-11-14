import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { copyFileSync, existsSync, mkdirSync, writeFileSync } from 'node:fs'
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

// vite empties dist on build, which would drop the embed placeholder and
// break `go build` on a fresh checkout; write it back after every bundle.
function keepEmbedPlaceholder(): Plugin {
  return {
    name: 'keep-embed-placeholder',
    closeBundle() {
      mkdirSync(resolve(root, 'dist'), { recursive: true })
      writeFileSync(resolve(root, 'dist/.gitkeep'), '')
    },
  }
}

// Dev proxy forwards API calls to the Go backend so the browser only ever
// sees same-origin requests; in production the backend embeds this build.
export default defineConfig({
  plugins: [vue(), keepEmbedPlaceholder()],
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
