import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { startServer, type Server } from "../src/harness.js";

let srv: Server;
let states: Record<string, number>;

beforeAll(async () => {
  srv = await startServer();
  await srv.as("owner").post("/api/v1/projects", { key: "APP", name: "App" });
  await srv.as("owner").post("/api/v1/projects", { key: "OTH", name: "Other" });
  const list = (await srv.as("owner").get("/api/v1/projects/APP/states")).body.states;
  states = Object.fromEntries(list.map((s: any) => [s.name, s.id]));
});
afterAll(() => srv?.stop());

const create = (body: Record<string, unknown>, who: "owner" | "claude" = "owner") =>
  srv.as(who).post("/api/v1/issues", { project_key: "APP", ...body });

describe("creating issues", () => {
  it("numbers keys per project and fills defaults", async () => {
    const a = await create({ type: "epic", title: "Epic A" });
    expect(a.status).toBe(201);
    expect(a.body).toMatchObject({
      key: "APP-1", number: 1, project_key: "APP", type: "epic", title: "Epic A", description: "",
      priority: 0, assignee: null, parent_key: null, labels: [], child_count: 0, open_qa_count: 0,
      progress: { total: 0, done: 0 },
    });
    expect(a.body.state).toMatchObject({ name: "Todo", category: "unstarted" });
    expect(a.body.reporter.username).toBe("owner");
    const other = await srv.as("owner").post("/api/v1/issues", { project_key: "OTH", type: "task", title: "x" });
    expect(other.body.key).toBe("OTH-1");
    expect((await create({ type: "task", title: "t" })).body.key).toBe("APP-2");
  });

  it("validates input", async () => {
    expect((await create({ type: "bug", title: "x" })).status).toBe(400);
    expect((await create({ type: "task", title: "  " })).status).toBe(400);
    expect((await create({ type: "task", title: "x", priority: 9 })).status).toBe(400);
    expect((await create({ type: "task", title: "x", due_date: "11/10" })).status).toBe(400);
    expect((await create({ type: "task", title: "x", assignee: "nobody" })).status).toBe(400);
    expect((await srv.as("owner").post("/api/v1/issues", { type: "task", title: "x" })).status).toBe(400);
    expect((await srv.as("owner").post("/api/v1/issues", { project_key: "NOPE", type: "task", title: "x" })).status).toBe(404);
  });

  it("sets assignee (including 'me'), labels, estimate and due date", async () => {
    const res = await create({ type: "task", title: "x", assignee: "me", labels: ["b", "a", "a"], estimate: 3, due_date: "2026-11-01", priority: 2 }, "claude");
    expect(res.body.assignee.username).toBe("claude");
    expect(res.body.reporter).toMatchObject({ username: "claude", kind: "agent" });
    expect(res.body.labels.map((l: any) => l.name)).toEqual(["a", "b"]);
    expect(res.body).toMatchObject({ estimate: 3, due_date: "2026-11-01", priority: 2 });
  });

  it("uses QA states for QA issues and rejects mismatched states", async () => {
    const qa = await create({ type: "qa", title: "Which rule?" });
    expect(qa.body.state.name).toBe("未回答");
    expect((await create({ type: "qa", title: "q", state_id: states["Todo"] })).status).toBe(400);
    expect((await create({ type: "task", title: "t", state_id: states["未回答"] })).status).toBe(400);
  });

  it("records started/completed timestamps from the state category", async () => {
    const started = await create({ type: "task", title: "s", state_id: states["In Progress"] });
    expect(started.body.started_at).not.toBeNull();
    expect(started.body.completed_at).toBeNull();
    const done = await create({ type: "task", title: "d", state_id: states["Done"] });
    expect(done.body.completed_at).not.toBeNull();
  });
});

