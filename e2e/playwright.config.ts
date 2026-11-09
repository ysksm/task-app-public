import { defineConfig, devices } from "@playwright/test";

// Each test starts its own server (fresh demo data on a free port), so tests
// run in parallel and never depend on each other's changes.
export default defineConfig({
  testDir: "tests",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  retries: 0,
  reporter: process.env.CI ? "list" : [["list"], ["html", { open: "never" }]],
  use: {
    ...devices["Desktop Chrome"],
    viewport: { width: 1440, height: 900 },
    locale: "ja-JP",
    timezoneId: "Asia/Tokyo",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
