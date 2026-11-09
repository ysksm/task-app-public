// Option D (docs/09-requirements-loose.md §3): the M1 tables used loosely. A story
// can be a title alone (unsorted: no persona, step or release), carries a free
// Markdown body, and a criterion can be one line. The AI tidies stories through
// MCP create_story / update_story after asking with ask_question.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { callTool, startServer, type Client, type Server } from "../src/harness.js";

let srv: Server;
let owner: Client;
const base = "/api/v1/projects/APP";

beforeAll(async () => {
  srv = await startServer();
  owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
});
afterAll(() => srv?.stop());

describe("loose stories", () => {
  it("creates a story from a title alone; it is unsorted and has an empty body", async () => {
    const res = await owner.post(`${base}/stories`, { title: "朝の画面で、AI に任せたものだけを見たい" });
    expect(res.status).toBe(201);
    assertOperation("createStory", 201, res.body);
    expect(res.body).toMatchObject({ key: "APP-ST1", body: "", persona_key: null, release: "", step_keys: [], status: "draft" });
    const map = (await owner.get(`${base}/storymap`)).body;
    assertOperation("getStoryMap", 200, map);
    expect(map.activities).toEqual([]);
    expect(map.gaps.stories_without_steps).toEqual(["APP-ST1"]);
  });

  it("keeps a free Markdown body and records its changes", async () => {
    const created = await owner.post(`${base}/stories`, { title: "週末に滞りを見たい", body: "  金曜の夕方に\n- 止まっているもの  " });
    expect(created.body.body).toBe("金曜の夕方に\n- 止まっているもの");
    const patched = await owner.patch(`${base}/stories/${created.body.key}`, { body: "書き直した" });
    expect(patched.status).toBe(200);
    assertOperation("updateStory", 200, patched.body);
    expect(patched.body.body).toBe("書き直した");
    expect(patched.body.history[0]).toMatchObject({ action: "updated", detail: { body: "書き直した" } });
    expect((await owner.patch(`${base}/stories/${created.body.key}`, { bodyy: "x" })).status).toBe(400);
  });

  it("accepts a one-line criterion (then only) and colours the card", async () => {
    const c = await owner.post(`${base}/stories/APP-ST1/criteria`, { then: "AI が担当している自分の起票だけが並ぶ" });
    expect(c.status).toBe(201);
    expect(c.body).toMatchObject({ given: "", when: "", then: "AI が担当している自分の起票だけが並ぶ" });
    expect((await owner.get(`${base}/stories/APP-ST1`)).body.status).toBe("ready");
    expect((await owner.post(`${base}/stories/APP-ST1/criteria`, { then: "  " })).status).toBe(400);
  });

  it("shows the body on the issue, so the agent can read why", async () => {
    const task = (await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "任せたものの一覧" })).body.key;
    await owner.patch(`${base}/stories/APP-ST1`, { body: "自分が起票して AI が担当しているもの" });
    await owner.put(`${base}/stories/APP-ST1/issues/${task}`);
    const issue = (await owner.get(`/api/v1/issues/${task}`)).body;
    assertOperation("getIssue", 200, issue);
    expect(issue.stories[0]).toMatchObject({ key: "APP-ST1", body: "自分が起票して AI が担当しているもの" });
  });
});

describe("MCP create_story / update_story", () => {
  it("lets the agent add a story and tidy one: placement, release, body and criteria", async () => {
    const act = (await owner.post(`${base}/activities`, { name: "朝: 今日を決める" })).body;
    const step = (await owner.post(`${base}/activities/${act.key}/steps`, { name: "自分の未完了だけを見る" })).body;
    const claude = await srv.mcpHttp("claude");

    const made = await callTool(claude, "create_story", { project: "APP", title: "AI が質問をまとめて返す", criteria: [{ then: "質問が 1 件にまとまる" }] });
    expect(made).toMatchObject({ title: "AI が質問をまとめて返す", step_keys: [], status: "ready" });
    expect(made.criteria.map((c: any) => c.then)).toEqual(["質問が 1 件にまとまる"]);

    const crit = (await owner.get(`${base}/stories/APP-ST2`)).body;
    const tidied = await callTool(claude, "update_story", {
      project: "APP", key: "ST2", step_keys: [step.key], release: "R1", body: "金曜に見る",
      add_criteria: [{ given: "金曜の夕方", then: "3 日動いていないものが上に出る" }],
    });
    expect(tidied).toMatchObject({ key: "APP-ST2", step_keys: [step.key], release: "R1", body: "金曜に見る", status: "ready" });
    expect(tidied.criteria.at(-1)).toMatchObject({ given: "金曜の夕方", when: "", then: "3 日動いていないものが上に出る" });
    expect(crit.status).toBe("draft");

    const ckey = tidied.criteria[0].key;
    const edited = await callTool(claude, "update_story", { project: "APP", key: "APP-ST2", step_keys: [], update_criteria: [{ key: ckey, then: "滞りが上に出る" }] });
    expect(edited.step_keys).toEqual([]);
    expect(edited.criteria[0].then).toBe("滞りが上に出る");
    // The change is attributed to the agent.
    expect(edited.history[0].actor.username).toBe("claude");

    const bad: any = await claude.callTool({ name: "update_story", arguments: { project: "APP", key: "APP-ST99", title: "x" } });
    expect(bad.isError).toBe(true);
  });
});
