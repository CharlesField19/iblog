import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('.', import.meta.url))
export default defineConfig({
  root,
  base: '/',
  plugins: [vue()],
  build: { outDir: 'dist-rebuilt', emptyOutDir: true },
  server: { proxy: { '/api': 'http://127.0.0.1:8080' } },
})
