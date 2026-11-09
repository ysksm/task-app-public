// Board across projects: columns are state categories, rows are projects.
import { expect, test } from "../fixtures.js";

test("shows every project on one board and moves a card by category", async ({ page, demo }) => {
  const owner = demo.api("owner");
  expect((await owner.post("/api/v1/projects", { key: "OPS", name: "運用" })).status).toBe(201);
  const ops = (await owner.post("/api/v1/issues", { project_key: "OPS", type: "task", title: "バックアップを確かめる" })).body.key;
  const filter = encodeURIComponent(JSON.stringify({ and: [{ field: "project", op: "eq", value: "APP" }, { field: "state_category", op: "eq", value: "unstarted" }, { field: "type", op: "eq", value: "task" }] }));
  const todo = (await owner.get(`/api/v1/issues?filter=${filter}&limit=1`)).body.issues[0].key;

  await page.goto("/");
  await page.getByRole("link", { name: "全体ボード" }).click();
  const board = page.getByTestId("category-board");
  await expect(board.getByTestId("lane-APP")).toBeVisible();
  await expect(board.getByTestId("lane-OPS").getByText(ops)).toBeVisible();

  // Drag with real mouse moves (dnd-kit's pointer sensor).
  const card = board.getByTestId("cell-APP|unstarted").getByText(todo, { exact: true });
  const a = (await card.boundingBox())!;
  const b = (await board.getByTestId("cell-APP|started").boundingBox())!;
  await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width / 2, b.y + 20, { steps: 12 });
  await page.mouse.up();
  await expect(board.getByTestId("cell-APP|started").getByText(todo, { exact: true })).toBeVisible();
  // Each project shows its own count per category (split by its states when several).
  await expect(board.getByTestId("cell-OPS|unstarted").getByTestId("cell-count")).toHaveText("1");
  await expect(board.getByTestId("cell-OPS|started").getByTestId("cell-count")).toHaveText("0");
  await expect.poll(async () => (await owner.get(`/api/v1/issues/${todo}`)).body.state.category).toBe("started");

  // Without lanes the columns hold every project.
  await page.getByRole("button", { name: "プロジェクトで分ける" }).click();
  await expect(page).toHaveURL(/lanes=none/);
  await expect(board.getByTestId("lane-APP")).toHaveCount(0);
  await expect(board.getByTestId("cell-*|unstarted").getByText(ops)).toBeVisible();
});

test("G then B opens the board", async ({ page }) => {
  await page.goto("/my");
  await page.keyboard.press("g");
  await page.keyboard.press("b");
  await expect(page).toHaveURL(/\/board/);
  await expect(page.getByTestId("category-board")).toBeVisible();
});

test("hiding closed issues removes the completed and canceled columns", async ({ page }) => {
  await page.goto("/board");
  const board = page.getByTestId("category-board");
  await expect(board.getByTestId("col-completed")).toBeVisible();
  await page.getByLabel("完了を隠す").check();
  await expect(board.getByTestId("col-completed")).toHaveCount(0);
  await expect(board.getByTestId("col-canceled")).toHaveCount(0);
  await expect(board.getByTestId("col-started")).toBeVisible();
  // The project board does the same.
  await page.goto("/p/APP/issues?view=board");
  await expect(page.getByTestId("state-col-Done")).toBeVisible();
  await page.getByLabel("完了を隠す").check();
  await expect(page.getByTestId("state-col-Done")).toHaveCount(0);
  await expect(page.getByTestId("state-col-In Progress")).toBeVisible();
});

test("groups lanes by project tier: その他 starts closed, 注力のみ narrows to focus projects", async ({ page, demo }) => {
  const owner = demo.api("owner");
  expect((await owner.post("/api/v1/projects", { key: "FOC", name: "注力の案件", tier: "focus" })).status).toBe(201);
  expect((await owner.post("/api/v1/projects", { key: "TRY", name: "お試し", tier: "side" })).status).toBe(201);
  const foc = (await owner.post("/api/v1/issues", { project_key: "FOC", type: "task", title: "大事な仕事" })).body.key;
  const tryKey = (await owner.post("/api/v1/issues", { project_key: "TRY", type: "task", title: "ちょっと試す" })).body.key;

  await page.goto("/board");
  const board = page.getByTestId("category-board");
  // 注力 → 通常 → その他
  const sections = board.locator("[data-testid^=tier-section-]");
  await expect(sections.first()).toHaveAttribute("data-testid", "tier-section-focus");
  await expect(sections.last()).toHaveAttribute("data-testid", "tier-section-side");
  await expect(board.getByTestId("tier-section-focus").getByTestId("lane-FOC").getByText(foc)).toBeVisible();
  // the その他 lane is closed (counts only) until opened
  const side = board.getByTestId("lane-TRY");
  await expect(side).toHaveAttribute("data-closed", "true");
  await expect(side.getByText(tryKey)).toHaveCount(0);
  await side.getByRole("button", { name: /お試し/ }).click();
  await expect(side.getByText(tryKey)).toBeVisible();

  // 注力のみ
  await page.getByTestId("focus-only").click();
  await expect(board.getByTestId("lane-FOC")).toBeVisible();
  await expect(board.getByTestId("lane-APP")).toHaveCount(0);
  await expect(board.getByTestId("lane-TRY")).toHaveCount(0);
  await page.getByTestId("focus-only").click();

  // Change a tier from the lane header: APP becomes その他.
  await board.getByTestId("tier-APP").click();
  await page.getByRole("button", { name: /^その他/ }).click();
  await expect(board.getByTestId("tier-section-side").getByTestId("lane-APP")).toBeVisible();
  await expect.poll(async () => (await owner.get("/api/v1/projects/APP")).body.tier).toBe("side");
});
