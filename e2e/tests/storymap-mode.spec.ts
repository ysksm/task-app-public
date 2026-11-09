// Option E (docs/09-requirements-loose.md §6): the project setting 要求の書き方
// picks the story map screen. まず書く (quick) is the option D screen,
// 整理して書く (structured) the option A screen. The data stays the same.
import { expect, test } from "../fixtures.js";

test("switches the story map between まず書く and 整理して書く from project settings", async ({ page, demo }) => {
  // New projects (and the demo) start in quick mode.
  await page.goto("/p/APP/storymap");
  await expect(page.getByTestId("storymap")).toHaveAttribute("data-mode", "quick");
  await expect(page.getByTestId("quick-story")).toBeVisible();
  await expect(page.getByTestId("unsorted-header")).toBeVisible();
  await expect(page.getByTestId("mode-badge")).toHaveText("まず書く");

  // The badge leads to the setting.
  await page.getByTestId("mode-badge").click();
  await expect(page).toHaveURL(/\/p\/APP\/settings/);
  await expect(page.getByTestId("requirements-mode-quick")).toHaveAttribute("aria-pressed", "true");
  await page.getByTestId("requirements-mode-structured").click();
  await expect(page.getByTestId("requirements-mode-structured")).toHaveAttribute("aria-pressed", "true");
  await expect.poll(async () => (await demo.api("owner").get("/api/v1/projects/APP")).body.requirements_mode).toBe("structured");

  // Structured: no quick input, no 未整理 column; unsorted stories are a gap shown above the map.
  await page.goto("/p/APP/storymap");
  const map = page.getByTestId("storymap");
  await expect(map).toHaveAttribute("data-mode", "structured");
  await expect(page.getByTestId("quick-story")).toHaveCount(0);
  await expect(page.getByTestId("unsorted-header")).toHaveCount(0);
  await expect(page.getByTestId("unplaced")).toContainText("未整理 2件");
  await expect(page.getByTestId("unplaced").getByTestId("story-APP-ST6")).toBeVisible();
  await expect(page.getByTestId("gaps")).toHaveAttribute("data-warn", "true");
  await expect(page.getByTestId("persona-filter")).toBeVisible();
  // As in option A, a story can still be added without a step from here.
  await page.getByTestId("add-unplaced-story").click();
  await page.getByTestId("add-unplaced-story-input").fill("置き場所はあとで");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("unplaced")).toContainText("未整理 3件");
  await page.getByTestId("story-peek").getByRole("button", { name: "閉じる" }).click();

  // The unsorted story keeps its body; the detail asks for a step.
  await page.getByTestId("unplaced").getByTestId("story-APP-ST6").click();
  const detail = page.getByTestId("story-peek");
  await expect(detail.getByTestId("story-body")).toBeVisible();
  await expect(detail.getByTestId("story-no-step")).toBeVisible();
  await expect(detail.getByTestId("story-more-fields")).toHaveCount(0);
  await expect(detail.getByTestId("story-i-want")).toBeVisible();

  // Back to quick: the same story is in 未整理 again.
  await demo.api("owner").patch("/api/v1/projects/APP", { requirements_mode: "quick" });
  await page.reload();
  await expect(page.getByTestId("cell-unsorted-unplanned").getByTestId("story-APP-ST6")).toBeVisible();
});

test("structured: a story is added under a step with Given / When / Then", async ({ page, demo }) => {
  const owner = demo.api("owner");
  await owner.patch("/api/v1/projects/APP", { requirements_mode: "structured" });
  await page.goto("/p/APP/storymap");
  const cell = page.getByTestId("cell-APP-A1-2-R1");
  await cell.hover();
  await page.getByTestId("add-story-APP-A1-2-R1").click();
  await page.getByTestId("add-story-APP-A1-2-R1-input").fill("ロックされても自分で解除できる");
  await page.keyboard.press("Enter");

  const detail = page.getByTestId("story-peek");
  await expect(detail).toHaveAttribute("data-story-key", "APP-ST8");
  // The template fields are there without 項目を足す, and the criterion has three fields without 詳しく.
  await detail.getByTestId("story-i-want").click();
  await detail.getByTestId("story-i-want-input").fill("ロックされても待たずに使いたい");
  await detail.getByTestId("story-title").click();
  await detail.getByTestId("story-title-input").press("Escape");
  await expect(detail.getByTestId("new-criterion-detail")).toHaveCount(0);
  await detail.getByTestId("new-criterion-given").fill("ロックされたユーザー");
  await detail.getByTestId("new-criterion-when").fill("メールのリンクを開く");
  await detail.getByTestId("new-criterion-then").fill("すぐにログインできる");
  await detail.getByTestId("add-criterion").click();
  await expect(cell.getByTestId("story-APP-ST8")).toHaveAttribute("data-status", "ready");

  const st = (await owner.get("/api/v1/projects/APP/stories/APP-ST8")).body;
  expect(st).toMatchObject({ i_want: "ロックされても待たずに使いたい", step_keys: ["APP-A1-2"], release: "R1" });
  expect(st.criteria).toEqual([expect.objectContaining({ given: "ロックされたユーザー", when: "メールのリンクを開く", then: "すぐにログインできる" })]);

  // A one-line criterion written in quick mode shows as three fields here (前提・操作 empty).
  const one = (await owner.post("/api/v1/projects/APP/stories/APP-ST8/criteria", { then: "結果だけの条件" })).body;
  await expect(detail.getByTestId(`criterion-${one.key}`)).toContainText("前提");
});

test("a new project needs an explicit choice of 要求の書き方", async ({ page, demo }) => {
  await page.goto("/projects");
  await page.getByRole("button", { name: "新規プロジェクト" }).click();
  await page.getByPlaceholder("KEY").fill("NEW");
  await page.getByPlaceholder("プロジェクト名").fill("新しいもの");
  // Nothing is preselected, so 作成 waits for the choice.
  await expect(page.getByTestId("new-project-mode")).toHaveValue("");
  await expect(page.getByTestId("create-project")).toBeDisabled();
  await page.getByTestId("new-project-mode").selectOption("structured");
  await page.getByTestId("create-project").click();
  await expect(page).toHaveURL(/\/p\/NEW\/issues/);
  expect((await demo.api("owner").get("/api/v1/projects/NEW")).body.requirements_mode).toBe("structured");
});
