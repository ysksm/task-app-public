// Drafts of a story map (docs/08-devcycle.md §5.9): brainstorm in a draft
// project, review the import plan, import into the target in one go.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { callTool, startServer, type Client, type Server } from "../src/harness.js";

let srv: Server;
let owner: Client;
let claude: Client;
const T = "/api/v1/projects/APP";
const D = "/api/v1/projects/APPD1";

const plan = async () => {
  const r = await owner.get(`${D}/import-plan`);
  expect(r.status).toBe(200);
  assertOperation("getImportPlan", 200, r.body);
  return r.body;
};
const brief = (p: any) => p.items.map((i: any) => [i.action, i.kind, i.target_key ?? i.draft_key, i.label, i.conflict]);

beforeAll(async () => {
  srv = await startServer();
  owner = srv.as("owner");
  claude = srv.as("claude");
  await owner.post("/api/v1/projects", { key: "APP", name: "App", requirements_mode: "structured" });
  // The target: one persona, one activity with two steps, two stories in R1, one criterion, one linked issue.
  await owner.post(`${T}/personas`, { name: "オーナー" });
  await owner.post(`${T}/activities`, { name: "朝", persona_key: "APP-P1" });
  await owner.post(`${T}/activities/APP-A1/steps`, { name: "一覧を見る" });
  await owner.post(`${T}/activities/APP-A1/steps`, { name: "絞る" });
  await owner.post(`${T}/stories`, { title: "未完了だけ見る", release: "R1", step_keys: ["APP-A1-1"], persona_key: "APP-P1" });
  await owner.post(`${T}/stories`, { title: "古い案", release: "R1", step_keys: ["APP-A1-2"] });
  await owner.post(`${T}/stories/APP-ST1/criteria`, { then: "完了は出ない" });
  const issue = await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "完了を隠す" });
  await owner.put(`${T}/stories/APP-ST1/issues/${issue.body.key}`, {});
});
afterAll(() => srv?.stop());

