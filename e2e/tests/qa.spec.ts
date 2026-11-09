// QA inbox: a human answers an agent's question with choices (QA-01..05).
import { expect, test } from "../fixtures.js";

test("answers a question with choices and it leaves the open list", async ({ page, demo }) => {
  await page.goto("/inbox/qa?key=APP-8");
  const main = page.getByRole("main");
  await expect(main.getByText("パスワード規則はどれにしますか？")).toBeVisible();

  await main.getByRole("button", { name: /12文字以上 \+ 記号必須/ }).click();
  await main.getByRole("button", { name: /^OK/ }).click();
  await main.getByRole("button", { name: /ログイン状態を保持する/ }).click();
  await main.getByRole("button", { name: /パスワードを忘れた方へ/ }).click();
  await main.getByRole("button", { name: "回答する" }).click();

  await expect(main.getByRole("button", { name: "回答を修正" })).toBeVisible();
  const qa = (await demo.api("owner").get("/api/v1/qa/APP-8")).body;
  expect(qa.status).toBe("answered");
  const labels = (i: number) => qa.items[i].answer.selected_options.map((o: { label: string }) => o.label);
  expect(labels(0)).toEqual(["12文字以上 + 記号必須"]);
  expect(labels(1)).toEqual(["OK"]);
  expect(labels(2)).toEqual(["ログイン状態を保持する", "パスワードを忘れた方へ"]);

  await page.goto("/inbox/qa");
  await expect(page.getByRole("main").getByText(/ORDER BY の複数キー対応/).first()).toBeVisible();
  await expect(page.getByRole("main").getByText(/ログイン画面の仕様確認/)).toHaveCount(0);

  // The agent reads the answers through the API.
  const answers = await demo.api("claude").get("/api/v1/qa/APP-8");
  expect(answers.body.items.every((i: { answer: unknown }) => i.answer)).toBe(true);
});

test("shows a partially answered question as still missing required items", async ({ page }) => {
  await page.goto("/inbox/qa?key=APP-8");
  const main = page.getByRole("main");
  await main.getByRole("button", { name: /8文字以上 \+ 英数字混在/ }).click();
  await expect(main.getByText("必須の質問があと 2 件")).toBeVisible();
  await expect(main.getByRole("button", { name: "途中まで保存" })).toBeEnabled();
});
