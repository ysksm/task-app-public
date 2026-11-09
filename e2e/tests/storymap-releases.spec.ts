// Release slices on the story map (docs/08-devcycle.md §5.8): each row shows its
// name, goal, status and progress; the first slice not shipped is 次に出す.
import { expect, test } from "../fixtures.js";

test("shows the release slices with goal, status and progress, and edits them on the map", async ({ page, demo }) => {
  const owner = demo.api("owner");
  await page.goto("/p/APP/storymap");
  const r1 = page.getByTestId("release-R1");
  await expect(r1).toContainText("ログインして、ORCA から自分のタスクを読める");
  await expect(page.getByTestId("release-status-R1")).toHaveValue("building");
  await expect(page.getByTestId("release-progress-R1")).toHaveText("1/3 完了");
  await expect(r1.getByTestId("release-next")).toBeVisible();
  await expect(page.getByTestId("release-R2").getByTestId("release-next")).toHaveCount(0);

  // Shipping R1 moves 次に出す to R2.
  await page.getByTestId("release-status-R1").selectOption("shipped");
  await expect(r1).toHaveAttribute("data-status", "shipped");
  await expect(page.getByTestId("release-R2").getByTestId("release-next")).toBeVisible();

  // Write R2's goal on the map.
  await page.getByTestId("release-goal-R2").click();
  await page.getByTestId("release-goal-R2-input").fill("ひとりで立ち直れる");
  await page.getByTestId("release-name-R2").click(); // blur saves
  await expect.poll(async () => (await owner.get("/api/v1/projects/APP/releases")).body.releases[1].goal).toBe("ひとりで立ち直れる");

  // Add a slice; it is a real row, and it can be moved up.
  await page.getByTestId("add-release").click();
  await page.getByTestId("add-release-input").fill("R3");
  await page.keyboard.press("Enter");
  const r3 = page.getByTestId("release-R3");
  await expect(r3).toContainText("0/0 完了");
  await r3.hover();
  await r3.getByTitle("上へ").click();
  await expect.poll(async () => (await owner.get("/api/v1/projects/APP/storymap")).body.releases).toEqual(["R1", "R3", "R2"]);
  // An empty slice can be removed.
  await r3.hover();
  await r3.getByTitle(/リリースを消す/).click();
  await expect(r3).toHaveCount(0);
});

test("renaming a slice moves its stories with it", async ({ page, demo }) => {
  await page.goto("/p/APP/storymap");
  await page.getByTestId("release-name-R2").click();
  await page.getByTestId("release-name-R2-input").fill("つぎ");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("cell-APP-A2-2-つぎ").getByTestId("story-APP-ST5")).toBeVisible();
  expect((await demo.api("owner").get("/api/v1/projects/APP/stories")).body.stories.find((s: any) => s.key === "APP-ST5").release).toBe("つぎ");
});
