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
    // vue-i18n build flags: drop the Options-API path (this app is
    // `legacy: false`) and the devtools hooks from the production bundle.
    // Worth ~2.5 KB gzipped on vendor-vue.
    __VUE_I18N_FULL_INSTALL__: false,
    __VUE_I18N_LEGACY_API__: false,
    __INTLIFY_PROD_DEVTOOLS__: false,
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // No manualChunks: Rollup's default splitting respects the dynamic
    // import() boundaries that already exist in the codebase. The previous
    // manualChunks function grouped node_modules by package name, which
    // inadvertently pulled the Vue runtime into the 1.43 MB ApexCharts
    // chunk (because `id.includes('vue')` matched vue-related code before
    // the `vendor-vue` bucket was tested). That chunk was then
    // modulepreload'd in the entry, defeating the defineAsyncComponent in
    // apexChartTheme.ts and the IntersectionObserver in DashboardView.
    // The same mechanism pulled xterm.js into the entry via vendor-misc.
    // Measured impact: 3 126 KB → 1 420 KB brut, 779 KB → 316 KB gzip.
    chunkSizeWarningLimit: 900,
  },
})
