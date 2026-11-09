import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { startServer, type Server } from "../src/harness.js";

let srv: Server;
let target: string;

beforeAll(async () => {
  srv = await startServer();
  await srv.as("owner").post("/api/v1/projects", { key: "APP", name: "App" });
  target = (await srv.as("owner").post("/api/v1/issues", { project_key: "APP", type: "task", title: "Login", assignee: "claude" })).body.key;
});
afterAll(() => srv?.stop());

const ask = (body: Record<string, unknown>) => srv.as("claude").post("/api/v1/qa", { target_key: target, ...body });

const fourKinds = [
  { question: "Is 8 characters enough?", answer_type: "yes_no", options: [{ id: "yes", recommended: true }] },
  { question: "Japanese error messages only?", answer_type: "ok_ng" },
  {
    question: "Which password rule?", answer_type: "single_choice", allow_other: true,
    options: [{ id: "a", label: "8+", recommended: true }, { id: "b", label: "12+ with symbols" }, { id: "c", label: "8+ mixed" }],
  },
  {
    question: "Which elements to show?", answer_type: "multi_choice",
    options: [{ label: "Remember me" }, { label: "Forgot password" }, { label: "Sign up" }, { label: "SSO" }],
  },
  { question: "Anything else?", answer_type: "text", required: false },
];

describe("asking questions", () => {
  it("creates a QA ticket with several questions under the target", async () => {
    const res = await ask({ context: "No rule in the spec", provisional_decision: "Using 8+", is_blocking: true, items: fourKinds });
    expect(res.status).toBe(201);
    const qa = res.body;
    expect(qa).toMatchObject({
      project_key: "APP", status: "open", target_key: target, is_blocking: true, context: "No rule in the spec",
      provisional_decision: "Using 8+", asked_by: { username: "claude", kind: "agent" }, assignee: { username: "owner" },
      answered_at: null, closed_at: null,
    });
    expect(qa.title).toBe("Is 8 characters enough?（他4件）");
    expect(qa.state.name).toBe("未回答");
    expect(qa.items.map((i: any) => i.answer_type)).toEqual(["yes_no", "ok_ng", "single_choice", "multi_choice", "text"]);
    expect(qa.items[0].options).toEqual([{ id: "yes", label: "はい", recommended: true }, { id: "no", label: "いいえ" }]);
    expect(qa.items[1].options.map((o: any) => o.label)).toEqual(["OK", "NG"]);
    expect(qa.items[3].options.map((o: any) => o.id)).toEqual(["a", "b", "c", "d"]);
    expect(qa.items[4]).toMatchObject({ required: false, options: [], answer: null });

    // The QA is a child issue of the target and counts as open + blocking there.
    const issue = (await srv.as("owner").get(`/api/v1/issues/${target}`)).body;
    expect(issue.open_qa_count).toBe(1);
    expect(issue.blocking_qa_count).toBe(1);
    const child = issue.children.find((c: any) => c.key === qa.key);
    expect(child.type).toBe("qa");
    expect(child.description).toContain("Which password rule?");
  });

  it("validates questions", async () => {
    const bad: [string, unknown][] = [
      ["no items", []],
      ["unknown type", [{ question: "q", answer_type: "scale" }]],
      ["empty question", [{ question: " ", answer_type: "text" }]],
      ["too few options", [{ question: "q", answer_type: "single_choice", options: [{ label: "only" }] }]],
      ["too many options", [{ question: "q", answer_type: "multi_choice", options: "abcdefg".split("").map((l) => ({ label: l })) }]],
      ["missing label", [{ question: "q", answer_type: "single_choice", options: [{ label: "x" }, { label: "" }] }]],
      ["duplicate id", [{ question: "q", answer_type: "single_choice", options: [{ id: "x", label: "1" }, { id: "x", label: "2" }] }]],
      ["options on text", [{ question: "q", answer_type: "text", options: [{ label: "x" }] }]],
    ];
    for (const [name, items] of bad) {
      const res = await ask({ items });
      expect(res.status, name).toBe(400);
    }
    expect((await srv.as("claude").post("/api/v1/qa", { target_key: "APP-999", items: fourKinds })).status).toBe(400);
  });

  it("needs a project without a target and accepts an explicit assignee", async () => {
    expect((await srv.as("claude").post("/api/v1/qa", { items: fourKinds })).status).toBe(400);
    const res = await srv.as("claude").post("/api/v1/qa", { project_key: "APP", assignee: "alice", items: [{ question: "Root?", answer_type: "text" }] });
    expect(res.body).toMatchObject({ target_key: null, assignee: { username: "alice" }, title: "Root?" });
  });
});

