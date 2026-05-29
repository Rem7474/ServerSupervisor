import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { playwright } from '@vitest/browser-playwright'

// Real-browser test config (Playwright/Chromium). Used for components whose
// behaviour depends on actual rendering — canvas charts (Chart.js) and the
// D3/SVG world map — which happy-dom cannot execute. Run via `npm run test:browser`.
// Files: src/**/*.browser.test.ts
export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify('test'),
  },
  test: {
    globals: true,
    include: ['src/**/*.browser.test.ts'],
    browser: {
      enabled: true,
      provider: playwright(),
      headless: true,
      instances: [{ browser: 'chromium' }],
    },
  },
})
