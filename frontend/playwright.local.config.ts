import { defineConfig, devices } from '@playwright/test'
export default defineConfig({
  testDir: './tests/e2e',
  reporter: 'list',
  use: { baseURL: 'http://localhost:5173', launchOptions: { executablePath: '/opt/pw-browsers/chromium' } },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: { command: 'npx vite --port 5173', url: 'http://localhost:5173', reuseExistingServer: true },
})
