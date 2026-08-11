#!/usr/bin/env node
// Captures the README/docs screenshots against a running DEMO_MODE stack
// (see ../../docker-compose.demo.yml and CONTRIBUTING.md's "Mode démo"
// section). Plain `playwright` (not @playwright/test) — a single linear
// login-then-loop script has no need for the test-runner's isolation/
// reporter machinery, and this avoids adding a devDependency that isn't
// otherwise used anywhere in the repo.
//
// Usage: BASE_URL=http://localhost:8080 ADMIN_USER=demo ADMIN_PASSWORD=... \
//        node scripts/capture-screenshots.mjs

import { chromium } from 'playwright'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'
const ADMIN_USER = process.env.ADMIN_USER || 'demo'
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'DemoPass123!'
const OUTPUT_DIR = process.env.OUTPUT_DIR || path.resolve(__dirname, '../../screenshot')

// Route -> output filename, PascalCase matching the existing screenshot/
// naming convention exactly. demo-web-01 is seeded by cmd/seed-demo.
const ROUTES = [
  { path: '/', file: 'Dashboard.png' },
  { path: '/docker', file: 'Docker.png' },
  { path: '/apt', file: 'APT.png' },
  { path: '/audit', file: 'Audit.png' },
  { path: '/hosts/demo-web-01', file: 'HostDetail.png' },
  { path: '/proxmox', file: 'Proxmox.png' },
  { path: '/monitoring', file: 'Monitoring.png' },
]

// Matches the pixel dimensions of the existing committed screenshots
// (~2879x1799 = 1440x900 CSS px at deviceScaleFactor 2, viewport-only —
// this app's shell is a fixed-height layout with internal scroll
// containers, not a page that grows with content).
const VIEWPORT = { width: 1440, height: 900 }
const DEVICE_SCALE_FACTOR = 2

// "Fully loaded" detection with no arbitrary sleep: network-idle, then wait
// for either the shared LoadingSkeleton component to detach (used by
// Dashboard/APT/Audit/HostDetail/Proxmox) or a content selector to appear
// (covers Docker/Monitoring, which don't use LoadingSkeleton) — whichever
// signal a given view actually uses. Both waits are best-effort: a view that
// never shows either (already loaded before Playwright's selector attaches)
// shouldn't block the capture.
async function waitForLoaded(page) {
  await page
    .locator('.loading-skeleton')
    .first()
    .waitFor({ state: 'detached', timeout: 10000 })
    .catch(() => {})
  await page
    .locator('table.card-table tbody tr, .empty-state, canvas')
    .first()
    .waitFor({ state: 'visible', timeout: 10000 })
    .catch(() => {})
}

async function main() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE_FACTOR,
  })
  const page = await context.newPage()

  try {
    await login(page)
    for (const route of ROUTES) {
      await capture(page, route)
    }
  } finally {
    await browser.close()
  }
}

async function login(page) {
  await page.goto(`${BASE_URL}/login`, { waitUntil: 'networkidle' })
  await page.fill('input[name="username"]', ADMIN_USER)
  await page.fill('input[name="password"]', ADMIN_PASSWORD)
  await page.click('button[type="submit"]')
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 15000 })
}

async function capture(page, route) {
  const url = `${BASE_URL}${route.path}`
  await page.goto(url, { waitUntil: 'networkidle' })
  await waitForLoaded(page)
  const outPath = path.join(OUTPUT_DIR, route.file)
  await page.screenshot({ path: outPath, fullPage: false })
  console.log(`captured ${route.path} -> ${outPath}`)
}

main().catch((err) => {
  console.error('capture-screenshots failed:', err)
  process.exit(1)
})
