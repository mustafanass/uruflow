import { readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { gzipSync } from 'node:zlib'

import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

function compressAssets(directory: string) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) compressAssets(path)
    else if (/\.(html|js|css|svg|json)$/.test(entry.name)) {
      const source = readFileSync(path)
      const compressed = gzipSync(source, { level: 9 })
      if (compressed.length < source.length)
        writeFileSync(path + '.gz', compressed)
    }
  }
}

export default defineConfig({
  plugins: [
    react(),
    {
      name: 'keep-assets-directory',
      closeBundle() {
        compressAssets(resolve(import.meta.dirname, '../internal/web/assets'))
        writeFileSync(
          resolve(import.meta.dirname, '../internal/web/assets/.keep'),
          '',
        )
      },
    },
  ],
  resolve: { alias: { '@': resolve(import.meta.dirname, 'src') } },
  build: {
    outDir: '../internal/web/assets',
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    proxy: {
      '/api': {
        target: process.env.URUFLOW_API_URL || 'http://127.0.0.1:9002',
        changeOrigin: true,
      },
    },
  },
})
