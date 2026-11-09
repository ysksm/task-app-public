// MCP tools that build the story map itself (docs/08-devcycle.md §5.10): personas,
// activities with their steps, and stories placed under them, so an agent can lay
// out a map (usually in a draft) without the REST API.
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

describe("MCP create_persona / create_activity / create_step", () => {
  it("lays out a map: a persona, an activity with steps, and a story for the persona under a step", async () => {
    const claude = await srv.mcpHttp("claude");
    const p = await callTool(claude, "create_persona", { project: "APP", name: "自分", description: "朝型にしたい" });
    expect(p).toMatchObject({ key: "APP-P1", name: "自分", kind: "human", description: "朝型にしたい" });

    const a = await callTool(claude, "create_activity", {
      project: "APP", name: "朝", persona_key: "APP-P1", steps: [{ name: "起きる" }, { name: "家を出る", description: "7:00" }],
    });
    expect(a).toMatchObject({ key: "APP-A1", name: "朝", persona_key: "APP-P1" });
    expect(a.steps.map((s: any) => [s.key, s.name, s.description])).toEqual([["APP-A1-1", "起きる", ""], ["APP-A1-2", "家を出る", "7:00"]]);
    const later = await callTool(claude, "create_step", { project: "APP", activity: "APP-A1", name: "会社に着く" });
    expect(later).toMatchObject({ key: "APP-A1-3", name: "会社に着く" });

    const st = await callTool(claude, "create_story", {
      project: "APP", title: "起きた時刻を記録する", persona_key: "APP-P1", i_want: "起きた時刻を残したい", so_that: "朝型か見るため",
      step_keys: ["APP-A1-1"], release: "R1", criteria: [{ given: "朝", when: "「起きた」を押す", then: "時刻が残る" }],
    });
    expect(st).toMatchObject({ persona_key: "APP-P1", i_want: "起きた時刻を残したい", so_that: "朝型か見るため", step_keys: ["APP-A1-1"], release: "R1" });

    const map = (await owner.get(`${base}/storymap`)).body;
    assertOperation("getStoryMap", 200, map);
    expect(map.personas.map((x: any) => x.key)).toEqual(["APP-P1"]);
    expect(map.activities[0].steps.map((s: any) => s.name)).toEqual(["起きる", "家を出る", "会社に着く"]);
    expect(map.gaps.steps_without_stories).toEqual(["APP-A1-2", "APP-A1-3"]);
  });

  it("updates them: rename, move and drop the persona", async () => {
    const claude = await srv.mcpHttp("claude");
    await callTool(claude, "create_activity", { project: "APP", name: "夜" });
    expect(await callTool(claude, "update_persona", { project: "APP", key: "APP-P1", name: "わたし", description: "定時に帰りたい" }))
      .toMatchObject({ key: "APP-P1", name: "わたし", description: "定時に帰りたい" });
    expect(await callTool(claude, "update_activity", { project: "APP", key: "APP-A2", name: "夜の時間", position: 0, persona_key: "APP-P1" }))
      .toMatchObject({ key: "APP-A2", name: "夜の時間", persona_key: "APP-P1" });
    expect(await callTool(claude, "update_activity", { project: "APP", key: "APP-A2", persona_key: "" })).toMatchObject({ persona_key: null });
    expect(await callTool(claude, "update_step", { project: "APP", key: "APP-A1-3", name: "会社に 7:45 に着く", position: 0 }))
      .toMatchObject({ key: "APP-A1-3", name: "会社に 7:45 に着く" });

    const map = (await owner.get(`${base}/storymap`)).body;
    expect(map.activities.map((x: any) => x.key)).toEqual(["APP-A2", "APP-A1"]);
    expect(map.activities[1].steps.map((s: any) => s.key)).toEqual(["APP-A1-3", "APP-A1-1", "APP-A1-2"]);
  });

  it("returns mistakes as tool errors", async () => {
    const claude = await srv.mcpHttp("claude");
    const noStep: any = await claude.callTool({ name: "create_step", arguments: { project: "APP", activity: "APP-A9", name: "x" } });
    expect(noStep.isError).toBe(true);
    const noPersona: any = await claude.callTool({ name: "create_activity", arguments: { project: "APP", name: "x", persona_key: "APP-P9" } });
    expect(noPersona.isError).toBe(true);
  });
});
