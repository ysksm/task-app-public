// Drafts of a story map (docs/08-devcycle.md §5.9): make a draft from the map,
// brainstorm there, review the import plan and import in one go.
import { expect, test } from "../fixtures.js";

test("makes a draft, changes it, reviews the plan and imports it", async ({ page, demo }) => {
  const owner = demo.api("owner");
  await page.goto("/p/APP/storymap");
  await page.getByTestId("drafts").click();
  await page.getByTestId("create-draft").click();

  await expect(page).toHaveURL(/\/p\/APPD1\/storymap/);
  await expect(page.getByTestId("draft-banner")).toContainText("APP に取り込む前の試し場所");
  await expect(page.getByTestId("storymap")).toHaveAttribute("data-mode", "quick");
  await expect(page.getByTestId("story-APPD1-ST1")).toBeVisible(); // a copy with the same numbers
  await expect(page.getByTestId("drafts")).toHaveCount(0);

  // Brainstorm: one new story, one renamed slice.
  await page.getByTestId("quick-story").fill("下書きで足した");
  await page.keyboard.press("Enter");
  await page.getByTestId("release-name-R2").click();
  await page.getByTestId("release-name-R2-input").fill("つぎ");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("release-つぎ")).toBeVisible();
  // The target has not changed.
  expect((await owner.get("/api/v1/projects/APP/storymap")).body.releases).toContain("R2");

  // A draft story does not link issues.
  await page.getByTestId("story-APPD1-ST1").click();
  await expect(page.getByTestId("draft-no-issues")).toBeVisible();

  await page.getByTestId("open-import-plan").click();
  const plan = page.getByTestId("import-plan");
  await expect(plan.getByTestId("plan-create")).toContainText("下書きで足した");
  await expect(plan.getByTestId("plan-update")).toContainText("R2");
  await expect(plan.getByTestId("plan-update")).toContainText("つぎ");
  await plan.getByTestId("apply-import").click();
  await expect(plan.getByTestId("import-done")).toContainText("APP に 2 件を取り込みました");

  await plan.getByTestId("go-target").click();
  await expect(page).toHaveURL(/\/p\/APP\/storymap/);
  await expect(page.getByTestId("release-つぎ")).toBeVisible();
  await expect(page.getByTestId("storymap")).toContainText("下書きで足した");
  await page.getByTestId("drafts").click();
  await expect(page.getByTestId("draft-APPD1")).toBeVisible();
});

test("asks before overwriting a change made in the target since the copy", async ({ page, demo }) => {
  const owner = demo.api("owner");
  expect((await owner.post("/api/v1/projects/APP/drafts", {})).status).toBe(201);
  await owner.patch("/api/v1/projects/APP/stories/APP-ST1", { title: "本番で直した" });
  await owner.patch("/api/v1/projects/APPD1/stories/APPD1-ST1", { title: "下書きで直した" });

  await page.goto("/p/APPD1/storymap");
  await page.getByTestId("open-import-plan").click();
  const plan = page.getByTestId("import-plan");
  await expect(plan.getByTestId("plan-conflict")).toBeVisible();
  await expect(plan.getByTestId("apply-import")).toBeDisabled();
  await plan.getByTestId("force-import").check();
  await plan.getByTestId("apply-import").click();
  await expect(plan.getByTestId("import-done")).toBeVisible();
  expect((await owner.get("/api/v1/projects/APP/stories/APP-ST1")).body.title).toBe("下書きで直した");

  // Throwing the draft away leaves the target alone.
  await plan.getByRole("button", { name: "下書きに戻る" }).click();
  await page.getByTestId("discard-draft").click();
  await page.getByTestId("discard-draft-confirm").click();
  await expect(page).toHaveURL(/\/p\/APP\/storymap/);
  expect((await owner.get("/api/v1/projects/APP/drafts")).body.drafts).toEqual([]);
});
