/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // 当前工作区从 C: 入口访问、真实目录落在 D:；Windows junction 下若把 root 归一到真实路径，
  // Vitest 的 setupFiles 会被错误解析成 /src/...。这里显式保留入口路径。
  resolve: {
    preserveSymlinks: true,
  },
  // vitest 配置：jsdom 环境 + react 插件复用
  test: {
    environment: 'jsdom',
    globals: true,
    // 在 preserveSymlinks 下保留入口路径后，使用标准的项目根相对路径即可稳定解析。
    setupFiles: ['/src/setupTests.ts'],
  },
})
