import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { playwright } from '@vitest/browser-playwright'

// Real-browser test config (Playwright/Chromium). Used for components whose
// behaviour depends on actual rendering — ApexCharts (SVG) and the D3/SVG
// world map — which happy-dom cannot execute. Run via `npm run test:browser`.
// Files: src/**/*.browser.test.ts. Runs in CI (ci-frontend.yml).
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
    coverage: {
      provider: 'v8',
      // Separate directory from vitest.config.ts's happy-dom coverage run —
      // both feed SonarCloud (sonar-project.properties lists both lcov paths),
      // which merges per-file hit counts across report paths. This report
      // also lists every src/**/*.{ts,vue} file (not just the ones a browser
      // test executes) as a 0%-or-covered baseline, same as the happy-dom
      // report — merging two baselines is harmless (SonarCloud sums hit
      // counts per file/line across report paths).
      reportsDirectory: 'coverage-browser',
      reporter: ['lcov', 'text'],
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/**/*.{test,spec}.{ts,js}', 'src/**/*.browser.test.ts', 'src/types/generated.ts'],
    },
  },
})
