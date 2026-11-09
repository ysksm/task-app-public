// Requirements layer (docs/08-devcycle.md §5, M1): personas, jobs, the story
// map (activities → steps), stories, acceptance criteria and their links to
// issues. Keys are per project and kind and never reused; removal archives.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { startServer, type Client, type Server } from "../src/harness.js";

let srv: Server;
let owner: Client;
const base = "/api/v1/projects/APP";

beforeAll(async () => {
  srv = await startServer();
  owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
  await owner.post("/api/v1/projects", { key: "OPS", name: "Ops" });
});
afterAll(() => srv?.stop());

describe("story map", () => {
  it("starts empty", async () => {
    const res = await owner.get(`${base}/storymap`);
    expect(res.status).toBe(200);
    assertOperation("getStoryMap", 200, res.body);
    expect(res.body).toEqual({
      project_key: "APP", mode: "quick", draft_of: null, personas: [], jobs: [], activities: [], stories: [], releases: [], release_slices: [],
      gaps: { stories_without_steps: [], steps_without_stories: [], stories_without_criteria: [], stories_without_issues: [], jobs_without_activities: [] },
    });
    expect((await owner.get("/api/v1/projects/NOPE/storymap")).status).toBe(404);
  });

  it("builds personas, jobs, activities, steps, stories and criteria with readable keys", async () => {
    const p1 = await owner.post(`${base}/personas`, { name: "オーナー", description: "複数のプロジェクトを並行して進める" });
    expect(p1.status).toBe(201);
    assertOperation("createPersona", 201, p1.body);
    expect(p1.body).toMatchObject({ key: "APP-P1", name: "オーナー", kind: "human", position: 1 });
    const p2 = await owner.post(`${base}/personas`, { name: "AI エージェント", kind: "agent" });
    expect(p2.body).toMatchObject({ key: "APP-P2", kind: "agent", position: 2 });

    const a1 = await owner.post(`${base}/activities`, { name: "朝: 今日を決める", persona_key: "APP-P1" });
    expect(a1.status).toBe(201);
    assertOperation("createActivity", 201, a1.body);
    expect(a1.body).toMatchObject({ key: "APP-A1", persona_key: "APP-P1", steps: [] });
    const a2 = await owner.post(`${base}/activities`, { name: "AI に答える", persona_key: "p1" });
    expect(a2.body).toMatchObject({ key: "APP-A2", persona_key: "APP-P1" });

    const s11 = await owner.post(`${base}/activities/APP-A1/steps`, { name: "自分の未完了だけを見る" });
    expect(s11.status).toBe(201);
    assertOperation("createStep", 201, s11.body);
    expect(s11.body).toMatchObject({ key: "APP-A1-1", activity_key: "APP-A1", position: 1 });
    expect((await owner.post(`${base}/activities/A1/steps`, { name: "注力に絞る" })).body.key).toBe("APP-A1-2");
    expect((await owner.post(`${base}/activities/APP-A2/steps`, { name: "質問に答える" })).body.key).toBe("APP-A2-1");

    const job = await owner.post(`${base}/jobs`, { persona_key: "APP-P1", situation: "朝、作業を始める前", motivation: "今日手をつけるものを数件に絞りたい", outcome: "迷わずに始められる", activity_keys: ["APP-A1"] });
    expect(job.status).toBe(201);
    assertOperation("createJob", 201, job.body);
    expect(job.body).toMatchObject({ key: "APP-J1", activity_keys: ["APP-A1"] });

    const st = await owner.post(`${base}/stories`, { title: "完了を隠すを既定に", persona_key: "APP-P1", i_want: "朝は未完了だけを見たい", so_that: "迷わない", release: "R1", step_keys: ["APP-A1-1"] });
    expect(st.status).toBe(201);
    assertOperation("createStory", 201, st.body);
    expect(st.body).toMatchObject({ key: "APP-ST1", step_keys: ["APP-A1-1"], status: "draft", criteria_count: 0, issues: [], criteria: [] });
    expect(st.body.history.map((h: any) => h.action)).toEqual(["created"]);

    const c = await owner.post(`${base}/stories/APP-ST1/criteria`, { given: "マイタスクを開いた", when: "何も操作しない", then: "完了したイシューは表示されない" });
    expect(c.status).toBe(201);
    assertOperation("createCriterion", 201, c.body);
    expect(c.body).toMatchObject({ key: "APP-AC1", story_key: "APP-ST1", then: "完了したイシューは表示されない", position: 1 });

    const map = (await owner.get(`${base}/storymap`)).body;
    assertOperation("getStoryMap", 200, map);
    expect(map.personas.map((x: any) => x.key)).toEqual(["APP-P1", "APP-P2"]);
    expect(map.activities.map((a: any) => [a.key, a.steps.map((s: any) => s.key)])).toEqual([["APP-A1", ["APP-A1-1", "APP-A1-2"]], ["APP-A2", ["APP-A2-1"]]]);
    expect(map.stories).toHaveLength(1);
    expect(map.stories[0]).toMatchObject({ key: "APP-ST1", status: "ready", criteria_count: 1 });
    expect(map.stories[0].criteria).toBeUndefined(); // detail only
    expect(map.releases).toEqual(["R1"]);
    expect(map.gaps).toEqual({
      stories_without_steps: [], steps_without_stories: ["APP-A1-2", "APP-A2-1"], stories_without_criteria: [],
      stories_without_issues: ["APP-ST1"], jobs_without_activities: [],
    });

    const listed = (await owner.get(`${base}/personas`)).body;
    assertOperation("listPersonas", 200, listed);
    assertOperation("listJobs", 200, (await owner.get(`${base}/jobs`)).body);
    const stories = (await owner.get(`${base}/stories`)).body;
    assertOperation("listStories", 200, stories);
    expect(stories.stories.map((x: any) => x.key)).toEqual(["APP-ST1"]);
  });

  it("links issues of the same project and derives the story status from them", async () => {
    const task = (await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "マイタスクの既定を変える" })).body;
    const other = (await owner.post("/api/v1/issues", { project_key: "OPS", type: "task", title: "別のプロジェクト" })).body;
    const linked = await owner.put(`${base}/stories/APP-ST1/issues/${task.key}`);
    expect(linked.status).toBe(200);
    assertOperation("linkStoryIssue", 200, linked.body);
    expect(linked.body.issues).toEqual([{ key: task.key, type: "task", title: "マイタスクの既定を変える", state_name: expect.any(String), state_category: "unstarted" }]);
    expect(linked.body.status).toBe("ready");
    // Linking again changes nothing (no second history entry).
    expect((await owner.put(`${base}/stories/APP-ST1/issues/${task.key}`)).body.history.filter((h: any) => h.action === "linked")).toHaveLength(1);

    expect((await owner.put(`${base}/stories/APP-ST1/issues/${other.key}`)).status).toBe(400);
    expect((await owner.put(`${base}/stories/APP-ST1/issues/APP-999`)).status).toBe(404);

    // The issue tells why it exists (also over MCP: get_issue).
    const detail = (await owner.get(`/api/v1/issues/${task.key}`)).body;
    assertOperation("getIssue", 200, detail);
    expect(detail.stories).toEqual([expect.objectContaining({
      key: "APP-ST1", title: "完了を隠すを既定に", persona_key: "APP-P1", i_want: "朝は未完了だけを見たい", so_that: "迷わない", step_keys: ["APP-A1-1"],
      criteria: [expect.objectContaining({ key: "APP-AC1", given: "マイタスクを開いた", when: "何も操作しない", then: "完了したイシューは表示されない" })],
    })]);
    expect((await owner.get(`/api/v1/issues/${other.key}`)).body.stories).toEqual([]);
    // Lists stay light.
    expect((await owner.get("/api/v1/issues?project=APP")).body.issues[0].stories).toBeUndefined();

    const states = (await owner.get("/api/v1/projects/APP/states")).body.states;
    const started = states.find((s: any) => s.category === "started" && s.applies_to === "all");
    const done = states.find((s: any) => s.category === "completed" && s.applies_to === "all");
    await owner.patch(`/api/v1/issues/${task.key}`, { state_id: started.id });
    expect((await owner.get(`${base}/stories/APP-ST1`)).body.status).toBe("in_progress");
    await owner.patch(`/api/v1/issues/${task.key}`, { state_id: done.id });
    expect((await owner.get(`${base}/stories/APP-ST1`)).body.status).toBe("done");
    // A finished task does not close the story: it stays on the map.
    expect((await owner.get(`${base}/storymap`)).body.stories.map((s: any) => s.key)).toEqual(["APP-ST1"]);

    const unlinked = await owner.delete(`${base}/stories/APP-ST1/issues/${task.key}`);
    expect(unlinked.status).toBe(200);
    assertOperation("unlinkStoryIssue", 200, unlinked.body);
    expect(unlinked.body).toMatchObject({ issues: [], status: "ready" });
    expect(unlinked.body.history.map((h: any) => h.action).slice(0, 2)).toEqual(["unlinked", "linked"]);
    await owner.put(`${base}/stories/APP-ST1/issues/${task.key}`);
  });

  it("updates every element and records the changes", async () => {
    const p = await owner.patch(`${base}/personas/APP-P2`, { name: "Claude", description: "MCP で作業する" });
    expect(p.status).toBe(200);
    assertOperation("updatePersona", 200, p.body);
    expect(p.body).toMatchObject({ key: "APP-P2", name: "Claude", kind: "agent" });

    const a = await owner.patch(`${base}/activities/APP-A2`, { name: "AI に答える・受け入れる", persona_key: null });
    assertOperation("updateActivity", 200, a.body);
    expect(a.body).toMatchObject({ name: "AI に答える・受け入れる", persona_key: null });

    const s = await owner.patch(`${base}/steps/APP-A2-1`, { name: "QA に答える" });
    assertOperation("updateStep", 200, s.body);
    expect(s.body).toMatchObject({ key: "APP-A2-1", name: "QA に答える" });

    const j = await owner.patch(`${base}/jobs/APP-J1`, { activity_keys: ["APP-A1", "APP-A2"], outcome: "すぐ始められる" });
    assertOperation("updateJob", 200, j.body);
    expect(j.body).toMatchObject({ activity_keys: ["APP-A1", "APP-A2"], outcome: "すぐ始められる" });

    const st = await owner.patch(`${base}/stories/APP-ST1`, { step_keys: ["APP-A1-1", "APP-A2-1"], release: "R2", title: "完了を隠す（既定）" });
    expect(st.status).toBe(200);
    assertOperation("updateStory", 200, st.body);
    expect(st.body).toMatchObject({ step_keys: ["APP-A1-1", "APP-A2-1"], release: "R2", title: "完了を隠す（既定）" });
    expect(st.body.history[0]).toMatchObject({ entity_key: "APP-ST1", action: "updated", actor: { username: "owner" } });
    expect(st.body.history[0].detail).toMatchObject({ release: "R2", step_keys: ["APP-A1-1", "APP-A2-1"] });

    const c = await owner.patch(`${base}/criteria/APP-AC1`, { then: "完了したイシューは「完了を隠す」で隠れている" });
    assertOperation("updateCriterion", 200, c.body);
    expect(c.body.then).toBe("完了したイシューは「完了を隠す」で隠れている");
    expect((await owner.get(`${base}/stories/APP-ST1`)).body.history[0]).toMatchObject({ entity_key: "APP-AC1", action: "updated" });

    // Agents edit the same map (the users are the owner and AI agents).
    const byAgent = await srv.as("claude").post(`${base}/stories/APP-ST1/criteria`, { then: "MCP の list_my_issues と同じ既定" });
    expect(byAgent.body.key).toBe("APP-AC2");
    expect((await owner.get(`${base}/stories/APP-ST1`)).body.history[0].actor.username).toBe("claude");
  });

  it("archives instead of deleting and never reuses keys", async () => {
    expect((await owner.delete(`${base}/criteria/APP-AC2`)).status).toBe(204);
    expect((await owner.get(`${base}/stories/APP-ST1`)).body.criteria.map((c: any) => c.key)).toEqual(["APP-AC1"]);
    expect((await owner.patch(`${base}/criteria/APP-AC2`, { then: "x" })).status).toBe(404);
    expect((await owner.post(`${base}/stories/APP-ST1/criteria`, { then: "次の条件" })).body.key).toBe("APP-AC3");

    const st2 = (await owner.post(`${base}/stories`, { title: "消すストーリー" })).body;
    expect(st2.key).toBe("APP-ST2");
    expect((await owner.delete(`${base}/stories/APP-ST2`)).status).toBe(204);
    expect((await owner.get(`${base}/stories/APP-ST2`)).status).toBe(404);
    expect((await owner.post(`${base}/stories`, { title: "次" })).body.key).toBe("APP-ST3");

    // Archiving an activity takes its steps off the map; stories placed only there lose their placement.
    const a3 = (await owner.post(`${base}/activities`, { name: "振り返る" })).body;
    await owner.post(`${base}/activities/${a3.key}/steps`, { name: "終わったものを見る" });
    await owner.patch(`${base}/stories/APP-ST3`, { step_keys: [`${a3.key}-1`] });
    expect((await owner.delete(`${base}/activities/${a3.key}`)).status).toBe(204);
    const map = (await owner.get(`${base}/storymap`)).body;
    expect(map.activities.map((a: any) => a.key)).toEqual(["APP-A1", "APP-A2"]);
    expect(map.stories.find((s: any) => s.key === "APP-ST3").step_keys).toEqual([]);
    expect(map.gaps.stories_without_steps).toEqual(["APP-ST3"]);
    expect(map.gaps.stories_without_criteria).toEqual(["APP-ST3"]);

    expect((await owner.delete(`${base}/steps/APP-A1-2`)).status).toBe(204);
    expect((await owner.post(`${base}/activities/APP-A1/steps`, { name: "注力に絞る（2）" })).body.key).toBe("APP-A1-3");

    // An archived persona disappears from the things that referred to it.
    expect((await owner.delete(`${base}/personas/APP-P1`)).status).toBe(204);
    const after = (await owner.get(`${base}/storymap`)).body;
    expect(after.personas.map((p: any) => p.key)).toEqual(["APP-P2"]);
    expect(after.activities[0].persona_key).toBeNull();
    expect((await owner.post(`${base}/personas`, { name: "新" })).body.key).toBe("APP-P3");

    expect((await owner.delete(`${base}/jobs/APP-J1`)).status).toBe(204);
    expect((await owner.get(`${base}/jobs`)).body.jobs).toEqual([]);
  });

  it("keeps keys per project", async () => {
    expect((await owner.post("/api/v1/projects/OPS/stories", { title: "別" })).body.key).toBe("OPS-ST1");
    // A key of another project does not resolve here.
    expect((await owner.get(`${base}/stories/OPS-ST1`)).status).toBe(404);
  });

  it("rejects invalid input", async () => {
    const bad: [string, string, unknown, number][] = [
      ["post", `${base}/personas`, { name: " " }, 400],
      ["post", `${base}/personas`, { name: "x", kind: "robot" }, 400],
      ["post", `${base}/jobs`, { motivation: "" }, 400],
      ["post", `${base}/jobs`, { motivation: "x", activity_keys: ["APP-A99"] }, 400],
      ["post", `${base}/jobs`, { motivation: "x", persona_key: "APP-P99" }, 400],
      ["post", `${base}/activities`, { name: "" }, 400],
      ["post", `${base}/activities/APP-A99/steps`, { name: "x" }, 404],
      ["post", `${base}/stories`, { title: "" }, 400],
      ["post", `${base}/stories`, { title: "x", step_keys: ["APP-A1-99"] }, 400],
      ["post", `${base}/stories`, { title: "x", step_keys: ["nonsense"] }, 400],
      ["post", "/api/v1/projects/OPS/stories", { title: "x", step_keys: ["APP-A1-1"] }, 400],
      ["post", `${base}/stories/APP-ST1/criteria`, { given: "g", when: "w" }, 400],
      ["patch", `${base}/stories/APP-ST1`, { title: " " }, 400],
      ["patch", `${base}/stories/APP-ST99`, { title: "x" }, 404],
      ["patch", `${base}/stories/XYZ`, { title: "x" }, 404],
      ["post", `${base}/stories`, { title: "x", unknown_field: 1 }, 400],
    ];
    for (const [method, path, body, status] of bad) {
      const res = await (owner as any)[method](path, body);
      expect(res.status, `${method} ${path} ${JSON.stringify(body)}`).toBe(status);
    }
  });

  it("is in the export", async () => {
    const dump = (await owner.get("/api/v1/export")).body;
    expect(dump.tables.req_stories.map((s: any) => s.num).sort()).toEqual([1, 1, 2, 3]);
    expect(dump.tables.req_story_issues).toHaveLength(1);
    expect(dump.tables.req_changes.length).toBeGreaterThan(10);
  });

  it("is readable over MCP (get_storymap, get_issue)", async () => {
    const mcp = await srv.mcpHttp("claude");
    const res: any = await mcp.callTool({ name: "get_storymap", arguments: { project: "APP" } });
    const map = JSON.parse(res.content[0].text);
    expect(map.stories.map((s: any) => s.key)).toEqual(["APP-ST1", "APP-ST3"]);
    const issueKey = map.stories[0].issues[0].key;
    const got: any = await mcp.callTool({ name: "get_issue", arguments: { key: issueKey } });
    expect(JSON.parse(got.content[0].text).issue.stories[0].key).toBe("APP-ST1");
  });
});
