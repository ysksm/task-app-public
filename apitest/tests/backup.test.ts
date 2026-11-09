// Automatic backups (NFR-02): `serve --backup-interval/--backup-keep/--backup-dir`,
// GET/POST /api/v1/backups, `taskapp backup` without --out and `taskapp restore`.
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { BIN, startServer, type Server } from "../src/harness.js";

const NAME = /^taskapp-\d{8}-\d{6}(-\d+)?\.db$/;
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe("scheduled backups", () => {
  let srv: Server;
  const dir = mkdtempSync(join(tmpdir(), "taskapp-backups-"));
  beforeAll(async () => {
    srv = await startServer({ serveArgs: ["--backup-interval", "1s", "--backup-keep", "2", "--backup-dir", dir] });
  });
  afterAll(async () => {
    await srv?.stop();
    rmSync(dir, { recursive: true, force: true });
  });
  const backups = () => readdirSync(dir).filter((n) => NAME.test(n));

  it("backs up on start, again after changes, and keeps only --backup-keep files", async () => {
    await expect.poll(backups, { timeout: 5_000 }).toHaveLength(1);
    // The schedule checks once a minute, so take more on demand to see the rotation.
    for (let i = 0; i < 3; i++) {
      expect((await srv.as("owner").post("/api/v1/backups")).status).toBe(201);
    }
    expect(backups()).toHaveLength(2);
  });

  it("reports the schedule and the files, newest first", async () => {
    writeFileSync(join(dir, "notes.txt"), "not a backup");
    const res = await srv.as("owner").get("/api/v1/backups");
    expect(res.status).toBe(200);
    assertOperation("backupStatus", res.status, res.body);
    expect(res.body).toMatchObject({ enabled: true, dir, interval_seconds: 1, keep: 2, last_error: "" });
    expect(res.body.files.map((f: any) => f.name)).toEqual(backups().sort().reverse());
    expect(res.body.last_backup_at).toBe(res.body.files[0].created_at);
    expect(existsSync(join(dir, "notes.txt"))).toBe(true);
  });

  it("creates a backup now (owner only)", async () => {
    const res = await srv.as("owner").post("/api/v1/backups");
    expect(res.status).toBe(201);
    assertOperation("createBackup", res.status, res.body);
    expect(res.body.name).toMatch(NAME);
    expect(res.body.size).toBeGreaterThan(0);
    expect((await srv.as("alice").post("/api/v1/backups")).status).toBe(403);
    expect((await srv.as("claude").get("/api/v1/backups")).status).toBe(403);
  });
});

describe("backup settings", () => {
  it("--backup-interval 0 turns the schedule off; the default directory is <data>/backups", async () => {
    const srv = await startServer({ serveArgs: ["--backup-interval", "0"] });
    try {
      await sleep(300);
      const res = await srv.as("owner").get("/api/v1/backups");
      expect(res.body).toMatchObject({ enabled: false, dir: join(srv.dataDir, "backups"), files: [], next_backup_at: null, last_backup_at: null });
      // `taskapp backup` without --out writes into the same directory and,
      // unlike the schedule, removes nothing unless --backup-keep is given.
      const path = srv.cli("backup");
      expect(path.startsWith(join(srv.dataDir, "backups"))).toBe(true);
      for (let i = 0; i < 8; i++) expect((await srv.as("owner").post("/api/v1/backups")).status).toBe(201);
      srv.cli("backup");
      expect((await srv.as("owner").get("/api/v1/backups")).body.files).toHaveLength(8); // the server keeps 7, +1 manual
      srv.cli("backup", "--backup-keep", "3");
      expect((await srv.as("owner").get("/api/v1/backups")).body.files).toHaveLength(3);
    } finally {
      await srv.stop();
    }
  });
});

describe("restore", () => {
  it("replaces the database with a backup and keeps the previous one aside", async () => {
    const data = mkdtempSync(join(tmpdir(), "taskapp-restore-"));
    const run = (...args: string[]) => execFileSync(BIN, [...args, "--data", data], { encoding: "utf8", stdio: "pipe" }).trim();
    try {
      run("seed", "--preset", "demo");
      const backup = run("backup");
      const srv = await startServer({ preset: "none", setup: (cli) => { cli("restore", "--from", backup); } });
      try {
        // The demo data (users, tokens, issues) came back with the backup.
        const owner = srv.raw("Bearer tk_demo_owner");
        const count = async () => (await owner.get("/api/v1/issues?project=APP&limit=200")).body.issues.length;
        const before = await count();
        expect(before).toBe(21);
        expect((await owner.post("/api/v1/issues", { project_key: "APP", type: "task", title: "after restore" })).status).toBe(201);
        expect(await count()).toBe(before + 1);
        // Refused while a server runs on the data directory.
        expect(() => srv.cli("restore", "--from", backup)).toThrow(/stop it first/);
      } finally {
        await srv.stop();
      }
      // Restoring over existing data keeps the current database aside.
      run("restore", "--from", backup);
      expect(readdirSync(data).filter((n) => n.startsWith("taskapp.db.before-restore-"))).toHaveLength(1);
    } finally {
      rmSync(data, { recursive: true, force: true });
    }
  });

  it("refuses files that are not taskapp databases", () => {
    const data = mkdtempSync(join(tmpdir(), "taskapp-restore-"));
    try {
      const bad = join(data, "bad.db");
      writeFileSync(bad, "hello");
      expect(() => execFileSync(BIN, ["restore", "--from", bad, "--data", data], { stdio: "pipe" })).toThrow(/not a taskapp database/);
    } finally {
      rmSync(data, { recursive: true, force: true });
    }
  });
});
