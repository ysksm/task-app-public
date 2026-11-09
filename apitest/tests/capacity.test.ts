import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { startServer, type Server } from "../src/harness.js";

// Capacity section of the settings page: counts and storage (owner only).
let srv: Server;
beforeAll(async () => {
  srv = await startServer(); // the test preset has users but no projects
  await srv.as("owner").post("/api/v1/projects", { key: "CAP", name: "capacity" });
  await srv.as("owner").post("/api/v1/projects", { key: "BIG", name: "bigger" });
  for (const t of ["a", "b"]) await srv.as("owner").post("/api/v1/issues", { project_key: "BIG", type: "task", title: t });
});
afterAll(() => srv?.stop());

describe("capacity", () => {
  it("reports counts, storage and memory to the owner", async () => {
    const before = (await srv.as("owner").get("/api/v1/capacity")).body;
    const res = await srv.as("owner").get("/api/v1/capacity");
    expect(res.status).toBe(200);
    assertOperation("capacity", 200, res.body);
    const { counts, storage, memory } = res.body;
    expect(counts).toMatchObject({ projects: 2, issues: 2, open_issues: 2, largest_project: "BIG", largest_project_issues: 2, users: 4 });
    expect(counts.issues).toBeGreaterThanOrEqual(counts.open_issues);
    expect(counts.largest_project_issues).toBeLessThanOrEqual(counts.issues);
    expect(storage.db_bytes).toBeGreaterThan(0);
    expect(storage.data_dir_bytes).toBeGreaterThanOrEqual(storage.db_bytes);
    expect(storage.disk_total_bytes).toBeGreaterThan(storage.disk_free_bytes); // unix test machines
    expect(memory.sys_bytes).toBeGreaterThan(0);

    // A new issue shows up in the counts, a deleted one does not.
    const key = (await srv.as("owner").post("/api/v1/issues", { project_key: counts.largest_project, type: "task", title: "count me" })).body.key;
    const after = (await srv.as("owner").get("/api/v1/capacity")).body.counts;
    expect(after.issues).toBe(before.counts.issues + 1);
    expect(after.open_issues).toBe(before.counts.open_issues + 1);
    expect((await srv.as("owner").delete(`/api/v1/issues/${key}`)).status).toBe(200);
    expect((await srv.as("owner").get("/api/v1/capacity")).body.counts.issues).toBe(before.counts.issues);
  });

  it("is for the owner only", async () => {
    expect((await srv.as("alice").get("/api/v1/capacity")).status).toBe(403);
    expect((await srv.as("claude").get("/api/v1/capacity")).status).toBe(403);
    expect((await srv.raw().get("/api/v1/capacity")).status).toBe(401);
  });
});
