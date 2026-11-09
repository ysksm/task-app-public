// Checks that responses match api/openapi.yaml, using the demo data so that
// every field (labels, children, ancestors, QA, comments) is populated.
import { afterAll, beforeAll, describe, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { startServer, type Server } from "../src/harness.js";

let srv: Server;
beforeAll(async () => {
  srv = await startServer({ preset: "demo" });
  // Demo tokens follow the same pattern as the test preset.
});
afterAll(() => srv?.stop());

const as = () => srv.raw("Bearer tk_demo_owner");

describe("responses match the OpenAPI contract", () => {
  const cases: [string, string, () => Promise<{ status: number; body: unknown }>][] = [
    ["getMe", "/me", () => as().get("/api/v1/me")],
    ["serverInfo", "/server", () => as().get("/api/v1/server")],
    ["listUsers", "/users", () => as().get("/api/v1/users")],
    ["listProjects", "/projects", () => as().get("/api/v1/projects")],
    ["getProject", "/projects/APP", () => as().get("/api/v1/projects/APP")],
    ["listStates", "/projects/APP/states", () => as().get("/api/v1/projects/APP/states")],
    ["listLabels", "/projects/APP/labels", () => as().get("/api/v1/projects/APP/labels")],
    ["listIssues", "/issues?include_ancestors", () => as().get("/api/v1/issues?include_ancestors=true&project=APP")],
    ["getIssue", "/issues/APP-3", () => as().get("/api/v1/issues/APP-3")],
    ["listComments", "/issues/APP-3/comments", () => as().get("/api/v1/issues/APP-3/comments")],
    ["listActivity", "/issues/APP-3/activity", () => as().get("/api/v1/issues/APP-3/activity")],
    ["createIssue", "POST /issues", () => as().post("/api/v1/issues", { project_key: "APP", type: "task", title: "contract" })],
    ["updateIssue", "PATCH /issues/APP-4", () => as().patch("/api/v1/issues/APP-4", { priority: 1, assignee: null })],
    ["addComment", "POST comment", () => as().post("/api/v1/issues/APP-4/comments", { body: "hi" })],
    ["listCheckpoints", "/issues/APP-3/checkpoints", () => as().get("/api/v1/issues/APP-3/checkpoints")],
    ["recordCheckpoint", "POST checkpoint", () => as().post("/api/v1/issues/APP-4/checkpoints", { summary: "s", rationale: "r" })],
    ["listQA", "/qa", () => as().get("/api/v1/qa")],
    ["getQA", "/qa/APP-8", () => as().get("/api/v1/qa/APP-8")],
    ["createQA", "POST /qa", () => as().post("/api/v1/qa", { target_key: "APP-4", items: [{ question: "q", answer_type: "multi_choice", options: [{ label: "a" }, { label: "b" }] }] })],
    ["answerQA", "POST answers", async () => {
      const qa = (await as().get("/api/v1/qa/APP-16")).body;
      return as().post("/api/v1/qa/APP-16/answers", { answers: [{ item_id: qa.items[0].id, selected_option_ids: ["no"] }, { item_id: qa.items[1].id, text: "none" }] });
    }],
    ["closeQA", "POST close", () => as().post("/api/v1/qa/APP-16/close")],
    ["agentActivity", "/agents/activity", () => as().get("/api/v1/agents/activity")],
    ["getStoryMap", "/projects/APP/storymap", () => as().get("/api/v1/projects/APP/storymap")],
    ["getStory", "/projects/APP/stories/APP-ST1", () => as().get("/api/v1/projects/APP/stories/APP-ST1")],
    ["getIssue", "/issues/APP-3 with stories", () => as().get("/api/v1/issues/APP-3")],
    ["listTokens", "/tokens", () => as().get("/api/v1/tokens")],
    ["issueToken", "POST /tokens", () => as().post("/api/v1/tokens", { user: "claude", name: "contract" })],
    ["getIssue", "404 error envelope", () => as().get("/api/v1/issues/APP-999")],
    ["createIssue", "422 error envelope", () => as().post("/api/v1/issues", { type: "epic", title: "x", parent_key: "APP-1" })],
  ];
  for (const [op, label, call] of cases) {
    it(`${op} ${label}`, async () => {
      const res = await call();
      assertOperation(op, res.status, res.body);
    });
  }
});
