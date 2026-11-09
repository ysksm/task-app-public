// Saved views (VIEW-03): save the current filter and open it from the sidebar.
import { expect, test } from "../fixtures.js";

test("saves the current filter as a view and opens it from the sidebar", async ({ page, demo }) => {
  await page.goto("/p/APP/issues");
  await page.keyboard.press("/");
  await page.keyboard.type("ログイン");
  await expect(page.getByText("README を整備する")).toHaveCount(0);
  await expect(page.getByText("ログイン画面を実装する")).toBeVisible();

  await page.getByRole("button", { name: "ビューを保存" }).click();
  await page.getByPlaceholder("ビューの名前").fill("ログイン関連");
  await page.getByRole("button", { name: "保存", exact: true }).click();

  const link = page.getByRole("navigation").getByRole("link", { name: /ログイン関連/ });
  await expect(link).toBeVisible();
  await page.goto("/my");
  await link.click();
  await expect(page).toHaveURL(/\/views\//);
  await expect(page.getByText("ログイン画面を実装する")).toBeVisible();
  await expect(page.getByText("README を整備する")).toHaveCount(0);
  const views = (await demo.api("owner").get("/api/v1/views")).body;
  expect(views.views ?? views).toEqual(expect.arrayContaining([expect.objectContaining({ name: "ログイン関連", project_key: "APP" })]));
});
