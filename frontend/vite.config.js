import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'fs'
import { dirname, resolve } from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const pkg = JSON.parse(readFileSync(resolve(__dirname, 'package.json'), 'utf-8'))

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined

          if (id.includes('cytoscape') || id.includes('d3-force')) return 'vendor-graph'
          if (id.includes('chart.js') || id.includes('vue-chartjs')) return 'vendor-chart'
          if (id.includes('/d3-')) return 'vendor-d3'
          if (id.includes('world-atlas') || id.includes('topojson-client')) return 'vendor-map'
          if (id.includes('vue') || id.includes('pinia') || id.includes('vue-router')) return 'vendor-vue'
          if (id.includes('axios')) return 'vendor-http'
          if (id.includes('dayjs')) return 'vendor-date'
          if (id.includes('@tabler')) return 'vendor-tabler'

          return 'vendor-misc'
        },
      },
    },
  },
})
