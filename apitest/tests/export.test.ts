// Export / import / backup (NFR-02). A restore keeps every id, so Jira
// clients, MCP agents (tokens) and links keep working.
import { existsSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { execFileSync } from "node:child_process";
import { BIN, startServer, type Server } from "../src/harness.js";

let src: Server;
let restored: Server;
const dir = mkdtempSync(join(tmpdir(), "taskapp-export-"));
const file = join(dir, "export.json");

beforeAll(async () => {
  src = await startServer({ preset: "demo" });
  // Exported while the server is running.
  src.cli("export", "--out", file);
  restored = await startServer({ preset: "none", setup: (cli) => cli("import", "--in", file) });
});
afterAll(async () => {
  await Promise.all([src?.stop(), restored?.stop()]);
  rmSync(dir, { recursive: true, force: true });
});

const owner = (s: Server) => s.raw("Bearer tk_demo_owner");

describe("export and import", () => {
  it("restores the same data, ids and tokens", async () => {
    for (const path of [
      "/api/v1/issues?project=APP&sort=key",
      "/api/v1/issues/APP-3",
      "/api/v1/qa/APP-8",
      "/api/v1/issues/APP-3/checkpoints",
      "/api/v1/issues/APP-2/acceptance",
      "/api/v1/views",
      "/api/v1/projects/APP/states",
      "/api/v1/users",
    ]) {
      const a = await owner(src).get(path);
      const b = await owner(restored).get(path);
      expect(b.status, path).toBe(200);
      expect(b.body, path).toEqual(a.body);
    }
    // Jira ids and agent tokens survive.
    const jira = (s: Server) => s.jira("owner").get("/rest/api/3/issue/APP-3?fields=summary");
    expect((await jira(restored)).body.id).toBe((await jira(src)).body.id);
    expect((await restored.raw("Bearer tk_demo_claude").get("/api/v1/me")).body.username).toBe("claude");
  });

  it("continues numbering after the imported data", async () => {
    const created = await owner(restored).post("/api/v1/issues", { project_key: "APP", type: "task", title: "after restore" });
    expect(created.status).toBe(201);
    expect(created.body.key).toBe("APP-22");
  });

  it("refuses to import into a database that has data", () => {
    expect(() => src.cli("import", "--in", file)).toThrow(/not empty/);
  });

  it("refuses files that are not exports, or from a newer schema", () => {
    const bad = join(dir, "bad.json");
    writeFileSync(bad, JSON.stringify({ hello: 1 }));
    const empty = mkdtempSync(join(tmpdir(), "taskapp-empty-"));
    try {
      const load = () => execFileSync(BIN, ["import", "--in", bad, "--data", empty], { stdio: "pipe" });
      expect(load).toThrow(/unsupported export format/);
      writeFileSync(bad, JSON.stringify({ format: 1, schema_version: 9999, exported_at: "", tables: {} }));
      expect(load).toThrow(/newer than this binary/);
    } finally {
      rmSync(empty, { recursive: true, force: true });
    }
  });

  it("downloads an export through the API (owner only)", async () => {
    const res = await owner(src).get("/api/v1/export");
    expect(res.status).toBe(200);
    assertOperation("exportData", res.status, res.body);
    expect(res.headers.get("content-disposition")).toMatch(/attachment; filename="taskapp-export-\d{8}-\d{6}\.json"/);
    expect(res.body.tables.issues).toHaveLength(21);
    expect(res.body.tables.api_tokens[0].token_hash).toMatch(/^[0-9a-f]{64}$/);
    expect((await src.raw("Bearer tk_demo_alice").get("/api/v1/export")).status).toBe(403);
  });
});

describe("backup", () => {
  it("copies the database file while the server is running", () => {
    const out = join(dir, "backup.db");
    src.cli("backup", "--out", out);
    expect(existsSync(out)).toBe(true);
    expect(() => src.cli("backup", "--out", out)).toThrow(/already exists/);
  });
});
