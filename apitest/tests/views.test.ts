// Saved views (VIEW-05) and project settings: states and labels (PRJ-04).
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { filterParam, startServer, type Client, type Server } from "../src/harness.js";

let srv: Server;
let owner: Client;

beforeAll(async () => {
  srv = await startServer();
  owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
  await owner.post("/api/v1/projects", { key: "OPS", name: "Ops" });
});
afterAll(() => srv?.stop());

describe("saved views", () => {
  it("creates, lists, updates and deletes the caller's views", async () => {
    const filter = { and: [{ field: "assignee", op: "eq", value: "me" }, { field: "state_category", op: "in", value: ["started"] }] };
    const created = await owner.post("/api/v1/views", { name: "自分の進行中", project_key: "APP", filter, display: { view: "map" } });
    expect(created.status).toBe(201);
    assertOperation("createView", created.status, created.body);
    expect(created.body).toMatchObject({ name: "自分の進行中", project_key: "APP", filter, display: { view: "map" }, position: 1 });

    const cross = await owner.post("/api/v1/views", { name: "全プロジェクトの緊急", filter: { field: "priority", op: "eq", value: 1 } });
    expect(cross.body).toMatchObject({ project_key: null, display: {}, position: 2 });

    const list = (await owner.get("/api/v1/views")).body;
    assertOperation("listViews", 200, list);
    expect(list.views.map((v: any) => v.name)).toEqual(["自分の進行中", "全プロジェクトの緊急"]);

    // Views are private to their owner.
    expect((await srv.as("alice").get("/api/v1/views")).body.views).toEqual([]);
    expect((await srv.as("alice").get(`/api/v1/views/${created.body.id}`)).status).toBe(404);
    expect((await srv.as("alice").delete(`/api/v1/views/${created.body.id}`)).status).toBe(404);

    const updated = await owner.patch(`/api/v1/views/${created.body.id}`, { name: "進行中", display: { view: "board", group: "assignee" }, position: 3 });
    expect(updated.status).toBe(200);
    assertOperation("updateView", 200, updated.body);
    expect(updated.body).toMatchObject({ name: "進行中", display: { view: "board", group: "assignee" }, filter });
    expect((await owner.get("/api/v1/views")).body.views.map((v: any) => v.name)).toEqual(["全プロジェクトの緊急", "進行中"]);

    // The stored filter works with the issues API as is.
    const view = (await owner.get(`/api/v1/views/${created.body.id}`)).body;
    expect((await owner.get(`/api/v1/issues?project=APP&filter=${filterParam(view.filter)}`)).status).toBe(200);

    expect((await owner.delete(`/api/v1/views/${created.body.id}`)).status).toBe(204);
    expect((await owner.get(`/api/v1/views/${created.body.id}`)).status).toBe(404);
  });

  it("rejects invalid views", async () => {
    expect((await owner.post("/api/v1/views", { name: " " })).status).toBe(400);
    expect((await owner.post("/api/v1/views", { name: "x", filter: { field: "nope", op: "eq", value: 1 } })).status).toBe(400);
    expect((await owner.post("/api/v1/views", { name: "x", display: [1] })).status).toBe(400);
    expect((await owner.post("/api/v1/views", { name: "x", project_key: "NOPE" })).status).toBe(404);
  });
});