describe("hierarchy rules", () => {
  let epic: string, story: string, feature: string, task: string, qa: string;
  beforeAll(async () => {
    epic = (await create({ type: "epic", title: "E" })).body.key;
    story = (await create({ type: "story", title: "S", parent_key: epic })).body.key;
    feature = (await create({ type: "feature", title: "F", parent_key: story })).body.key;
    task = (await create({ type: "task", title: "T", parent_key: feature })).body.key;
    qa = (await create({ type: "qa", title: "Q", parent_key: task })).body.key;
  });

  it("allows the standard chain and tasks under tasks", async () => {
    const sub = await create({ type: "task", title: "sub", parent_key: task });
    expect(sub.status).toBe(201);
    const subsub = await create({ type: "task", title: "subsub", parent_key: sub.body.key });
    expect(subsub.status).toBe(201);
    expect(subsub.body.parent_key).toBe(sub.body.key);
  });

  it("allows skipping levels by default", async () => {
    expect((await create({ type: "task", title: "t", parent_key: epic })).status).toBe(201);
    expect((await create({ type: "feature", title: "f", parent_key: epic })).status).toBe(201);
    expect((await create({ type: "task", title: "t", parent_key: story })).status).toBe(201);
  });

  it("derives the project from the parent", async () => {
    const res = await srv.as("owner").post("/api/v1/issues", { type: "task", title: "t", parent_key: feature });
    expect(res.body.project_key).toBe("APP");
  });

  it.each([
    ["epic", "epic"], ["story", "story"], ["story", "feature"], ["story", "task"],
    ["feature", "feature"], ["feature", "task"], ["task", "qa"], ["qa", "qa"],
  ])("rejects a %s under a %s", async (child, parentType) => {
    const parent = { epic, story, feature, task, qa }[parentType]!;
    const res = await create({ type: child, title: "x", parent_key: parent });
    expect(res.status).toBe(422);
    expect(res.body.error.code).toBe("hierarchy_violation");
  });

  it("rejects parents from another project", async () => {
    const res = await srv.as("owner").post("/api/v1/issues", { project_key: "OTH", type: "task", title: "x", parent_key: task });
    expect(res.status).toBe(422);
  });

  it("enforces the no-skip setting", async () => {
    await srv.as("owner").post("/api/v1/projects", { key: "STRICT", name: "Strict", allow_skip_levels: false });
    const e = (await srv.as("owner").post("/api/v1/issues", { project_key: "STRICT", type: "epic", title: "e" })).body.key;
    const res = await srv.as("owner").post("/api/v1/issues", { type: "task", title: "t", parent_key: e });
    expect(res.status).toBe(422);
    expect(res.body.error.message).toMatch(/skipping levels is disabled/);
  });

  it("moves issues, re-validating rules and preventing cycles", async () => {
    const t2 = (await create({ type: "task", title: "movable" })).body.key;
    const moved = await srv.as("owner").patch(`/api/v1/issues/${t2}`, { parent_key: feature });
    expect(moved.status).toBe(200);
    expect(moved.body.parent_key).toBe(feature);
    expect(moved.body.ancestors.map((a: any) => a.key)).toEqual([epic, story, feature]);

    expect((await srv.as("owner").patch(`/api/v1/issues/${task}`, { parent_key: task })).status).toBe(422);
    const child = (await create({ type: "task", title: "c", parent_key: t2 })).body.key;
    const cycle = await srv.as("owner").patch(`/api/v1/issues/${t2}`, { parent_key: child });
    expect(cycle.status).toBe(422);
    expect(cycle.body.error.code).toBe("hierarchy_violation");

    expect((await srv.as("owner").patch(`/api/v1/issues/${story}`, { parent_key: feature })).status).toBe(422);
    const root = await srv.as("owner").patch(`/api/v1/issues/${t2}`, { parent_key: null });
    expect(root.body.parent_key).toBeNull();
  });

  it("re-validates children when the type changes", async () => {
    // An epic must be a root (feature has a parent).
    expect((await srv.as("owner").patch(`/api/v1/issues/${feature}`, { type: "epic" })).status).toBe(422);
    // The story has feature children, which cannot sit under a task.
    const res = await srv.as("owner").patch(`/api/v1/issues/${story}`, { type: "task" });
    expect(res.status).toBe(422);
    expect(res.body.error.message).toMatch(/existing children/);
    expect((await srv.as("owner").patch(`/api/v1/issues/${task}`, { type: "qa" })).status).toBe(400);
    const leaf = (await create({ type: "task", title: "leaf", parent_key: story })).body.key;
    const changed = await srv.as("owner").patch(`/api/v1/issues/${leaf}`, { type: "feature" });
    expect(changed.status).toBe(200);
    expect(changed.body.type).toBe("feature");
  });

  it("shows children, progress and open QA counts", async () => {
    const res = await srv.as("owner").get(`/api/v1/issues/${task}`);
    expect(res.body.open_qa_count).toBe(1);
    expect(res.body.children.map((c: any) => c.type)).toContain("qa");
    const f = (await srv.as("owner").get(`/api/v1/issues/${feature}`)).body;
    expect(f.ancestors.map((a: any) => a.key)).toEqual([epic, story]);
    expect(f.progress.total).toBeGreaterThan(0);
    expect(f.progress.done).toBe(0);
  });
});

