// Settings: API tokens (shown once) and the data export (NFR-02).
import { readFileSync } from "node:fs";
import { expect, test } from "../fixtures.js";

test("issues an agent token that works for the API", async ({ page, demo }) => {
  await page.goto("/settings");
  await page.locator("select").first().selectOption({ label: "Claude（エージェント）" });
  await page.getByPlaceholder("用途（例: claude-code）").fill("e2e");
  await page.getByRole("button", { name: "発行" }).click();
  await expect(page.getByText("@claude のトークン「e2e」。下の MCP・Jira のコマンドにも入っています。")).toBeVisible();
  const token = (await page.getByTestId("shown-token").getByText(/^tk_/).textContent())!.trim();
  const me = await demo.server.raw(`Bearer ${token}`).get("/api/v1/me");
  expect(me.body.username).toBe("claude");
  // The commands use the token, and it can be shown again after a reload.
  await expect(page.getByTestId("mcp").getByText(`--header "Authorization: Bearer ${token}"`, { exact: false }).first()).toBeVisible();
  await page.reload();
  await expect(page.getByTestId("shown-token")).toHaveCount(0);
  await page.getByTestId("token-e2e").getByTitle("表示してコマンドに使う").click();
  await expect(page.getByTestId("shown-token").getByText(token, { exact: true })).toBeVisible();
  await expect(page.getByTestId("mcp").getByText(`Bearer ${token}`, { exact: false }).first()).toBeVisible();
});

test("downloads the JSON export", async ({ page }) => {
  await page.goto("/settings");
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("link", { name: "JSON をエクスポート" }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/^taskapp-export-\d{8}-\d{6}\.json$/);
  const dump = JSON.parse(readFileSync((await download.path())!, "utf8"));
  expect(dump.format).toBe(1);
  expect(dump.tables.issues).toHaveLength(21);
});

test("shows the automatic backups and takes one now", async ({ page, demo }) => {
  await page.goto("/settings");
  const box = page.getByTestId("auto-backup");
  await expect(box.getByText("有効")).toBeVisible();
  await expect(box.getByText(/1 日ごと/)).toBeVisible();
  // The server backs up once on start.
  await expect(box.getByText(/^taskapp-\d{8}-\d{6}\.db$/)).toHaveCount(1);
  await box.getByRole("button", { name: "今すぐバックアップ" }).click();
  await expect(box.getByText(/を作成しました$/)).toBeVisible();
  await expect(box.getByText(/^taskapp-\d{8}-\d{6}(-\d+)?\.db$/)).toHaveCount(2);
  expect((await demo.api("owner").get("/api/v1/backups")).body.files).toHaveLength(2);
});

test("MCP commands follow the chosen Claude Code scope", async ({ page }) => {
  await page.goto("/settings");
  const box = page.getByTestId("mcp");
  await expect(box.getByText(/^claude mcp add --scope user --transport http taskapp http:\/\/.+\/mcp --header "Authorization: Bearer <token>"$/)).toBeVisible();
  await box.getByTestId("mcp-scope-local").click();
  await expect(box.getByText(/^claude mcp add --scope local --transport http /)).toBeVisible();
  await expect(box.getByText("claude mcp remove taskapp -s local")).toBeVisible();
  // project scope goes into .mcp.json in git: the token comes from the environment.
  await box.getByTestId("mcp-scope-project").click();
  await expect(box.getByText(/--scope project --transport http .+ --header 'Authorization: Bearer \$\{TASKAPP_TOKEN\}'$/)).toBeVisible();
  await expect(box.getByText(/-e 'TASKAPP_TOKEN=\$\{TASKAPP_TOKEN\}' -- /)).toBeVisible();
  await expect(box.getByText("export TASKAPP_TOKEN=<token>")).toBeVisible();
});

test("shows how close the data is to the checked scale", async ({ page, demo }) => {
  await page.goto("/settings");
  const box = page.getByTestId("capacity");
  const issues = (await demo.api("owner").get("/api/v1/capacity")).body.counts.issues;
  await expect(box.getByTestId("cap-issues")).toContainText(`${issues} / 10,000`);
  await expect(box.getByTestId("cap-issues")).toHaveAttribute("data-level", "ok");
  await expect(box.getByTestId("cap-load")).toContainText(`${issues} / 2,000`);
  await expect(box.getByTestId("cap-map")).toContainText("最大のプロジェクト（APP）");
  await expect(box.getByTestId("cap-disk")).toContainText("taskapp のデータ");
});
