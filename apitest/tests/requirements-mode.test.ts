// Option E (docs/09-requirements-loose.md §6): each project picks how it writes
// requirements. requirements_mode = structured (option A) | quick (option D).
// The data and the API rules are the same in both modes; the mode only tells
// the screens and the agents how strictly to write.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { callTool, startServer, type Client, type Server } from "../src/harness.js";

let srv: Server;
let owner: Client;

beforeAll(async () => {
  srv = await startServer();
  owner = srv.as("owner");
});
afterAll(() => srv?.stop());

describe("requirements_mode", () => {
  it("defaults new projects to quick and can be chosen on create", async () => {
    const q = await owner.post("/api/v1/projects", { key: "QK", name: "Quick" });
    expect(q.status).toBe(201);
    assertOperation("createProject", 201, q.body);
    expect(q.body.requirements_mode).toBe("quick");
    const s = await owner.post("/api/v1/projects", { key: "ST", name: "Structured", requirements_mode: "structured" });
    expect(s.status).toBe(201);
    expect(s.body.requirements_mode).toBe("structured");
    const list = (await owner.get("/api/v1/projects")).body.projects;
    expect(Object.fromEntries(list.map((p: any) => [p.key, p.requirements_mode]))).toMatchObject({ QK: "quick", ST: "structured" });
  });

  it("switches with PATCH and rejects unknown modes", async () => {
    const upd = await owner.patch("/api/v1/projects/QK", { requirements_mode: "structured" });
    expect(upd.status).toBe(200);
    assertOperation("updateProject", 200, upd.body);
    expect(upd.body.requirements_mode).toBe("structured");
    expect((await owner.get("/api/v1/projects/QK")).body.requirements_mode).toBe("structured");
    for (const res of [
      await owner.post("/api/v1/projects", { key: "BAD", name: "x", requirements_mode: "loose" }),
      await owner.patch("/api/v1/projects/QK", { requirements_mode: "" }),
      await owner.patch("/api/v1/projects/QK", { requirements_mode: "strict" }),
    ]) {
      expect(res.status).toBe(400);
      expect(res.body.error.code).toBe("invalid_argument");
    }
    expect((await owner.patch("/api/v1/projects/QK", { requirements_mode: "quick" })).body.requirements_mode).toBe("quick");
  });

  it("returns the mode with the story map, over REST and MCP", async () => {
    const rest = (await owner.get("/api/v1/projects/ST/storymap")).body;
    assertOperation("getStoryMap", 200, rest);
    expect(rest.mode).toBe("structured");
    expect((await owner.get("/api/v1/projects/QK/storymap")).body.mode).toBe("quick");
    const claude = await srv.mcpHttp("claude");
    expect((await callTool(claude, "get_storymap", { project: "ST" })).mode).toBe("structured");
  });

  it("keeps the data and the rules the same in both modes", async () => {
    // A one-line, unsorted story is accepted in a structured project too; it shows up as a gap.
    const st = await owner.post("/api/v1/projects/ST/stories", { title: "ひとことだけ", body: "メモ" });
    expect(st.status).toBe(201);
    const c = await owner.post(`/api/v1/projects/ST/stories/${st.body.key}/criteria`, { then: "結果だけ" });
    expect(c.status).toBe(201);
    let map = (await owner.get("/api/v1/projects/ST/storymap")).body;
    expect(map.gaps.stories_without_steps).toEqual([st.body.key]);
    // Switching the mode changes nothing in the data.
    await owner.patch("/api/v1/projects/ST", { requirements_mode: "quick" });
    map = (await owner.get("/api/v1/projects/ST/storymap")).body;
    expect(map.mode).toBe("quick");
    expect(map.stories).toEqual([expect.objectContaining({ key: st.body.key, body: "メモ", step_keys: [], criteria_count: 1 })]);
    expect(map.gaps.stories_without_steps).toEqual([st.body.key]);
  });
});
