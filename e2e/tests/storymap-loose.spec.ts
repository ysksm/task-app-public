// Option D (docs/09-requirements-loose.md §3): the story map used loosely. One line
// goes to 未整理, criteria are one line with 詳しく for the rest, and a project
// without activities shows only the release × 未整理 grid.
import { expect, peek, test } from "../fixtures.js";

test("quick input puts a story into 未整理; placing it later moves it into a step column", async ({ page, demo }) => {
  await page.goto("/p/APP/storymap");
  // The demo has two stories written this way already.
  const unsorted = page.getByTestId("cell-unsorted-unplanned");
  await expect(page.getByTestId("unsorted-header")).toContainText("未整理2");
  await expect(unsorted.getByTestId("story-APP-ST6")).toContainText("朝の画面で、AI に任せたものだけを見たい");
  await expect(unsorted.getByTestId("story-APP-ST6").getByTestId("story-body-hint")).toBeVisible();

  await page.getByTestId("quick-story").fill("AI の質問をまとめて返したい");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("quick-story")).toHaveValue("");
  await expect(unsorted.getByTestId("story-APP-ST8")).toHaveAttribute("data-status", "draft");
  expect((await demo.api("owner").get("/api/v1/projects/APP/stories/APP-ST8")).body).toMatchObject({ persona_key: null, release: "", step_keys: [] });

  // Write the body, then place it from the detail.
  await unsorted.getByTestId("story-APP-ST8").click();
  const detail = page.getByTestId("story-peek");
  await expect(detail.getByTestId("story-steps")).toContainText("未整理");
  await detail.getByTestId("story-body").click();
  await detail.getByTestId("story-body-input").fill("**質問が溜まる**と答えにくい");
  await detail.getByTestId("story-title").click();
  await expect(detail.getByTestId("story-body").locator("strong")).toHaveText("質問が溜まる");
  await detail.getByTestId("story-add-step").selectOption("APP-A2-1");
  await detail.getByTestId("story-release").click();
  await detail.getByTestId("story-release-input").fill("R2");
  await detail.getByTestId("story-release-input").press("Enter");
  await expect(page.getByTestId("cell-APP-A2-1-R2").getByTestId("story-APP-ST8")).toBeVisible();
  await expect(page.getByTestId("unsorted-header")).toContainText("未整理2");
  await expect(detail.getByTestId("story-history")).toContainText("本文");
});

test("a criterion is one line; 詳しく opens the precondition and the action", async ({ page, demo }) => {
  await page.goto("/p/APP/storymap?peek=APP-ST7");
  const detail = page.getByTestId("story-peek");
  // Only the title and body are shown at first; the persona row stays because the demo has personas.
  await expect(detail.getByTestId("story-i-want")).toHaveCount(0);
  await expect(detail.getByTestId("new-criterion-given")).toHaveCount(0);

  await detail.getByTestId("new-criterion-then").fill("3 日動いていないものが上に出る");
  await detail.getByTestId("add-criterion").click();
  await expect(detail.getByTestId("criteria")).toContainText("3 日動いていないものが上に出る");
  await expect(page.getByTestId("story-APP-ST7")).toHaveAttribute("data-status", "ready");

  await detail.getByTestId("new-criterion-detail").click();
  await detail.getByTestId("new-criterion-given").fill("金曜の夕方");
  await detail.getByTestId("new-criterion-then").fill("プロジェクトごとの件数が出る");
  await detail.getByTestId("add-criterion").click();
  await expect(detail.getByTestId("criteria")).toContainText("前提");
  const crit = (await demo.api("owner").get("/api/v1/projects/APP/stories/APP-ST7")).body.criteria;
  expect(crit.map((c: any) => [c.given, c.when, c.then])).toEqual([["", "", "3 日動いていないものが上に出る"], ["金曜の夕方", "", "プロジェクトごとの件数が出る"]]);

  // The issue shows the one-line criterion as it is, and copies it as it is.
  const owner = demo.api("owner");
  const task = (await owner.post("/api/v1/issues", { project_key: "APP", type: "feature", title: "滞りの一覧" })).body.key;
  await owner.put(`/api/v1/projects/APP/stories/APP-ST7/issues/${task}`);
  await page.goto(`/p/APP/issues?peek=${task}`);
  await expect(peek(page).getByTestId("issue-stories")).toContainText("3 日動いていないものが上に出る");
  await expect(peek(page).getByTestId("issue-story-body")).toContainText("金曜の夕方に");
  await peek(page).getByTestId("copy-story-criteria").click();
  await expect.poll(async () => (await owner.get(`/api/v1/issues/${task}/acceptance`)).body.criteria.map((c: any) => c.description)).toEqual([
    "3 日動いていないものが上に出る",
    "【前提】金曜の夕方 【結果】プロジェクトごとの件数が出る",
  ]);
});

test("a project without activities shows only release × 未整理, no persona filter, gaps as a plain count", async ({ page, demo }) => {
  await demo.api("owner").post("/api/v1/projects", { key: "NEW", name: "New" });
  await page.goto("/p/NEW/storymap");
  await expect(page.getByTestId("persona-filter")).toHaveCount(0);
  await expect(page.getByTestId("persona-panel")).toHaveCount(0);
  await expect(page.getByTestId("no-activities-hint")).toBeVisible();
  await page.getByTestId("quick-story").fill("まずは書いておく");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("cell-unsorted-unplanned").getByTestId("story-NEW-ST1")).toBeVisible();
  await expect(page.locator('[data-testid^="activity-NEW-"]')).toHaveCount(0);
  await expect(page.getByTestId("gaps")).toContainText("抜け 3"); // no step, no criterion, no issue
  await expect(page.getByTestId("gaps")).not.toHaveClass(/e0823d/);
  // The detail has no persona or step choices to make yet.
  await page.getByTestId("story-NEW-ST1").click();
  await expect(page.getByTestId("story-peek").getByTestId("story-persona")).toHaveCount(0);
});