describe("updating, comments and history", () => {
  it("updates fields, clears nullable fields and logs activity", async () => {
    const key = (await create({ type: "task", title: "before", assignee: "alice", due_date: "2026-12-01" })).body.key;
    const res = await srv.as("claude").patch(`/api/v1/issues/${key}`, {
      title: "after", description: "**md**", priority: 1, assignee: null, due_date: null, labels: ["x"], state_id: states["In Progress"],
    });
    expect(res.status).toBe(200);
    expect(res.body).toMatchObject({ title: "after", description: "**md**", priority: 1, assignee: null, due_date: null });
    expect(res.body.state.name).toBe("In Progress");
    expect(res.body.started_at).not.toBeNull();

    const done = await srv.as("claude").patch(`/api/v1/issues/${key}`, { state_id: states["Done"] });
    expect(done.body.completed_at).not.toBeNull();
    const reopened = await srv.as("claude").patch(`/api/v1/issues/${key}`, { state_id: states["Todo"] });
    expect(reopened.body.completed_at).toBeNull();

    const activity = (await srv.as("owner").get(`/api/v1/issues/${key}/activity`)).body.activity;
    expect(activity[0]).toMatchObject({ action: "created", source: "api" });
    const stateChanges = activity.filter((a: any) => a.field === "state").map((a: any) => [a.old_value, a.new_value]);
    expect(stateChanges).toEqual([["Todo", "In Progress"], ["In Progress", "Done"], ["Done", "Todo"]]);
    const assignee = activity.find((a: any) => a.field === "assignee");
    expect(assignee).toMatchObject({ old_value: "alice", new_value: "", actor: { username: "claude", kind: "agent" } });
  });

  it("does not log anything for a no-op patch", async () => {
    const key = (await create({ type: "task", title: "same" })).body.key;
    await srv.as("owner").patch(`/api/v1/issues/${key}`, { title: "same" });
    const activity = (await srv.as("owner").get(`/api/v1/issues/${key}/activity`)).body.activity;
    expect(activity).toHaveLength(1);
  });

  it("adds and lists comments", async () => {
    const key = (await create({ type: "task", title: "c" })).body.key;
    expect((await srv.as("claude").post(`/api/v1/issues/${key}/comments`, { body: " " })).status).toBe(400);
    const c = await srv.as("claude").post(`/api/v1/issues/${key}/comments`, { body: "progress note" });
    expect(c.status).toBe(201);
    expect(c.body).toMatchObject({ body: "progress note", source: "api", author: { username: "claude" } });
    const list = (await srv.as("owner").get(`/api/v1/issues/${key}/comments`)).body.comments;
    expect(list.map((x: any) => x.body)).toEqual(["progress note"]);
  });

  it("soft-deletes an issue with its descendants", async () => {
    const parent = (await create({ type: "task", title: "p" })).body.key;
    const child = (await create({ type: "task", title: "c", parent_key: parent })).body.key;
    const res = await srv.as("owner").delete(`/api/v1/issues/${parent}`);
    expect(res.body).toEqual({ deleted: 2 });
    expect((await srv.as("owner").get(`/api/v1/issues/${child}`)).status).toBe(404);
    expect((await srv.as("owner").get(`/api/v1/issues/${parent}`)).status).toBe(404);
  });
});
