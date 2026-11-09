// Issue list, detail panel, creation and live updates (ISS-01..05, VIEW-01).
import { expect, peek, test } from "../fixtures.js";

test("my tasks shows the owner's issues and opens the detail panel", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/my/);
  await expect(page.getByText("ユーザーとしてログインしたい")).toBeVisible();
  await page.getByText("ユーザーとしてログインしたい").click();
  await expect(peek(page)).toHaveAttribute("data-issue-key", "APP-2");
  await page.keyboard.press("Escape");
  await expect(peek(page)).toHaveCount(0);
});

test("changes the state from the detail panel", async ({ page, demo }) => {
  await page.goto("/p/APP/issues?peek=APP-15");
  const detail = peek(page);
  await detail.getByRole("button", { name: "Todo" }).click();
  await page.getByRole("button", { name: "In Progress" }).last().click();
  await expect(detail.getByRole("button", { name: "In Progress" })).toBeVisible();
  await expect.poll(async () => (await demo.api("owner").get("/api/v1/issues/APP-15")).body.state.name).toBe("In Progress");
});

test("creates an issue with the C shortcut", async ({ page, demo }) => {
  await page.goto("/p/APP/issues");
  await expect(page.getByText("README を整備する")).toBeVisible();
  await page.keyboard.press("c");
  await page.getByPlaceholder("タイトル").fill("E2E で作ったタスク");
  await page.getByPlaceholder("タイトル").press("Control+Enter");
  await expect(peek(page)).toHaveAttribute("data-issue-key", "APP-22");
  await expect(peek(page).getByRole("textbox").first()).toHaveValue("E2E で作ったタスク");
  await expect(page.getByRole("main").getByText("E2E で作ったタスク")).toBeVisible();
  expect((await demo.api("owner").get("/api/v1/issues/APP-22")).body.title).toBe("E2E で作ったタスク");
});

test("reflects changes made by an agent without reloading", async ({ page, demo }) => {
  await page.goto("/p/APP/issues");
  await expect(page.getByText("search API")).toBeVisible();
  const res = await demo.api("claude").patch("/api/v1/issues/APP-15", { title: "search API（エージェントが更新）" });
  expect(res.status).toBe(200);
  await expect(page.getByText("search API（エージェントが更新）")).toBeVisible();
});