describe("drafts", () => {
  it("copies the map into a draft that keeps the numbers and is hidden from project lists", async () => {
    const r = await claude.post(`${T}/drafts`, {});
    expect(r.status).toBe(201);
    assertOperation("createDraft", 201, r.body);
    expect(r.body).toMatchObject({ key: "APPD1", draft_of: "APP", requirements_mode: "quick", name: "App（下書き）" });

    const map = (await owner.get(`${D}/storymap`)).body;
    assertOperation("getStoryMap", 200, map);
    expect(map.draft_of).toBe("APP");
    expect(map.stories.map((s: any) => [s.key, s.title, s.release, s.step_keys, s.issues.length])).toEqual([
      ["APPD1-ST1", "未完了だけ見る", "R1", ["APPD1-A1-1"], 0],
      ["APPD1-ST2", "古い案", "R1", ["APPD1-A1-2"], 0],
    ]);
    expect(map.release_slices.map((x: any) => x.key)).toEqual(["APPD1-RL1"]);

    const projects = (await owner.get("/api/v1/projects")).body.projects.map((p: any) => p.key);
    expect(projects).toContain("APP");
    expect(projects).not.toContain("APPD1");
    const drafts = await owner.get(`${T}/drafts`);
    assertOperation("listDrafts", 200, drafts.body);
    expect(drafts.body.drafts.map((p: any) => p.key)).toEqual(["APPD1"]);
    expect((await owner.get(`${T}/storymap`)).body.draft_of).toBeNull();

    // A fresh copy has nothing to import.
    const p = await plan();
    expect(p).toMatchObject({ draft_key: "APPD1", target_key: "APP", items: [], target_only: [], conflicts: 0 });
  });

  it("plans creates, updates and archives from what was done in the draft", async () => {
    await claude.patch(`${D}/stories/APPD1-ST1`, { title: "朝は未完了だけ見る" });
    await claude.delete(`${D}/stories/APPD1-ST2`);
    await claude.patch(`${D}/releases/APPD1-RL1`, { name: "いま" });
    await claude.post(`${D}/activities`, { name: "振り返る", persona_key: "APPD1-P1" });
    await claude.post(`${D}/activities/APPD1-A2/steps`, { name: "終わったものを見る" });
    const st = await claude.post(`${D}/stories`, { title: "週の終わりに見る", release: "つぎ", step_keys: ["APPD1-A2-1"] });
    expect(st.body.key).toBe("APPD1-ST3"); // numbering continues from the target's
    await claude.post(`${D}/stories/APPD1-ST3/criteria`, { then: "今週終わったものが出る" });
    await claude.post(`${D}/stories`, { title: "置き場所はあとで" });

    const p = await plan();
    expect(brief(p)).toEqual([
      ["create", "activity", "APPD1-A2", "振り返る", false],
      ["create", "step", "APPD1-A2-1", "終わったものを見る", false],
      ["update", "release", "APP-RL1", "R1", false],
      ["create", "release", "APPD1-RL2", "つぎ", false],
      ["update", "story", "APP-ST1", "未完了だけ見る", false],
      ["archive", "story", "APP-ST2", "古い案", false],
      ["create", "story", "APPD1-ST3", "週の終わりに見る", false],
      ["create", "story", "APPD1-ST4", "置き場所はあとで", false],
      ["create", "criterion", "APPD1-AC2", "今週終わったものが出る", false],
    ]);
    expect(p.items.find((i: any) => i.target_key === "APP-ST1").changes).toEqual([{ field: "タイトル", from: "未完了だけ見る", to: "朝は未完了だけ見る" }]);
    expect(p.items.find((i: any) => i.target_key === "APP-RL1").changes).toEqual([{ field: "名前", from: "R1", to: "いま" }]);
    expect(p.items.filter((i: any) => i.action === "create").every((i: any) => i.target_key === null)).toBe(true);
    expect(p.counts).toEqual({ create: 6, update: 2, archive: 1, restore: 0 });
    // APP is structured: a story without a step lands in 未整理 there.
    expect(p.warnings).toEqual(["新しいストーリーのうち 1 件は手順がありません。APP では「未整理」に入ります"]);
  });

  it("lets only a human import", async () => {
    const r = await claude.post(`${D}/import`, {});
    expect(r.status).toBe(403);
    expect(r.body.error.code).toBe("forbidden");
  });

  it("imports in one go, keeping issue links, and leaves nothing to import afterwards", async () => {
    const r = await owner.post(`${D}/import`, {});
    expect(r.status).toBe(200);
    assertOperation("applyImport", 200, r.body);
    expect(r.body.created).toEqual(["APP-A2", "APP-A2-1", "APP-RL2", "APP-ST3", "APP-ST4", "APP-AC2"]);

    const map = (await owner.get(`${T}/storymap`)).body;
    expect(map.stories.map((s: any) => [s.key, s.title, s.release, s.step_keys, s.issues.length])).toEqual([
      ["APP-ST1", "朝は未完了だけ見る", "いま", ["APP-A1-1"], 1],
      ["APP-ST3", "週の終わりに見る", "つぎ", ["APP-A2-1"], 0],
      ["APP-ST4", "置き場所はあとで", "", [], 0],
    ]);
    expect(map.activities.map((a: any) => [a.key, a.name, a.persona_key, a.steps.map((s: any) => s.key)])).toEqual([
      ["APP-A1", "朝", "APP-P1", ["APP-A1-1", "APP-A1-2"]],
      ["APP-A2", "振り返る", "APP-P1", ["APP-A2-1"]],
    ]);
    expect(map.releases).toEqual(["いま", "つぎ"]);
    const st3 = (await owner.get(`${T}/stories/APP-ST3`)).body;
    expect(st3.criteria.map((c: any) => [c.key, c.then])).toEqual([["APP-AC2", "今週終わったものが出る"]]);
    expect(st3.history[0]).toMatchObject({ action: "created", detail: { from_draft: "APPD1" } });

    expect((await plan()).items).toEqual([]);
  });

  it("keeps target-only elements and target changes the draft did not touch", async () => {
    await owner.post(`${T}/stories`, { title: "本番で足した", release: "いま" });
    await owner.patch(`${T}/stories/APP-ST3`, { i_want: "本番で書いた" });
    const p = await plan();
    expect(p.items).toEqual([]);
    expect(p.target_only).toEqual([{ kind: "story", key: "APP-ST5", label: "本番で足した" }]);
  });

  it("stops on a conflict unless forced", async () => {
    await owner.patch(`${T}/stories/APP-ST1`, { title: "本番で直した" });
    await claude.patch(`${D}/stories/APPD1-ST1`, { title: "下書きで直した" });
    const p = await plan();
    expect(brief(p)).toEqual([["update", "story", "APP-ST1", "本番で直した", true]]);
    expect(p.conflicts).toBe(1);

    const stop = await owner.post(`${D}/import`, {});
    expect(stop.status).toBe(409);
    expect(stop.body.error.code).toBe("conflict");
    expect((await owner.get(`${T}/stories/APP-ST1`)).body.title).toBe("本番で直した");

    const forced = await owner.post(`${D}/import`, { force: true });
    expect(forced.status).toBe(200);
    expect((await owner.get(`${T}/stories/APP-ST1`)).body.title).toBe("下書きで直した");
    expect((await plan()).items).toEqual([]);
  });

  it("matches a slice by name when an empty draft names one the target has", async () => {
    const r = await owner.post(`${T}/drafts`, { from: "empty", key: "APPX" });
    expect(r.status).toBe(201);
    await owner.post("/api/v1/projects/APPX/stories", { title: "空から書いた", release: "いま" });
    const p = (await owner.get("/api/v1/projects/APPX/import-plan")).body;
    expect(brief(p)).toEqual([["create", "story", "APPX-ST1", "空から書いた", false]]);
    expect(p.target_only.map((x: any) => x.key)).toContain("APP-ST1");
    const done = await owner.post("/api/v1/projects/APPX/import", {});
    expect(done.status).toBe(200);
    expect(done.body.created).toEqual(["APP-ST6"]);
    expect((await owner.get(`${T}/storymap`)).body.releases).toEqual(["いま", "つぎ"]);
  });

  it("rejects a draft of a draft, a plan of a normal project and a bad source", async () => {
    for (const [res, code, status] of [
      [await owner.post(`${D}/drafts`, {}), "invalid_argument", 400],
      [await owner.get(`${T}/import-plan`), "invalid_argument", 400],
      [await owner.post(`${T}/drafts`, { from: "jira" }), "invalid_argument", 400],
      [await owner.post(`${T}/drafts`, { key: "APP" }), "conflict", 409],
      [await owner.get("/api/v1/projects/NOPE/drafts"), "not_found", 404],
    ] as const) {
      expect(res.status).toBe(status);
      expect(res.body.error.code).toBe(code);
    }
  });

  it("drops an archived draft from the list", async () => {
    await owner.patch("/api/v1/projects/APPX", { archived: true });
    expect((await owner.get(`${T}/drafts`)).body.drafts.map((p: any) => p.key)).toEqual(["APPD1"]);
  });
});

