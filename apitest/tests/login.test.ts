import { execFileSync } from "node:child_process";
import { request } from "node:http";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { BIN, startServer, type Server } from "../src/harness.js";

// Logging in from other devices (01 §4.12): passwords for people, automatic
// owner login only for a browser on this machine.

/** `taskapp passwd`, with the password on stdin. */
function passwd(srv: Server, user: string, password: string): string {
  return execFileSync(BIN, ["passwd", "--user", user, "--data", srv.dataDir], { input: password + "\n", encoding: "utf8", stdio: ["pipe", "pipe", "pipe"] });
}

async function login(srv: Server, username: string, password: string, remember?: boolean) {
  const res = await fetch(srv.baseUrl + "/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(remember === undefined ? { username, password } : { username, password, remember }),
  });
  const body = await res.json().catch(() => null);
  const setCookie = res.headers.get("set-cookie") ?? "";
  const cookie = setCookie.split(";")[0];
  return { status: res.status, body, cookie, setCookie };
}

async function withCookie(srv: Server, cookie: string, method: string, path: string, body?: unknown) {
  const res = await fetch(srv.baseUrl + path, {
    method,
    headers: { Cookie: cookie, ...(body ? { "Content-Type": "application/json" } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  return { status: res.status, body: res.status === 204 ? null : await res.json() };
}

/** POST /auth/local with an explicit Host header (fetch does not allow setting Host). */
function localLoginWithHost(srv: Server, host: string): Promise<number> {
  const url = new URL(srv.baseUrl + "/api/v1/auth/local");
  return new Promise((resolve, reject) => {
    const req = request({ hostname: url.hostname, port: url.port, path: url.pathname, method: "POST", headers: { Host: host } }, (res) => {
      res.resume();
      resolve(res.statusCode!);
    });
    req.on("error", reject);
    req.end();
  });
}

describe("password login", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer();
  });
  afterAll(() => srv?.stop());

  it("tells the login page what is available", async () => {
    const res = await srv.raw().get("/api/v1/auth/options");
    assertOperation("authOptions", 200, res.body);
    expect(res.body).toEqual({ local_login: true, password_login: false });
  });

  it("logs a person in with the password set by the CLI, and out again", async () => {
    passwd(srv, "alice", "alice-secret-1");
    expect((await srv.raw().get("/api/v1/auth/options")).body.password_login).toBe(true);

    const ok = await login(srv, "alice", "alice-secret-1");
    expect(ok.status).toBe(200);
    assertOperation("login", 200, ok.body);
    expect(ok.body).toMatchObject({ username: "alice", is_owner: false, has_password: true });
    expect(ok.cookie).toMatch(/^taskapp_session=/);

    const me = await withCookie(srv, ok.cookie, "GET", "/api/v1/me");
    expect(me.body.username).toBe("alice");

    expect((await withCookie(srv, ok.cookie, "POST", "/api/v1/auth/logout")).status).toBe(204);
    expect((await withCookie(srv, ok.cookie, "GET", "/api/v1/me")).status).toBe(401);
  });

  it("rejects a wrong password, an unknown user and agents with the same 401", async () => {
    for (const [user, password] of [["alice", "wrong-password"], ["nobody", "whatever-123"], ["claude", "whatever-123"]]) {
      const res = await login(srv, user, password);
      expect(res.status).toBe(401);
      expect(res.body.error).toEqual({ code: "unauthorized", message: "invalid username or password" });
      expect(res.cookie).toBe("");
    }
  });

  it("never returns the password hash, and leaves it out of the export", async () => {
    const users = await srv.as("owner").get("/api/v1/users");
    const alice = users.body.users.find((u: any) => u.username === "alice");
    expect(alice.has_password).toBe(true);
    expect(JSON.stringify(users.body)).not.toContain("pbkdf2");

    const exported = await srv.as("owner").get("/api/v1/export");
    const row = exported.body.tables.users.find((u: any) => u.username === "alice");
    expect(row.password_hash).toBe("");
    expect(JSON.stringify(exported.body)).not.toContain("pbkdf2");
  });
});

describe("sessions survive a restart (a deploy)", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer();
    passwd(srv, "alice", "alice-secret-1");
  });
  afterAll(() => srv?.stop());

  it("keeps both kinds of session across a restart; remember makes the cookie persistent", async () => {
    const plain = await login(srv, "alice", "alice-secret-1");
    expect(plain.setCookie).not.toMatch(/Max-Age|Expires/i);
    const kept = await login(srv, "alice", "alice-secret-1", true);
    expect(kept.setCookie).toMatch(/Max-Age=2592000/);
    expect(kept.setCookie).toMatch(/HttpOnly/);
    expect(kept.setCookie).not.toMatch(/Secure/); // plain http: a Secure cookie would be dropped

    await srv.restart();
    expect((await withCookie(srv, plain.cookie, "GET", "/api/v1/me")).body.username).toBe("alice");
    expect((await withCookie(srv, kept.cookie, "GET", "/api/v1/me")).body.username).toBe("alice");

    // Logging out ends that session only, for good.
    expect((await withCookie(srv, kept.cookie, "POST", "/api/v1/auth/logout")).status).toBe(204);
    await srv.restart();
    expect((await withCookie(srv, kept.cookie, "GET", "/api/v1/me")).status).toBe(401);
    expect((await withCookie(srv, plain.cookie, "GET", "/api/v1/me")).status).toBe(200);
    expect((await withCookie(srv, "taskapp_session=forged", "GET", "/api/v1/me")).status).toBe(401);
  });

  it("marks the cookie Secure behind an https proxy", async () => {
    const res = await fetch(srv.baseUrl + "/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Forwarded-Proto": "https" },
      body: JSON.stringify({ username: "alice", password: "alice-secret-1", remember: true }),
    });
    expect(res.headers.get("set-cookie")).toMatch(/Secure/);
  });

  it("leaves sessions out of the export", async () => {
    await login(srv, "alice", "alice-secret-1", true);
    const exported = await srv.as("owner").get("/api/v1/export");
    expect(exported.body.tables).not.toHaveProperty("sessions");
  });
});

