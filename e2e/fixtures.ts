// Test fixture: a fresh taskapp server with the demo data for every test.
// The server is started through the same black-box harness as the API tests
// (apitest/src/harness.ts), so TASKAPP_BIN can point at any implementation.
import { test as base, expect, type Page } from "@playwright/test";
import { startServer, type Client, type Server } from "../apitest/src/harness.js";

/** Fixed tokens created by `seed --preset demo`. */
const DEMO_TOKENS = { owner: "tk_demo_owner", alice: "tk_demo_alice", claude: "tk_demo_claude", codex: "tk_demo_codex" } as const;

export interface Demo {
  server: Server;
  /** API client authenticated as a demo user, for setting up state or checking the result. */
  api(who: keyof typeof DEMO_TOKENS): Client;
}

export const test = base.extend<{ demo: Demo; serveArgs: string[] }>({
  /** extra `taskapp serve` flags, e.g. test.use({ serveArgs: ["--require-login"] }) */
  serveArgs: [[], { option: true }],
  demo: async ({ serveArgs }, use) => {
    const server = await startServer({ preset: "demo", serveArgs });
    await use({ server, api: (who) => server.raw(`Bearer ${DEMO_TOKENS[who]}`) });
    await server.stop();
  },
  baseURL: async ({ demo }, use) => use(demo.server.baseUrl),
});

export { expect };

/** The detail panel (peek) on the right of a list. */
export const peek = (page: Page) => page.getByTestId("issue-detail");