describe("drafts: moving things around", () => {
  const S = "/api/v1/projects/SWP";
  const SD = "/api/v1/projects/SWPD1";
  beforeAll(async () => {
    await owner.post("/api/v1/projects", { key: "SWP", name: "Swap", requirements_mode: "quick" });
    await owner.post(`${S}/activities`, { name: "残す" });
    await owner.post(`${S}/activities`, { name: "やめる" });
    await owner.post(`${S}/activities/SWP-A2/steps`, { name: "やめる手順" });
    await owner.post(`${S}/stories`, { title: "一つめ", release: "R1" });
    await owner.post(`${S}/stories`, { title: "二つめ", release: "R2" });
    await owner.post(`${S}/drafts`, {});
  });

  it("archives an activity with its steps, swaps slice names and keeps a slice still in use", async () => {
    await owner.delete(`${SD}/activities/SWPD1-A2`);
    // Swap R1 and R2 by way of a temporary name, as a person would.
    await owner.patch(`${SD}/releases/SWPD1-RL1`, { name: "tmp" });
    await owner.patch(`${SD}/releases/SWPD1-RL2`, { name: "R1" });
    await owner.patch(`${SD}/releases/SWPD1-RL1`, { name: "R2" });
    // A slice deleted in the draft that a target-only story still uses stays.
    await owner.post(`${S}/stories`, { title: "本番だけ", release: "R3" });
    await owner.post(`${SD}/stories`, { title: "R3 へ", release: "R3" });
    const p = (await owner.get(`${SD}/import-plan`)).body;
    // The draft's R3 is matched to the target's R3 by name (nothing to change), so it is not listed.
    expect(p.items.map((i: any) => [i.action, i.kind, i.target_key ?? i.draft_key])).toEqual([
      ["archive", "activity", "SWP-A2"],
      ["update", "release", "SWP-RL1"],
      ["update", "release", "SWP-RL2"],
      ["create", "story", "SWPD1-ST3"],
    ]);
    const r = await owner.post(`${SD}/import`, {});
    expect(r.status).toBe(200);
    const map = (await owner.get(`${S}/storymap`)).body;
    expect(map.activities.map((a: any) => a.key)).toEqual(["SWP-A1"]);
    expect(map.stories.map((s: any) => [s.title, s.release])).toEqual([
      ["一つめ", "R2"], ["二つめ", "R1"], ["本番だけ", "R3"], ["R3 へ", "R3"],
    ]);
    expect((await owner.get(`${SD}/import-plan`)).body.items).toEqual([]);

    // Now delete the slice in the draft while the target still has a story in it.
    await owner.patch(`${SD}/stories/SWPD1-ST3`, { release: "R1" });
    const rl3 = (await owner.get(`${SD}/releases`)).body.releases.find((x: any) => x.name === "R3");
    expect((await owner.delete(`${SD}/releases/${rl3.key}`)).status).toBe(204);
    const p2 = (await owner.get(`${SD}/import-plan`)).body;
    expect(p2.items.map((i: any) => [i.action, i.kind, i.target_key])).toEqual([["archive", "release", "SWP-RL3"], ["update", "story", "SWP-ST4"]]);
    const r2 = await owner.post(`${SD}/import`, {});
    expect(r2.status).toBe(200);
    expect(r2.body.warnings).toEqual(["リリース SWP-RL3（R3）は SWP にまだストーリーが 1 件あるので残しました"]);
    expect((await owner.get(`${S}/storymap`)).body.releases).toEqual(["R2", "R1", "R3"]); // the rows keep their order; only the names swapped
  });
});