describe("setting passwords over the API", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer();
  });
  afterAll(() => srv?.stop());

  it("lets people set their own; the current one is required to change it", async () => {
    const first = await srv.as("alice").put("/api/v1/users/me/password", { password: "first-pass-1" });
    expect(first.status).toBe(200);
    assertOperation("setPassword", 200, first.body);
    expect(first.body).toMatchObject({ username: "alice", has_password: true });

    const noCurrent = await srv.as("alice").put("/api/v1/users/alice/password", { password: "second-pass-2" });
    expect(noCurrent.status).toBe(403);
    const wrongCurrent = await srv.as("alice").put("/api/v1/users/alice/password", { password: "second-pass-2", current_password: "nope" });
    expect(wrongCurrent.status).toBe(403);
    const changed = await srv.as("alice").put("/api/v1/users/alice/password", { password: "second-pass-2", current_password: "first-pass-1" });
    expect(changed.status).toBe(200);

    expect((await login(srv, "alice", "first-pass-1")).status).toBe(401);
    expect((await login(srv, "alice", "second-pass-2")).status).toBe(200);
  });

  it("lets only the owner set other people's passwords, or remove them", async () => {
    const denied = await srv.as("alice").put("/api/v1/users/owner/password", { password: "takeover-123" });
    expect(denied.status).toBe(403);
    expect(denied.body.error.code).toBe("forbidden");

    expect((await srv.as("owner").put("/api/v1/users/alice/password", { password: "reset-by-owner" })).status).toBe(200);
    expect((await login(srv, "alice", "reset-by-owner")).status).toBe(200);

    const removed = await srv.as("owner").put("/api/v1/users/alice/password", { password: "" });
    expect(removed.status).toBe(200);
    expect(removed.body.has_password).toBe(false);
    expect((await login(srv, "alice", "reset-by-owner")).status).toBe(401);
  });

  it("rejects short passwords and passwords for agents", async () => {
    const short = await srv.as("owner").put("/api/v1/users/alice/password", { password: "short" });
    expect(short.status).toBe(400);
    expect(short.body.error.code).toBe("invalid_argument");
    const agent = await srv.as("owner").put("/api/v1/users/claude/password", { password: "agents-have-none" });
    expect(agent.status).toBe(400);
  });

  it("ends the user's other sessions when the password changes", async () => {
    await srv.as("owner").put("/api/v1/users/alice/password", { password: "session-pass-1" });
    const phone = await login(srv, "alice", "session-pass-1");
    const laptop = await login(srv, "alice", "session-pass-1");
    const changed = await withCookie(srv, laptop.cookie, "PUT", "/api/v1/users/me/password", { password: "session-pass-2", current_password: "session-pass-1" });
    expect(changed.status).toBe(200);
    expect((await withCookie(srv, phone.cookie, "GET", "/api/v1/me")).status).toBe(401);
    expect((await withCookie(srv, laptop.cookie, "GET", "/api/v1/me")).status).toBe(200);
  });
});

describe("automatic owner login is for this machine only", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer();
  });
  afterAll(() => srv?.stop());

  it("accepts loopback Host names", async () => {
    const port = new URL(srv.baseUrl).port;
    for (const host of [`127.0.0.1:${port}`, `localhost:${port}`, `[::1]:${port}`]) {
      expect(await localLoginWithHost(srv, host)).toBe(200);
    }
  });

  it("refuses other Host names (DNS rebinding, reverse proxies)", async () => {
    for (const host of ["attacker.example", "192.168.1.20:7700", "my-pc.tailnet.ts.net"]) {
      expect(await localLoginWithHost(srv, host)).toBe(403);
    }
  });
});

describe("--require-login", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer({ serveArgs: ["--require-login"] });
  });
  afterAll(() => srv?.stop());

  it("turns automatic login off even on this machine; passwords still work", async () => {
    const local = await srv.raw().post("/api/v1/auth/local");
    expect(local.status).toBe(403);
    expect(local.body.error.code).toBe("forbidden");
    expect((await srv.raw().get("/api/v1/auth/options")).body.local_login).toBe(false);

    passwd(srv, "owner", "owner-secret-1");
    const res = await login(srv, "owner", "owner-secret-1");
    expect(res.status).toBe(200);
    expect(res.body.is_owner).toBe(true);
  });

  it("slows down guessing: 5 failures in a row lock the address out for a minute", async () => {
    for (let i = 0; i < 5; i++) expect((await login(srv, "owner", `guess-${i}-xxxx`)).status).toBe(401);
    const locked = await login(srv, "owner", "owner-secret-1");
    expect(locked.status).toBe(429);
    expect(locked.body.error.code).toBe("rate_limited");
  });
});
