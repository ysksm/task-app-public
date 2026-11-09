import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { startServer, type Server } from "../src/harness.js";

let srv: Server;
beforeAll(async () => {
  srv = await startServer();
});
afterAll(() => srv?.stop());

describe("health and authentication", () => {
  it("health needs no auth", async () => {
    const res = await srv.raw().get("/api/v1/health");
    expect(res.status).toBe(200);
    expect(res.body).toEqual({ status: "ok" });
  });

  it("returns the token's user from /me", async () => {
    const res = await srv.as("claude").get("/api/v1/me");
    expect(res.status).toBe(200);
    expect(res.body).toMatchObject({ username: "claude", kind: "agent", is_owner: false });
  });

  it("rejects missing and invalid tokens with the error envelope", async () => {
    for (const auth of [undefined, "Bearer tk_nope"]) {
      const res = await srv.raw(auth).get("/api/v1/me");
      expect(res.status).toBe(401);
      expect(res.body.error.code).toBe("unauthorized");
      expect(typeof res.body.error.message).toBe("string");
    }
  });

  it("issues working tokens from the CLI", async () => {
    const token = srv.cli("token", "create", "--user", "codex", "--name", "ci");
    expect(token).toMatch(/^tk_\S+$/);
    const res = await srv.raw(`Bearer ${token}`).get("/api/v1/me");
    expect(res.body.username).toBe("codex");
  });

  it("gives the browser an owner session via local login", async () => {
    const login = await fetch(srv.baseUrl + "/api/v1/auth/local", { method: "POST" });
    expect(login.status).toBe(200);
    const cookie = login.headers.get("set-cookie")!.split(";")[0];
    const me = await fetch(srv.baseUrl + "/api/v1/me", { headers: { Cookie: cookie } });
    expect((await me.json()).is_owner).toBe(true);
  });

  it("lists users with their kind", async () => {
    const res = await srv.as("owner").get("/api/v1/users");
    const kinds = Object.fromEntries(res.body.users.map((u: any) => [u.username, u.kind]));
    expect(kinds).toEqual({ owner: "human", alice: "human", claude: "agent", codex: "agent" });
  });

  it("answers unknown endpoints with 404 in the error envelope", async () => {
    const res = await srv.as("owner").get("/api/v1/nope");
    expect(res.status).toBe(404);
    expect(res.body.error.code).toBe("not_found");
  });
});
