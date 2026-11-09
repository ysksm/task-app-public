// S15 story map and S16 story detail (docs/08-devcycle.md §5.5). The demo data
// has two personas, two activities with steps and five stories in every status.
import { expect, peek, test } from "../fixtures.js";

test("shows the map, coloured by the linked issues, and reaches it from the issue list", async ({ page }) => {
  await page.goto("/p/APP/issues");
  await page.getByTestId("open-storymap").click();
  await expect(page).toHaveURL(/\/p\/APP\/storymap/);
  const map = page.getByTestId("storymap");
  await expect(map.getByTestId("activity-APP-A1")).toContainText("ログインする");
  await expect(map.getByTestId("step-APP-A1-2")).toContainText("失敗から立ち直る");
  // Rows are releases; cards sit under their step.
  await expect(map.getByTestId("cell-APP-A1-1-R1").getByTestId("story-APP-ST1")).toBeVisible();
  await expect(map.getByTestId("cell-APP-A2-2-R2").getByTestId("story-APP-ST5")).toBeVisible();
  await expect(map.getByTestId("story-APP-ST1")).toHaveAttribute("data-status", "in_progress");
  await expect(map.getByTestId("story-APP-ST2")).toHaveAttribute("data-status", "done");
  await expect(map.getByTestId("story-APP-ST4")).toHaveAttribute("data-status", "ready");
  await expect(map.getByTestId("story-APP-ST5")).toHaveAttribute("data-status", "draft");

  // Missing links are listed.
  await page.getByTestId("gaps").click();
  await expect(page.getByTestId("gaps-panel")).toContainText("受け入れ条件のないストーリー");
  await expect(page.getByTestId("gaps-panel")).toContainText("APP-ST5");
});

test("adds a story under a step, writes criteria and links an issue", async ({ page, demo }) => {
  const owner = demo.api("owner");
  const task = (await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "ロック解除の手順を画面に出す" })).body.key;

  await page.goto("/p/APP/storymap");
  const cell = page.getByTestId("cell-APP-A1-2-R1");
  await cell.hover();
  await page.getByTestId("add-story-APP-A1-2-R1").click();
  await page.getByTestId("add-story-APP-A1-2-R1-input").fill("ロックされても自分で解除できる");
  await page.keyboard.press("Enter");

  // The new story opens on the right.
  const detail = page.getByTestId("story-peek");
  await expect(detail).toHaveAttribute("data-story-key", "APP-ST8");
  await expect(cell.getByTestId("story-APP-ST8")).toHaveAttribute("data-status", "draft");

  await detail.getByTestId("story-more-fields").click();
  await detail.getByTestId("story-i-want").click();
  await detail.getByTestId("story-i-want-input").fill("ロックされても待たずに使いたい");
  await detail.getByTestId("story-title").click(); // blur saves
  await detail.getByTestId("story-title-input").press("Escape");

  await detail.getByTestId("new-criterion-detail").click();
  await detail.getByTestId("new-criterion-given").fill("ロックされたユーザー");
  await detail.getByTestId("new-criterion-when").fill("メールのリンクを開く");
  await detail.getByTestId("new-criterion-then").fill("すぐにログインできる");
  await detail.getByTestId("add-criterion").click();
  await expect(detail.getByTestId("criteria")).toContainText("すぐにログインできる");
  await expect(cell.getByTestId("story-APP-ST8")).toHaveAttribute("data-status", "ready");

  await detail.getByTestId("link-issue-input").fill(task.split("-")[1]); // the number alone is enough
  await detail.getByRole("button", { name: "つなぐ" }).click();
  await expect(detail.getByTestId("story-issues")).toContainText(task);
  await expect(detail.getByTestId("story-history")).toContainText(`イシューをつないだ: ${task}`);

  const st = (await owner.get("/api/v1/projects/APP/stories/APP-ST8")).body;
  expect(st).toMatchObject({ i_want: "ロックされても待たずに使いたい", step_keys: ["APP-A1-2"], release: "R1" });
  expect(st.criteria).toEqual([expect.objectContaining({ given: "ロックされたユーザー", when: "メールのリンクを開く", then: "すぐにログインできる" })]);

  // Starting the task colours the card.
  const states = (await owner.get("/api/v1/projects/APP/states")).body.states;
  await owner.patch(`/api/v1/issues/${task}`, { state_id: states.find((s: any) => s.category === "started" && s.applies_to === "all").id });
  await expect(cell.getByTestId("story-APP-ST8")).toHaveAttribute("data-status", "in_progress");

  // The issue tells why it exists.
  await detail.getByTestId("story-issues").getByText(task).click();
  await expect(page).toHaveURL(new RegExp(`/p/APP/issues\\?peek=${task}`));
  await expect(peek(page).getByTestId("issue-stories")).toContainText("ロックされても自分で解除できる");
  await expect(peek(page).getByTestId("issue-stories")).toContainText("【前提】ロックされたユーザー 【操作】メールのリンクを開く 【結果】すぐにログインできる");
});

test("edits personas, jobs, activities and steps", async ({ page, demo }) => {
  await page.goto("/p/APP/storymap");
  await page.getByTestId("toggle-panel").click();
  const panel = page.getByTestId("persona-panel");
  await panel.getByTestId("new-persona-name").fill("たまに見る人");
  await panel.getByTestId("add-persona").click();
  await expect(panel.getByTestId("persona-APP-P3")).toContainText("たまに見る人");
  await expect(page.getByTestId("persona-filter")).toContainText("たまに見る人");

  await panel.getByTestId("new-job-motivation").fill("週末に進み具合をまとめて見たい");
  await panel.getByTestId("add-job").click();
  const job = panel.getByTestId("job-APP-J2");
  await expect(job).toContainText("週末に進み具合をまとめて見たい");
  await job.getByRole("button", { name: "ORCA から使う" }).click();
  await expect.poll(async () => (await demo.api("owner").get("/api/v1/projects/APP/jobs")).body.jobs[1].activity_keys).toEqual(["APP-A2"]);

  await page.getByTestId("add-activity").click();
  await page.getByTestId("add-activity-input").fill("振り返る");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("activity-APP-A3")).toContainText("振り返る");
  await page.getByTestId("add-step-APP-A3").click();
  await page.getByTestId("add-step-APP-A3-input").fill("終わったものを見る");
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("step-APP-A3-1")).toContainText("終わったものを見る");

  // Showing one persona hides the activities of others.
  await page.getByTestId("persona-filter").getByRole("button", { name: "オーナー" }).click();
  await expect(page).toHaveURL(/persona=APP-P1/);
  await expect(page.getByTestId("activity-APP-A1")).toBeVisible();
});

test("copies a story's criteria into the issue's acceptance checklist", async ({ page, demo }) => {
  const owner = demo.api("owner");
  // APP-3 (feature) is linked to APP-ST1, which has two criteria.
  await page.goto("/p/APP/issues?peek=APP-3");
  await expect(peek(page).getByTestId("issue-stories")).toContainText("APP-ST1");
  await peek(page).getByTestId("copy-story-criteria").click();
  await expect.poll(async () => (await owner.get("/api/v1/issues/APP-3/acceptance")).body.criteria.map((c: any) => c.description)).toEqual([
    "【前提】登録済みのユーザー 【操作】正しいメールアドレスとパスワードを入れる 【結果】マイタスクが開く",
    "【前提】ログイン画面 【操作】誤ったパスワードを入れる 【結果】日本語のエラーが表示される",
  ]);
  // Already copied: the button goes away.
  await expect(peek(page).getByTestId("copy-story-criteria")).toHaveCount(0);
});
