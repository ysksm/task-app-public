// Acceptance inspection (docs/01-requirements.md §4.8, ACC-01〜05).
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { callTool, startServer, type Client, type Server } from "../src/harness.js";

let srv: Server;
let owner: Client;
let epic: string, story: string, feature: string, task: string;

const acceptanceState = async () =>
  (await owner.get("/api/v1/projects/APP/states")).body.states.find((s: any) => s.is_acceptance).id;
const stateNamed = async (name: string) =>
  (await owner.get("/api/v1/projects/APP/states")).body.states.find((s: any) => s.name === name).id;

beforeAll(async () => {
  srv = await startServer();
  owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
  const mk = async (body: Record<string, unknown>) => (await owner.post("/api/v1/issues", { project_key: "APP", ...body })).body.key;
  epic = await mk({ type: "epic", title: "Auth" });
  story = await mk({ type: "story", title: "Log in", parent_key: epic });
  feature = await mk({ type: "feature", title: "Login screen", parent_key: story });
  task = await mk({ type: "task", title: "Form", parent_key: feature });
});
afterAll(() => srv?.stop());

describe("criteria", () => {
  it("are set on stories and features as a replaceable checklist", async () => {
    const res = await owner.put(`/api/v1/issues/${story}/acceptance/criteria`, {
      criteria: [{ description: "正しい資格情報でログインできる" }, { description: "5 回失敗するとロックされる" }, { description: "ログイン状態が 7 日保持される" }],
    });
    expect(res.status).toBe(200);
    assertOperation("setAcceptanceCriteria", res.status, res.body);
    expect(res.body.criteria.map((c: any) => [c.position, c.description])).toEqual([
      [1, "正しい資格情報でログインできる"], [2, "5 回失敗するとロックされる"], [3, "ログイン状態が 7 日保持される"],
    ]);
    expect(res.body).toMatchObject({ issue_key: story, in_acceptance: false, reviews: [] });

    expect((await owner.put(`/api/v1/issues/${feature}/acceptance/criteria`, { criteria: [{ description: "x" }] })).status).toBe(200);
    for (const key of [epic, task]) {
      const bad = await owner.put(`/api/v1/issues/${key}/acceptance/criteria`, { criteria: [{ description: "x" }] });
      expect(bad.status).toBe(400);
      expect(bad.body.error.message).toMatch(/stories and features/);
    }
    expect((await owner.put(`/api/v1/issues/${story}/acceptance/criteria`, { criteria: [{ description: " " }] })).status).toBe(400);
  });

  it("reports readiness: unfinished children and unanswered QA", async () => {
    const agent = await srv.mcpHttp("claude");
    await callTool(agent, "ask_question", { target_key: task, questions: [{ question: "OK?", answer_type: "ok_ng" }] });
    const a = (await owner.get(`/api/v1/issues/${story}/acceptance`)).body;
    assertOperation("getAcceptance", 200, a);
    expect(a.readiness).toMatchObject({ open_children: 2, open_qa: 1, warnings: ["未完了の子イシューが 2 件あります", "未回答の QA が 1 件あります"] });
    // The reviewer can see which ones: unfinished descendants (nearest first), then unanswered QA.
    expect(a.readiness.items.map((i: any) => [i.key, i.type, i.parent_key])).toEqual([
      [feature, "feature", story], [task, "task", feature], [expect.any(String), "qa", task],
    ]);
    expect(a.readiness.items[0].state).toMatchObject({ name: "Todo", category: "unstarted" });
  });
});