describe("workflow states", () => {
  const states = async (key = "APP") => (await owner.get(`/api/v1/projects/${key}/states`)).body.states.filter((s: any) => s.applies_to === "all");
  const names = async () => (await states()).map((s: any) => s.name);

  it("adds a state at the end of its category", async () => {
    const res = await owner.post("/api/v1/projects/APP/states", { name: "Blocked", category: "started", color: "#e5484d" });
    expect(res.status).toBe(201);
    assertOperation("createState", res.status, res.body);
    expect(await names()).toEqual(["Backlog", "Todo", "In Progress", "In Review", "受入待ち", "Blocked", "Done", "Canceled"]);
    expect((await states()).map((s: any) => s.position)).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
    expect((await owner.post("/api/v1/projects/APP/states", { name: "blocked", category: "started" })).status).toBe(409);
    expect((await owner.post("/api/v1/projects/APP/states", { name: "X", category: "weird" })).status).toBe(400);
  });

  it("renames, recolors and reorders within the category", async () => {
    const blocked = (await states()).find((s: any) => s.name === "Blocked");
    const res = await owner.patch(`/api/v1/projects/APP/states/${blocked.id}`, { name: "保留", color: "#000000", position: 3 });
    expect(res.status).toBe(200);
    assertOperation("updateState", 200, res.body);
    expect(await names()).toEqual(["Backlog", "Todo", "保留", "In Progress", "In Review", "受入待ち", "Done", "Canceled"]);
    // A position outside the category is clamped to it.
    await owner.patch(`/api/v1/projects/APP/states/${blocked.id}`, { position: 1 });
    expect(await names()).toEqual(["Backlog", "Todo", "保留", "In Progress", "In Review", "受入待ち", "Done", "Canceled"]);
    await owner.patch(`/api/v1/projects/APP/states/${blocked.id}`, { position: 99 });
    expect(await names()).toEqual(["Backlog", "Todo", "In Progress", "In Review", "受入待ち", "保留", "Done", "Canceled"]);

    // Jira transitions follow the workflow.
    const issue = (await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "t" })).body.key;
    const tr = (await srv.jira("owner").get(`/rest/api/3/issue/${issue}/transitions`)).body.transitions.map((t: any) => t.name);
    expect(tr).toContain("保留");
  });

  it("moves the default and acceptance flags", async () => {
    const all = await states();
    const backlog = all.find((s: any) => s.name === "Backlog");
    expect((await owner.patch(`/api/v1/projects/APP/states/${backlog.id}`, { is_default: true })).status).toBe(200);
    expect((await states()).filter((s: any) => s.is_default).map((s: any) => s.name)).toEqual(["Backlog"]);
    const issue = (await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "new" })).body;
    expect(issue.state.name).toBe("Backlog");

    const review = all.find((s: any) => s.name === "In Review");
    expect((await owner.patch(`/api/v1/projects/APP/states/${review.id}`, { is_acceptance: true })).status).toBe(200);
    expect((await states()).filter((s: any) => s.is_acceptance).map((s: any) => s.name)).toEqual(["In Review"]);
    const done = all.find((s: any) => s.name === "Done");
    expect((await owner.patch(`/api/v1/projects/APP/states/${done.id}`, { is_acceptance: true })).status).toBe(400);
    expect((await owner.patch(`/api/v1/projects/APP/states/${done.id}`, { is_default: true })).status).toBe(400);
  });

  it("deletes only unused, non-essential states", async () => {
    const all = await states();
    const id = (n: string) => all.find((s: any) => s.name === n).id;
    const del = (n: string) => owner.delete(`/api/v1/projects/APP/states/${id(n)}`);
    expect((await del("Backlog")).body.error.message).toMatch(/issue\(s\) are in/); // in use (and default)
    expect((await del("In Review")).body.error.message).toMatch(/acceptance state/);
    expect((await del("Done")).body.error.message).toMatch(/last completed state/);
    expect((await del("保留")).status).toBe(204);
    expect(await names()).not.toContain("保留");
    expect((await states()).map((s: any) => s.position)).toEqual([1, 2, 3, 4, 5, 6, 7]);
    // QA states are not editable here.
    const qaState = (await owner.get("/api/v1/projects/APP/states")).body.states.find((s: any) => s.applies_to === "qa");
    expect((await owner.delete(`/api/v1/projects/APP/states/${qaState.id}`)).status).toBe(404);
    // A state of another project cannot be reached through this one.
    const ops = (await states("OPS"))[0];
    expect((await owner.patch(`/api/v1/projects/APP/states/${ops.id}`, { name: "x" })).status).toBe(404);
  });

  it("toggles skipping hierarchy levels", async () => {
    const epic = (await owner.post("/api/v1/issues", { project_key: "OPS", type: "epic", title: "E" })).body.key;
    await owner.patch("/api/v1/projects/OPS", { allow_skip_levels: false });
    expect((await owner.post("/api/v1/issues", { project_key: "OPS", type: "task", title: "T", parent_key: epic })).status).toBe(422);
    await owner.patch("/api/v1/projects/OPS", { allow_skip_levels: true });
    expect((await owner.post("/api/v1/issues", { project_key: "OPS", type: "task", title: "T", parent_key: epic })).status).toBe(201);
  });
});

describe("labels", () => {
  it("renames, recolors and deletes labels", async () => {
    const issue = (await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "labelled", labels: ["ui", "backend"] })).body.key;
    const labels = (await owner.get("/api/v1/projects/APP/labels")).body.labels;
    const ui = labels.find((l: any) => l.name === "ui");
    const res = await owner.patch(`/api/v1/projects/APP/labels/${ui.id}`, { name: "frontend", color: "#3e63dd" });
    expect(res.status).toBe(200);
    assertOperation("updateLabel", 200, res.body);
    expect(res.body).toEqual({ id: ui.id, name: "frontend", color: "#3e63dd" });
    expect((await owner.patch(`/api/v1/projects/APP/labels/${ui.id}`, { name: "backend" })).status).toBe(409);

    expect((await owner.delete(`/api/v1/projects/APP/labels/${ui.id}`)).status).toBe(204);
    expect((await owner.get(`/api/v1/issues/${issue}`)).body.labels.map((l: any) => l.name)).toEqual(["backend"]);
    expect((await owner.delete(`/api/v1/projects/APP/labels/${ui.id}`)).status).toBe(404);
  });
});
