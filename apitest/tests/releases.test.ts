// Release slices (docs/08-devcycle.md §5.8): the story map's rows with an
// outcome (goal), an order and a status. Stories keep naming their slice by
// text; writing an unknown name on a story creates the slice.
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

describe("release slices", () => {
  it("creates slices in order with a goal and a status", async () => {
    const r1 = await owner.post(`${base}/releases`, { name: "R1", goal: "ログインして一覧を読める", status: "building" });
    expect(r1.status).toBe(201);
    assertOperation("createRelease", 201, r1.body);
    expect(r1.body).toMatchObject({ key: "APP-RL1", name: "R1", goal: "ログインして一覧を読める", status: "building", metric: "", position: 1, stories: 0, done: 0 });
    const r2 = await owner.post(`${base}/releases`, { name: "R2" });
    expect(r2.body).toMatchObject({ key: "APP-RL2", status: "planned", position: 2 });
    const list = await owner.get(`${base}/releases`);
    assertOperation("listReleases", 200, list.body);
    expect(list.body.releases.map((r: any) => r.name)).toEqual(["R1", "R2"]);
  });

  it("rejects a missing name, a duplicate name and an unknown status", async () => {
    for (const [res, code, status] of [
      [await owner.post(`${base}/releases`, { name: " " }), "invalid_argument", 400],
      [await owner.post(`${base}/releases`, { name: "R1" }), "conflict", 409],
      [await owner.post(`${base}/releases`, { name: "R9", status: "done" }), "invalid_argument", 400],
      [await owner.patch(`${base}/releases/APP-RL1`, { status: "" }), "invalid_argument", 400],
      [await owner.patch(`${base}/releases/APP-RL1`, { name: "R2" }), "conflict", 409],
      [await owner.patch(`${base}/releases/APP-RL99`, { goal: "x" }), "not_found", 404],
    ] as const) {
      expect(res.status).toBe(status);
      expect(res.body.error.code).toBe(code);
    }
  });

  it("creates a slice from a name written on a story, and counts the stories", async () => {
    const st = await owner.post(`${base}/stories`, { title: "あとで", release: "いつか" });
    expect(st.status).toBe(201);
    await owner.post(`${base}/stories`, { title: "R1 のもの", release: "R1" });
    const map = (await owner.get(`${base}/storymap`)).body;
    assertOperation("getStoryMap", 200, map);
    expect(map.releases).toEqual(["R1", "R2", "いつか"]);
    expect(map.release_slices.map((r: any) => [r.key, r.name, r.status, r.stories, r.done])).toEqual([
      ["APP-RL1", "R1", "building", 1, 0], ["APP-RL2", "R2", "planned", 0, 0], ["APP-RL3", "いつか", "planned", 1, 0],
    ]);
    // Moving a story to another name creates that one too.
    await owner.patch(`${base}/stories/${st.body.key}`, { release: "R3" });
    expect((await owner.get(`${base}/releases`)).body.releases.map((r: any) => r.name)).toEqual(["R1", "R2", "いつか", "R3"]);
  });

  it("renames a slice on its stories and reorders by position", async () => {
    const ren = await owner.patch(`${base}/releases/RL1`, { name: "MVP", metric: "毎朝使っている" });
    expect(ren.status).toBe(200);
    assertOperation("updateRelease", 200, ren.body);
    expect(ren.body).toMatchObject({ key: "APP-RL1", name: "MVP", metric: "毎朝使っている", stories: 1 });
    const stories = (await owner.get(`${base}/stories`)).body.stories;
    expect(stories.find((s: any) => s.title === "R1 のもの").release).toBe("MVP");

    await owner.patch(`${base}/releases/APP-RL2`, { position: 0 });
    expect((await owner.get(`${base}/storymap`)).body.releases).toEqual(["R2", "MVP", "いつか", "R3"]);
  });

  it("deletes only an empty slice", async () => {
    const busy = await owner.delete(`${base}/releases/APP-RL1`);
    expect(busy.status).toBe(409);
    expect(busy.body.error.code).toBe("conflict");
    expect((await owner.delete(`${base}/releases/APP-RL3`)).status).toBe(204);
    expect((await owner.get(`${base}/releases`)).body.releases.map((r: any) => r.key)).toEqual(["APP-RL2", "APP-RL1", "APP-RL4"]);
    // Keys are not reused.
    expect((await owner.post(`${base}/releases`, { name: "次" })).body.key).toBe("APP-RL5");
  });

  it("lets the agent set a goal and a status through MCP update_release", async () => {
    const claude = await srv.mcpHttp("claude");
    const r = await callTool(claude, "update_release", { project: "APP", name: "R3", goal: "週末に滞りが見える", status: "building" });
    expect(r).toMatchObject({ key: "APP-RL4", name: "R3", goal: "週末に滞りが見える", status: "building" });
    const made = await callTool(claude, "update_release", { project: "APP", name: "あとで", goal: "いつか" });
    expect(made).toMatchObject({ name: "あとで", status: "planned", goal: "いつか" });
    const map = await callTool(claude, "get_storymap", { project: "APP" });
    expect(map.release_slices.find((x: any) => x.name === "R3").goal).toBe("週末に滞りが見える");
    await expect(callTool(claude, "update_release", { project: "APP", name: "R3", status: "finished" })).rejects.toThrow(/status/);
  });
});
