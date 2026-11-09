import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { filterParam, startServer, type Server, type Who } from "../src/harness.js";

let srv: Server;
let states: Record<string, number>;

// Fixed tree:
// APP-1 epic (owner)
//   APP-2 story (owner)
//     APP-3 feature (claude, label ui)
//       APP-4 task (claude, Done)
//       APP-5 task (codex, label ui, label api)
//         APP-6 task (codex, text "needle")
//       APP-7 qa (owner, reported by claude)
// APP-8 task (alice, priority 1)
// OTH-1 task (claude) in another project
beforeAll(async () => {
  srv = await startServer();
  const owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
  await owner.post("/api/v1/projects", { key: "OTH", name: "Other", tier: "side" });
  states = Object.fromEntries((await owner.get("/api/v1/projects/APP/states")).body.states.map((s: any) => [s.name, s.id]));
  const mk = (who: Who, body: Record<string, unknown>) => srv.as(who).post("/api/v1/issues", { project_key: "APP", ...body });
  await mk("owner", { type: "epic", title: "Epic", assignee: "owner" });
  await mk("owner", { type: "story", title: "Story", parent_key: "APP-1", assignee: "owner" });
  await mk("owner", { type: "feature", title: "Feature", parent_key: "APP-2", assignee: "claude", labels: ["ui"] });
  await mk("owner", { type: "task", title: "Done task", parent_key: "APP-3", assignee: "claude", state_id: states["Done"] });
  await mk("owner", { type: "task", title: "API task", parent_key: "APP-3", assignee: "codex", labels: ["ui", "api"] });
  await mk("owner", { type: "task", title: "Deep", description: "find the needle here", parent_key: "APP-5", assignee: "codex" });
  await mk("claude", { type: "qa", title: "Question", parent_key: "APP-3", assignee: "owner" });
  await mk("owner", { type: "task", title: "Alice task", assignee: "alice", priority: 1 });
  await srv.as("owner").post("/api/v1/issues", { project_key: "OTH", type: "task", title: "Other", assignee: "claude" });
});
afterAll(() => srv?.stop());

async function keys(filter: unknown, who: Who = "owner", extra = ""): Promise<string[]> {
  const res = await srv.as(who).get(`/api/v1/issues?sort=key&filter=${filterParam(filter)}${extra}`);
  expect(res.status, JSON.stringify(res.body)).toBe(200);
  return res.body.issues.map((i: any) => i.key);
}