describe("drafts over MCP", () => {
  it("lets an agent make a draft, work in it and show the plan", async () => {
    await owner.post("/api/v1/projects", { key: "MCPX", name: "Mcp" });
    await owner.post("/api/v1/projects/MCPX/stories", { title: "既存" });
    const agent = await srv.mcpHttp("claude");
    try {
      const d = await callTool(agent, "create_draft", { project: "MCPX" });
      expect(d).toMatchObject({ key: "MCPXD1", draft_of: "MCPX" });
      await callTool(agent, "create_story", { project: "MCPXD1", title: "AI が足した" });
      const p = await callTool(agent, "get_import_plan", { draft: "MCPXD1" });
      expect(p.items.map((i: any) => [i.action, i.label])).toEqual([["create", "AI が足した"]]);
      expect(agent.getInstructions()).toMatch(/create_draft/);
    } finally {
      await agent.close();
    }
  });
});

describe("drafts: conflicts survive an import in between", () => {
  const K = "/api/v1/projects/KEEP";
  const KD = "/api/v1/projects/KEEPD1";
  beforeAll(async () => {
    await owner.post("/api/v1/projects", { key: "KEEP", name: "Keep" });
    await owner.post(`${K}/stories`, { title: "そのまま", i_want: "はじめ" });
    await owner.post(`${K}/drafts`, {});
  });

  it("still flags a target change after an import that did not touch the element", async () => {
    await owner.patch(`${K}/stories/KEEP-ST1`, { i_want: "本番で書いた" });
    await owner.post(`${KD}/stories`, { title: "関係ないもの" });
    expect((await owner.post(`${KD}/import`, {})).status).toBe(200);

    await owner.patch(`${KD}/stories/KEEPD1-ST1`, { title: "下書きで直した" });
    const p = (await owner.get(`${KD}/import-plan`)).body;
    expect(p.items.map((i: any) => [i.action, i.target_key, i.conflict])).toEqual([["update", "KEEP-ST1", true]]);
    expect((await owner.post(`${KD}/import`, {})).status).toBe(409);
    expect((await owner.post(`${KD}/import`, { force: true })).status).toBe(200);
    expect((await owner.get(`${KD}/import-plan`)).body.items).toEqual([]);
  });

  it("warns before an import when a renamed slice would clash with one only the target has", async () => {
    await owner.post(`${K}/stories`, { title: "R1 のもの", release: "R1" });
    const d = await owner.post(`${K}/drafts`, { key: "KEEPD2" });
    expect(d.status).toBe(201);
    await owner.post(`${K}/releases`, { name: "つぎ" });
    await owner.patch("/api/v1/projects/KEEPD2/releases/KEEPD2-RL1", { name: "つぎ" });
    const p = (await owner.get("/api/v1/projects/KEEPD2/import-plan")).body;
    expect(p.warnings).toContain("リリース「つぎ」は KEEP にもう別にあるので、この名前には変えられません。下書きで別の名前にしてください");
    expect((await owner.post("/api/v1/projects/KEEPD2/import", {})).status).toBe(409);
  });
});
