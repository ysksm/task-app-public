// Acceptance inspection (ACC-01..06), including the checkpoint-4 fix:
// the issues behind the readiness warnings are listed and open in a peek.
import { expect, peek, test } from "../fixtures.js";

test("lists the unfinished issues behind the warnings and opens them", async ({ page }) => {
  await page.goto("/inbox/acceptance?key=APP-2");
  const main = page.getByRole("main");
  await expect(main.getByText("未完了の子イシューが 2 件あります")).toBeVisible();
  await expect(main.getByText("未回答の QA が 1 件あります")).toBeVisible();
  await main.getByRole("button", { name: /セッションCookieの設定/ }).click();
  await expect(peek(page)).toHaveAttribute("data-issue-key", "APP-4");
  await expect(page).toHaveURL(/peek=APP-4/);
});

test("accepts a story after judging every criterion", async ({ page, demo }) => {
  await page.goto("/inbox/acceptance?key=APP-2");
  const main = page.getByRole("main");
  const accept = main.getByRole("button", { name: /合格（→ 完了）/ });
  await expect(main.getByText("未判定 3 件")).toBeVisible();
  await expect(accept).toBeDisabled();
  for (const b of await main.getByRole("button", { name: "OK", exact: true }).all()) await b.click();
  await main.getByPlaceholder(/総評/).fill("E2E: 基準をすべて満たした");
  await accept.click();
  // Unfinished children: the server asks for confirmation first.
  await main.getByRole("button", { name: "警告を承知で合格にする" }).click();

  await expect.poll(async () => (await demo.api("owner").get("/api/v1/issues/APP-2")).body.state.category).toBe("completed");
  const acc = (await demo.api("owner").get("/api/v1/issues/APP-2/acceptance")).body;
  expect(acc.reviews.at(-1)).toMatchObject({ verdict: "accepted", summary: "E2E: 基準をすべて満たした" });
});

test("rejecting needs a reason and sends the story back to work", async ({ page, demo }) => {
  await page.goto("/inbox/acceptance?key=APP-2");
  const main = page.getByRole("main");
  const reject = main.getByRole("button", { name: /差し戻し（→ 作業中）/ });
  await expect(main.getByText("未判定 3 件")).toBeVisible();
  const oks = await main.getByRole("button", { name: "OK", exact: true }).all();
  await oks[0].click();
  await oks[2].click();
  await main.getByRole("button", { name: "NG", exact: true }).nth(1).click();
  await main.getByPlaceholder("NG の理由").fill("ロックされない");
  await expect(main.getByRole("button", { name: /合格（→ 完了）/ })).toBeDisabled();
  await expect(reject).toBeDisabled();
  await main.getByPlaceholder(/総評/).fill("ロックの実装を確認してください");
  await reject.click();

  await expect.poll(async () => (await demo.api("owner").get("/api/v1/issues/APP-2")).body.state.is_acceptance).toBe(false);
  const acc = (await demo.api("claude").get("/api/v1/issues/APP-2/acceptance")).body;
  expect(acc.reviews.at(-1).verdict).toBe("rejected");
});