describe("issue filters", () => {
  it("lists everything across projects without a filter", async () => {
    const res = await srv.as("owner").get("/api/v1/issues?sort=key");
    expect(res.body.total).toBe(9);
    expect(res.body.issues[0].key).toBe("APP-1");
    expect(res.body.issues.at(-1).key).toBe("OTH-1");
  });

  it("filters by project shortcut and field", async () => {
    const res = await srv.as("owner").get("/api/v1/issues?project=OTH");
    expect(res.body.issues.map((i: any) => i.key)).toEqual(["OTH-1"]);
    expect(await keys({ field: "project", op: "eq", value: "oth" })).toEqual(["OTH-1"]);
  });

  it("filters by project tier", async () => {
    expect(await keys({ field: "project_tier", op: "eq", value: "side" })).toEqual(["OTH-1"]);
    expect(await keys({ field: "project_tier", op: "in", value: ["focus", "side"] })).toEqual(["OTH-1"]);
    expect((await keys({ field: "project_tier", op: "neq", value: "side" })).every((k) => k.startsWith("APP-"))).toBe(true);
    const res = await srv.as("owner").get(`/api/v1/issues?filter=${filterParam({ field: "project_tier", op: "eq", value: "low" })}`);
    expect(res.status).toBe(400);
    expect(res.body.error.code).toBe("invalid_argument");
  });

  it("filters by type", async () => {
    expect(await keys({ field: "type", op: "in", value: ["epic", "story"] })).toEqual(["APP-1", "APP-2"]);
    expect(await keys({ field: "type", op: "eq", value: "qa" })).toEqual(["APP-7"]);
  });

  it("resolves 'me' to the caller (my tasks)", async () => {
    expect(await keys({ field: "assignee", op: "eq", value: "me" }, "claude")).toEqual(["APP-3", "APP-4", "OTH-1"]);
    expect(await keys({ field: "reporter", op: "eq", value: "me" }, "claude")).toEqual(["APP-7"]);
    expect(await keys({ field: "assignee", op: "in", value: ["alice", "codex"] })).toEqual(["APP-5", "APP-6", "APP-8"]);
  });

  it("filters by assignee kind and unassigned", async () => {
    expect(await keys({ and: [{ field: "assignee_kind", op: "eq", value: "agent" }, { field: "project", op: "eq", value: "APP" }] }))
      .toEqual(["APP-3", "APP-4", "APP-5", "APP-6"]);
    expect(await keys({ field: "assignee", op: "empty" })).toEqual([]);
  });

  it("filters by state and state category", async () => {
    expect(await keys({ field: "state_category", op: "eq", value: "completed" })).toEqual(["APP-4"]);
    expect(await keys({ field: "state", op: "eq", value: states["Done"] })).toEqual(["APP-4"]);
    const open = await keys({ field: "state_category", op: "not_in", value: ["completed", "canceled"] });
    expect(open).not.toContain("APP-4");
    expect(open).toHaveLength(8);
  });

  it("filters by label", async () => {
    expect(await keys({ field: "label", op: "eq", value: "ui" })).toEqual(["APP-3", "APP-5"]);
    expect(await keys({ field: "label", op: "in", value: ["api"] })).toEqual(["APP-5"]);
    expect(await keys({ and: [{ field: "label", op: "not_empty" }, { not: { field: "label", op: "eq", value: "api" } }] })).toEqual(["APP-3"]);
  });

  it("filters by parent and ancestor", async () => {
    expect(await keys({ field: "parent", op: "eq", value: "APP-3" })).toEqual(["APP-4", "APP-5", "APP-7"]);
    expect(await keys({ field: "ancestor", op: "eq", value: "APP-2" })).toEqual(["APP-3", "APP-4", "APP-5", "APP-6", "APP-7"]);
    expect(await keys({ and: [{ field: "parent", op: "empty" }, { field: "project", op: "eq", value: "APP" }] })).toEqual(["APP-1", "APP-8"]);
  });

  it("searches text in title, description and key", async () => {
    expect(await keys({ field: "text", op: "contains", value: "needle" })).toEqual(["APP-6"]);
    expect(await keys({ field: "text", op: "contains", value: "app-8" })).toEqual(["APP-8"]);
    expect(await keys({ field: "text", op: "contains", value: "100%" })).toEqual([]);
  });

  it("supports or/not and priority comparisons", async () => {
    expect(await keys({ or: [{ field: "priority", op: "eq", value: 1 }, { field: "type", op: "eq", value: "epic" }] })).toEqual(["APP-1", "APP-8"]);
    expect(await keys({ and: [{ field: "priority", op: "gt", value: 0 }] })).toEqual(["APP-8"]);
  });

  it("returns missing ancestors for the tree view", async () => {
    const res = await srv.as("owner").get(`/api/v1/issues?include_ancestors=true&filter=${filterParam({ field: "key", op: "eq", value: "APP-6" })}`);
    expect(res.body.issues.map((i: any) => i.key)).toEqual(["APP-6"]);
    expect(res.body.ancestors.map((i: any) => i.key).sort()).toEqual(["APP-1", "APP-2", "APP-3", "APP-5"]);
  });

  it("paginates with a total", async () => {
    const res = await srv.as("owner").get("/api/v1/issues?sort=key&limit=3&offset=3");
    expect(res.body.total).toBe(9);
    expect(res.body.issues.map((i: any) => i.key)).toEqual(["APP-4", "APP-5", "APP-6"]);
  });

  it("rejects invalid filters", async () => {
    for (const f of [
      { field: "nope", op: "eq", value: "x" },
      { field: "type", op: "contains", value: "x" },
      { field: "type", op: "in", value: "story" },
      { field: "type", op: "eq", value: "bug" },
      { and: [], field: "type", op: "eq", value: "task" },
    ]) {
      const res = await srv.as("owner").get(`/api/v1/issues?filter=${filterParam(f)}`);
      expect(res.status, JSON.stringify(f)).toBe(400);
      expect(res.body.error.code).toBe("invalid_argument");
    }
    expect((await srv.as("owner").get("/api/v1/issues?filter=%7Bbad")).status).toBe(400);
    expect((await srv.as("owner").get("/api/v1/issues?sort=random")).status).toBe(400);
  });
});