describe("review", () => {
  it("is only possible while the issue waits for acceptance", async () => {
    const criteria = (await owner.get(`/api/v1/issues/${story}/acceptance`)).body.criteria;
    const res = await owner.post(`/api/v1/issues/${story}/acceptance/reviews`, {
      verdict: "rejected", summary: "x", items: criteria.map((c: any) => ({ criterion_id: c.id, result: "ok" })),
    });
    expect(res.status).toBe(409);
    expect(res.body.error.message).toMatch(/not waiting for acceptance/);
  });

  it("is listed in the acceptance queue", async () => {
    // An agent asks for acceptance (MCP request_acceptance).
    const agent = await srv.mcpHttp("claude");
    await callTool(agent, "request_acceptance", { key: story });
    const queue = (await owner.get("/api/v1/acceptance?project=APP")).body;
    assertOperation("acceptanceQueue", 200, queue);
    expect(queue.items).toHaveLength(1);
    expect(queue.items[0]).toMatchObject({ issue: { key: story, state: { name: "受入待ち" } }, criteria_count: 3, rounds: 0, last_review: null });
    expect(queue.items[0].readiness.open_children).toBe(2);
  });

  it("validates the judgement", async () => {
    const criteria = (await owner.get(`/api/v1/issues/${story}/acceptance`)).body.criteria;
    const all = (result: string) => criteria.map((c: any) => ({ criterion_id: c.id, result }));
    const post = (body: Record<string, unknown>) => owner.post(`/api/v1/issues/${story}/acceptance/reviews`, body);

    expect((await post({ verdict: "maybe", items: all("ok") })).status).toBe(400);
    expect((await post({ verdict: "rejected", items: all("ng") })).body.error.message).toMatch(/差し戻し理由/);
    expect((await post({ verdict: "accepted", items: all("ok").slice(1) })).body.error.message).toMatch(/criterion 1 .* has no result/);
    expect((await post({ verdict: "accepted", items: [...all("ok").slice(1), { criterion_id: criteria[0].id, result: "ng" }] })).body.error.message)
      .toMatch(/cannot accept with NG/);
    expect((await post({ verdict: "accepted", items: [...all("ok"), { criterion_id: "nope", result: "ok" }] })).status).toBe(400);

    // Unfinished children / unanswered QA block acceptance unless forced.
    const blocked = await post({ verdict: "accepted", items: all("ok") });
    expect(blocked.status).toBe(409);
    expect(blocked.body.error.message).toMatch(/未完了の子イシューが 2 件.*force/);

    // Agents cannot judge.
    const agentRes = await srv.as("claude").post(`/api/v1/issues/${story}/acceptance/reviews`, { verdict: "accepted", items: all("ok"), force: true });
    expect(agentRes.status).toBe(403);
  });

  it("rejects back to In Progress with the reason, then accepts in round 2", async () => {
    const criteria = (await owner.get(`/api/v1/issues/${story}/acceptance`)).body.criteria;
    const rejected = await owner.post(`/api/v1/issues/${story}/acceptance/reviews`, {
      verdict: "rejected", summary: "エラー文言が英語のまま",
      items: [
        { criterion_id: criteria[0].id, result: "ok" },
        { criterion_id: criteria[1].id, result: "ng", note: "ロック解除の手段がない" },
        { criterion_id: criteria[2].id, result: "na" },
      ],
    });
    expect(rejected.status).toBe(201);
    assertOperation("reviewAcceptance", rejected.status, rejected.body);
    expect(rejected.body.review).toMatchObject({ round: 1, verdict: "rejected", summary: "エラー文言が英語のまま", reviewer: { username: "owner" } });
    expect(rejected.body.review.items[1]).toEqual({ criterion_id: criteria[1].id, description: "5 回失敗するとロックされる", result: "ng", note: "ロック解除の手段がない" });
    expect(rejected.body.issue.state.name).toBe("In Progress");

    // The agent reads why it was sent back.
    const agent = await srv.mcpHttp("claude");
    const detail = await callTool(agent, "get_issue", { key: story });
    expect(detail.acceptance).toMatchObject({ rounds: 1, latest_review: { verdict: "rejected", summary: "エラー文言が英語のまま" } });
    expect(detail.acceptance.criteria).toHaveLength(3);

    // Criteria edited after a review: history keeps the judged text.
    const edited = await owner.put(`/api/v1/issues/${story}/acceptance/criteria`, {
      criteria: [{ id: criteria[0].id, description: criteria[0].description }, { id: criteria[1].id, description: "5 回失敗するとロックされ、30 分で解除される" }],
    });
    expect(edited.status).toBe(200);
    expect(edited.body.criteria).toHaveLength(2);
    expect(edited.body.reviews[0].items.map((i: any) => i.description)).toEqual([
      "正しい資格情報でログインできる", "5 回失敗するとロックされる", "ログイン状態が 7 日保持される",
    ]);

    // Finish the work and ask again.
    const done = await stateNamed("Done");
    await owner.patch(`/api/v1/issues/${task}`, { state_id: done });
    await owner.patch(`/api/v1/issues/${feature}`, { state_id: done });
    const qa = (await owner.get(`/api/v1/qa?target=${task}`)).body.questions[0];
    await owner.post(`/api/v1/qa/${qa.key}/answers`, { answers: [{ item_id: qa.items[0].id, selected_option_ids: ["ok"] }] });
    await owner.patch(`/api/v1/issues/${story}`, { state_id: await acceptanceState() });
    expect((await owner.get(`/api/v1/issues/${story}/acceptance`)).body.readiness).toEqual({ open_children: 0, open_qa: 0, warnings: [], items: [] });

    const accepted = await owner.post(`/api/v1/issues/${story}/acceptance/reviews`, {
      verdict: "accepted", summary: "OK", items: edited.body.criteria.map((c: any) => ({ criterion_id: c.id, result: "ok" })),
    });
    expect(accepted.status).toBe(201);
    expect(accepted.body.review.round).toBe(2);
    expect(accepted.body.issue.state).toMatchObject({ name: "Done", category: "completed" });
    expect(accepted.body.issue.completed_at).not.toBeNull();
    expect(accepted.body.acceptance.reviews.map((r: any) => r.verdict)).toEqual(["rejected", "accepted"]);
    expect((await owner.get("/api/v1/acceptance")).body.items).toEqual([]);

    const activity = (await owner.get(`/api/v1/issues/${story}/activity`)).body.activity;
    assertOperation("listActivity", 200, { activity });
    expect(activity.filter((a: any) => ["accepted", "rejected"].includes(a.action)).map((a: any) => [a.action, a.new_value]))
      .toEqual([["rejected", "エラー文言が英語のまま"], ["accepted", "OK"]]);
  });

  it("can force acceptance despite warnings", async () => {
    const s2 = (await owner.post("/api/v1/issues", { project_key: "APP", type: "story", title: "Sign up", parent_key: epic })).body.key;
    await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "Open child", parent_key: s2 });
    await owner.patch(`/api/v1/issues/${s2}`, { state_id: await acceptanceState() });
    const res = await owner.post(`/api/v1/issues/${s2}/acceptance/reviews`, { verdict: "accepted", items: [], force: true });
    expect(res.status).toBe(201);
    expect(res.body.issue.state.name).toBe("Done");
  });
});
