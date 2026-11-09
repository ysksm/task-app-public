import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { startServer, type Server } from "../src/harness.js";

let srv: Server;
beforeAll(async () => {
  srv = await startServer();
});
afterAll(() => srv?.stop());

describe("projects", () => {
  it("creates a project with default workflow states", async () => {
    const res = await srv.as("owner").post("/api/v1/projects", { key: "app", name: "App" });
    expect(res.status).toBe(201);
    expect(res.body).toMatchObject({ key: "APP", name: "App", allow_skip_levels: true, tier: "normal", archived_at: null });
    expect(res.body.owner.username).toBe("owner");

    const states = (await srv.as("owner").get("/api/v1/projects/APP/states")).body.states;
    const names = (applies: string) => states.filter((s: any) => s.applies_to === applies).map((s: any) => s.name);
    expect(names("all")).toEqual(["Backlog", "Todo", "In Progress", "In Review", "受入待ち", "Done", "Canceled"]);
    expect(names("qa")).toEqual(["未回答", "回答済", "クローズ"]);
    const acceptance = states.find((s: any) => s.is_acceptance);
    expect(acceptance).toMatchObject({ name: "受入待ち", category: "started" });
  });

  it("rejects duplicate and invalid keys", async () => {
    expect((await srv.as("owner").post("/api/v1/projects", { key: "APP", name: "Dup" })).status).toBe(409);
    for (const key of ["A", "1AB", "TOO-LONG", "ABCDEFGHIJK"]) {
      const res = await srv.as("owner").post("/api/v1/projects", { key, name: "x" });
      expect(res.status, key).toBe(400);
      expect(res.body.error.code).toBe("invalid_argument");
    }
    expect((await srv.as("owner").post("/api/v1/projects", { key: "ZZ", name: " " })).status).toBe(400);
  });

  it("rejects unknown fields in request bodies", async () => {
    const res = await srv.as("owner").post("/api/v1/projects", { key: "XX", name: "x", bogus: 1 });
    expect(res.status).toBe(400);
  });

  it("returns 404 for unknown projects", async () => {
    const res = await srv.as("owner").get("/api/v1/projects/NOPE");
    expect(res.status).toBe(404);
    expect(res.body.error.code).toBe("not_found");
  });

  it("updates settings and hides archived projects from the list", async () => {
    await srv.as("owner").post("/api/v1/projects", { key: "OLD", name: "Old" });
    const upd = await srv.as("owner").patch("/api/v1/projects/OLD", { name: "Old one", allow_skip_levels: false, archived: true });
    expect(upd.status).toBe(200);
    expect(upd.body).toMatchObject({ name: "Old one", allow_skip_levels: false });
    expect(upd.body.archived_at).not.toBeNull();

    const keys = (await srv.as("owner").get("/api/v1/projects")).body.projects.map((p: any) => p.key);
    expect(keys).toContain("APP");
    expect(keys).not.toContain("OLD");
    const all = (await srv.as("owner").get("/api/v1/projects?include_archived=true")).body.projects.map((p: any) => p.key);
    expect(all).toContain("OLD");
  });

  it("sets the tier (focus / normal / side) on create and update", async () => {
    const me = srv.as("owner");
    const created = await me.post("/api/v1/projects", { key: "TIER", name: "Tier", tier: "focus" });
    expect(created.status).toBe(201);
    expect(created.body.tier).toBe("focus");
    const upd = await me.patch("/api/v1/projects/TIER", { tier: "side" });
    expect(upd.status).toBe(200);
    expect(upd.body.tier).toBe("side");
    expect((await me.get("/api/v1/projects")).body.projects.find((p: any) => p.key === "TIER").tier).toBe("side");
    for (const res of [await me.post("/api/v1/projects", { key: "BADT", name: "x", tier: "low" }), await me.patch("/api/v1/projects/TIER", { tier: "" })]) {
      expect(res.status).toBe(400);
      expect(res.body.error.code).toBe("invalid_argument");
    }
  });

  it("manages labels", async () => {
    const res = await srv.as("owner").post("/api/v1/projects/APP/labels", { name: "frontend", color: "#123456" });
    expect(res.status).toBe(201);
    expect((await srv.as("owner").post("/api/v1/projects/APP/labels", { name: "frontend" })).status).toBe(409);
    const labels = (await srv.as("owner").get("/api/v1/projects/APP/labels")).body.labels;
    expect(labels).toEqual([{ id: res.body.id, name: "frontend", color: "#123456" }]);
  });

  it("reports issue stats per project (QA excluded from progress)", async () => {
    const me = srv.as("owner");
    await me.post("/api/v1/projects", { key: "STAT", name: "Stats" });
    const states = (await me.get("/api/v1/projects/STAT/states")).body.states;
    const done = states.find((s: any) => s.name === "Done").id;
    await me.post("/api/v1/issues", { project_key: "STAT", type: "task", title: "a", state_id: done });
    await me.post("/api/v1/issues", { project_key: "STAT", type: "task", title: "b" });
    await me.post("/api/v1/issues", { project_key: "STAT", type: "qa", title: "q" });
    const p = (await me.get("/api/v1/projects/STAT")).body;
    expect(p.stats).toEqual({ total: 2, done: 1, by_type: { task: 2, qa: 1 } });
  });
});
