import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { startServer, type Server } from "../src/harness.js";

let srv: Server;
let task: string;

beforeAll(async () => {
  srv = await startServer();
  await srv.as("owner").post("/api/v1/projects", { key: "APP", name: "App" });
  await srv.as("owner").post("/api/v1/projects", { key: "OTH", name: "Other" });
  task = (await srv.as("owner").post("/api/v1/issues", { project_key: "APP", type: "task", title: "Login", assignee: "claude" })).body.key;
});
afterAll(() => srv?.stop());

describe("checkpoints", () => {
  it("records a checkpoint with its reasoning", async () => {
    const res = await srv.as("claude").post(`/api/v1/issues/${task}/checkpoints`, {
      summary: "Implemented the form", decision: "Use React Hook Form", rationale: "Already a dependency",
      alternatives: ["Formik"], confidence: "high", progress: 30, next_steps: "Validation",
      artifacts: { branch: "feat/login", commits: ["abc123"] },
    });
    expect(res.status).toBe(201);
    expect(res.body).toMatchObject({
      issue_key: task, seq: 1, author: { username: "claude", kind: "agent" }, summary: "Implemented the form",
      decision: "Use React Hook Form", rationale: "Already a dependency", alternatives: ["Formik"], confidence: "high",
      progress: 30, next_steps: "Validation", artifacts: { branch: "feat/login", commits: ["abc123"] }, source: "api",
    });
  });

  it("numbers checkpoints per issue and defaults optional fields", async () => {
    const res = await srv.as("claude").post(`/api/v1/issues/${task}/checkpoints`, { summary: "Second", rationale: "Because" });
    expect(res.body).toMatchObject({ seq: 2, confidence: "medium", progress: null, alternatives: [], artifacts: {}, decision: "" });
    const list = (await srv.as("owner").get(`/api/v1/issues/${task}/checkpoints`)).body.checkpoints;
    expect(list.map((c: any) => c.seq)).toEqual([1, 2]);
  });

  it("requires a summary and a rationale and validates ranges", async () => {
    const post = (b: object) => srv.as("claude").post(`/api/v1/issues/${task}/checkpoints`, b);
    expect((await post({ rationale: "r" })).status).toBe(400);
    const noReason = await post({ summary: "s" });
    expect(noReason.status).toBe(400);
    expect(noReason.body.error.message).toMatch(/rationale/);
    expect((await post({ summary: "s", rationale: "r", confidence: "sure" })).status).toBe(400);
    expect((await post({ summary: "s", rationale: "r", progress: 101 })).status).toBe(400);
    expect((await srv.as("claude").post("/api/v1/issues/APP-999/checkpoints", { summary: "s", rationale: "r" })).status).toBe(404);
  });

  it("shows checkpoints in the history and touches the issue", async () => {
    const activity = (await srv.as("owner").get(`/api/v1/issues/${task}/activity`)).body.activity;
    expect(activity.filter((a: any) => a.action === "checkpoint").map((a: any) => a.new_value)).toEqual(["Implemented the form", "Second"]);
  });
});

describe("agent activity feed", () => {
  beforeAll(async () => {
    await srv.as("codex").post(`/api/v1/issues/${task}/checkpoints`, { summary: "Unsure", rationale: "Guess", confidence: "low" });
    const other = (await srv.as("owner").post("/api/v1/issues", { project_key: "OTH", type: "task", title: "x" })).body.key;
    await srv.as("codex").post(`/api/v1/issues/${other}/checkpoints`, { summary: "Other project", rationale: "r" });
    await srv.as("claude").post("/api/v1/qa", { target_key: task, is_blocking: true, items: [{ question: "Q?", answer_type: "yes_no" }] });
    const states = (await srv.as("owner").get("/api/v1/projects/APP/states")).body.states;
    await srv.as("claude").patch(`/api/v1/issues/${task}`, { state_id: states.find((s: any) => s.name === "In Progress").id });
    // Human work is not part of the agent feed.
    await srv.as("owner").post(`/api/v1/issues/${task}/checkpoints`, { summary: "Human note", rationale: "r" });
  });

  it("lists agent checkpoints, questions and state changes, newest first", async () => {
    const entries = (await srv.as("owner").get("/api/v1/agents/activity")).body.entries;
    expect(entries[0]).toMatchObject({ kind: "state", actor: { username: "claude" }, change: { from: "Todo", to: "In Progress" }, issue: { key: task } });
    const kinds = entries.map((e: any) => e.kind);
    expect(kinds.filter((k: string) => k === "checkpoint")).toHaveLength(4);
    const q = entries.find((e: any) => e.kind === "question");
    expect(q.question).toMatchObject({ status: "open", is_blocking: true, item_count: 1, target_key: task });
    expect(entries.every((e: any) => e.actor.kind === "agent")).toBe(true);
    const times = entries.map((e: any) => e.at);
    expect([...times].sort().reverse()).toEqual(times);
  });

  it("filters by agent, project and low confidence", async () => {
    const byCodex = (await srv.as("owner").get("/api/v1/agents/activity?agent=codex")).body.entries;
    expect(byCodex.every((e: any) => e.actor.username === "codex")).toBe(true);
    expect(byCodex).toHaveLength(2);
    const app = (await srv.as("owner").get("/api/v1/agents/activity?project=OTH")).body.entries;
    expect(app.map((e: any) => e.checkpoint?.summary)).toEqual(["Other project"]);
    const low = (await srv.as("owner").get("/api/v1/agents/activity?low_confidence=true")).body.entries;
    expect(low.map((e: any) => e.checkpoint.summary)).toEqual(["Unsure"]);
  });
});
