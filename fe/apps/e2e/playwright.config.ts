import { defineConfig, devices } from "@playwright/test"

// The API runs outside Playwright (see README): API_PROXY_TARGET points the
// admin preview server at it, so the browser sees one origin like production.
const api = process.env.API_PROXY_TARGET ?? "http://localhost:3010"

export default defineConfig({
  testDir: "tests",
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["list"]] : "list",
  use: {
    baseURL: "http://localhost:4173",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "pnpm --filter admin exec vite preview --strictPort",
    url: "http://localhost:4173",
    env: { API_PROXY_TARGET: api },
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
})