describe("answering", () => {
  let key: string;
  let ids: string[];
  beforeAll(async () => {
    const qa = (await ask({ is_blocking: true, items: fourKinds })).body;
    key = qa.key;
    ids = qa.items.map((i: any) => i.id);
  });

  it("rejects answers that do not fit the format", async () => {
    const cases: [string, unknown][] = [
      ["unknown item", { item_id: "nope", selected_option_ids: ["yes"] }],
      ["yes_no needs one", { item_id: ids[0], selected_option_ids: ["yes", "no"] }],
      ["unknown option", { item_id: ids[1], selected_option_ids: ["maybe"] }],
      ["yes_no rejects text", { item_id: ids[0], selected_option_ids: ["yes"], text: "x" }],
      ["single needs exactly one", { item_id: ids[2], selected_option_ids: ["a", "b"] }],
      ["single option + other", { item_id: ids[2], selected_option_ids: ["a"], text: "other" }],
      ["multi needs one", { item_id: ids[3], selected_option_ids: [] }],
      ["multi no other allowed", { item_id: ids[3], selected_option_ids: ["a"], text: "x" }],
      ["text needs text", { item_id: ids[4], text: "  " }],
      ["text rejects options", { item_id: ids[4], selected_option_ids: ["a"], text: "x" }],
    ];
    for (const [name, answer] of cases) {
      const res = await srv.as("owner").post(`/api/v1/qa/${key}/answers`, { answers: [answer] });
      expect(res.status, name).toBe(400);
    }
    expect((await srv.as("owner").post(`/api/v1/qa/${key}/answers`, { answers: [] })).status).toBe(400);
  });

  it("saves partial answers without changing the status", async () => {
    const res = await srv.as("owner").post(`/api/v1/qa/${key}/answers`, {
      answers: [{ item_id: ids[0], selected_option_ids: ["no"], note: "Use 12" }],
    });
    expect(res.status).toBe(200);
    expect(res.body.status).toBe("open");
    expect(res.body.items[0].answer).toMatchObject({
      selected_option_ids: ["no"], selected_options: [{ id: "no", label: "いいえ" }], text: null, note: "Use 12",
      answered_by: { username: "owner", kind: "human" },
    });
  });

  it("becomes answered when every required question is answered (optional ones may stay empty)", async () => {
    const before = (await srv.as("owner").get(`/api/v1/issues/${target}`)).body.blocking_qa_count;
    const res = await srv.as("owner").post(`/api/v1/qa/${key}/answers`, {
      answers: [
        { item_id: ids[1], selected_option_ids: ["ok"] },
        { item_id: ids[2], text: "10+ characters" }, // free "other" answer
        { item_id: ids[3], selected_option_ids: ["b", "a", "b"] },
      ],
    });
    expect(res.body.status).toBe("answered");
    expect(res.body.state.name).toBe("回答済");
    expect(res.body.answered_at).not.toBeNull();
    expect(res.body.items[2].answer).toMatchObject({ selected_option_ids: [], text: "10+ characters" });
    expect(res.body.items[3].answer.selected_option_ids).toEqual(["b", "a"]);
    expect(res.body.items[3].answer.selected_options.map((o: any) => o.label)).toEqual(["Forgot password", "Remember me"]);
    expect(res.body.items[4].answer).toBeNull();

    // This QA no longer blocks the target (another blocking QA from earlier tests still does).
    const after = (await srv.as("owner").get(`/api/v1/issues/${target}`)).body.blocking_qa_count;
    expect(after).toBe(before - 1);
  });

  it("lets the agent read answers of its questions", async () => {
    const mine = (await srv.as("claude").get("/api/v1/qa?asked_by=me&status=answered")).body.questions;
    expect(mine.map((q: any) => q.key)).toContain(key);
    expect((await srv.as("codex").get("/api/v1/qa?asked_by=me")).body.questions).toEqual([]);
    const forTarget = (await srv.as("claude").get(`/api/v1/qa?target=${target}`)).body.questions;
    expect(forTarget.length).toBeGreaterThanOrEqual(2);
    const inbox = (await srv.as("owner").get("/api/v1/qa?assignee=me&status=open")).body.questions;
    expect(inbox.map((q: any) => q.key)).not.toContain(key);
  });

  it("allows re-answering until closed, then refuses", async () => {
    const again = await srv.as("owner").post(`/api/v1/qa/${key}/answers`, { answers: [{ item_id: ids[0], selected_option_ids: ["yes"] }] });
    expect(again.body.items[0].answer.selected_option_ids).toEqual(["yes"]);
    const closed = await srv.as("claude").post(`/api/v1/qa/${key}/close`);
    expect(closed.body).toMatchObject({ status: "closed" });
    expect(closed.body.state.name).toBe("クローズ");
    expect(closed.body.closed_at).not.toBeNull();
    const late = await srv.as("owner").post(`/api/v1/qa/${key}/answers`, { answers: [{ item_id: ids[0], selected_option_ids: ["no"] }] });
    expect(late.status).toBe(409);
  });

  it("records the answer in the issue history", async () => {
    const activity = (await srv.as("owner").get(`/api/v1/issues/${key}/activity`)).body.activity;
    const actions = activity.map((a: any) => `${a.action}:${a.field}:${a.new_value}`);
    expect(actions).toContain("answered::1");
    expect(actions).toContain("updated:state:回答済");
    expect(actions).toContain("updated:state:クローズ");
  });

  it("returns 404 for unknown or non-QA keys", async () => {
    expect((await srv.as("owner").get("/api/v1/qa/APP-999")).status).toBe(404);
    expect((await srv.as("owner").get(`/api/v1/qa/${target}`)).status).toBe(404);
    expect((await srv.as("owner").get("/api/v1/qa?status=pending")).status).toBe(400);
  });
});
