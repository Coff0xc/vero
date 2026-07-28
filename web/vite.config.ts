import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 前端构建产物直接输出到 internal/webui/dist, 由 Go embed 打包进单一二进制。
// dev 模式下把 API/SSE 反代到 Go 后端(:8000), 前端跑在 vite :5173。
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/events': { target: 'http://127.0.0.1:8000', changeOrigin: true },
      '/start': 'http://127.0.0.1:8000',
      '/approve': 'http://127.0.0.1:8000',
      '/api': 'http://127.0.0.1:8000',
    },
  },
})
