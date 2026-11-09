// Command palette and keyboard shortcuts (M02, ISS-07).
import { expect, peek, test } from "../fixtures.js";

test("finds an issue by key and opens it", async ({ page }) => {
  await page.goto("/my");
  await expect(page.getByText("マイタスク").first()).toBeVisible();
  await page.keyboard.press("Control+k");
  const input = page.getByPlaceholder("移動先・操作・イシュー（キーやタイトル）を検索");
  await input.fill("APP-3");
  await expect(page.getByRole("button", { name: /ログイン画面を実装する/ })).toBeVisible();
  await input.press("Enter");
  await expect(page).toHaveURL(/\/p\/APP\/issues.*peek=APP-3/);
  await expect(peek(page)).toHaveAttribute("data-issue-key", "APP-3");
});

test("moves to a page from the palette", async ({ page }) => {
  await page.goto("/my");
  await page.getByRole("button", { name: /検索・移動/ }).click();
  await page.getByPlaceholder(/移動先/).fill("ロードマップ");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/p\/APP\/issues.*view=roadmap/);
});

test("G-key navigation and the shortcut list", async ({ page }) => {
  await page.goto("/my");
  await expect(page.getByText("ユーザーとしてログインしたい")).toBeVisible();
  await page.keyboard.press("g");
  await page.keyboard.press("a");
  await expect(page).toHaveURL(/\/inbox\/acceptance/);
  await page.keyboard.press("g");
  await page.keyboard.press("q");
  await expect(page).toHaveURL(/\/inbox\/qa/);
  await page.keyboard.press("?");
  await expect(page.getByRole("heading", { name: "キーボードショートカット" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { name: "キーボードショートカット" })).toHaveCount(0);
});
