import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// Dedicated Vitest config. Component tests run in happy-dom and act as a
// regression safety net before/while refactoring large SFCs.
export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify('test'),
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    include: ['src/**/*.{test,spec}.{ts,js}'],
    // Browser-mode tests run under a separate config (vitest.browser.config.ts).
    exclude: ['**/node_modules/**', '**/*.browser.test.ts'],
    setupFiles: ['src/test/setup.ts'],
  },
})
