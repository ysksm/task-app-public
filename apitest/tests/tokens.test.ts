import { rmSync } from "node:fs";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { startServer, TOKENS, type Server } from "../src/harness.js";

let srv: Server;
beforeAll(async () => {
  srv = await startServer();
});
afterAll(() => srv?.stop());

describe("tokens and users", () => {
  it("lets the owner issue a token for an agent; the token works until revoked", async () => {
    const res = await srv.as("owner").post("/api/v1/tokens", { user: "claude", name: "mcp" });
    expect(res.status).toBe(201);
    expect(res.body.token).toMatch(/^tk_/);
    expect(res.body.info).toMatchObject({ name: "mcp", user: { username: "claude" }, prefix: res.body.token.slice(0, 7) });
    expect(res.body.info).not.toHaveProperty("token_hash");

    const client = srv.raw(`Bearer ${res.body.token}`);
    expect((await client.get("/api/v1/me")).body.username).toBe("claude");
    const listed = (await srv.as("owner").get("/api/v1/tokens")).body.tokens.find((t: any) => t.id === res.body.info.id);
    expect(listed.last_used_at).not.toBeNull();

    expect((await srv.as("owner").delete(`/api/v1/tokens/${res.body.info.id}`)).status).toBe(204);
    expect((await client.get("/api/v1/me")).status).toBe(401);
    expect((await srv.as("owner").delete(`/api/v1/tokens/${res.body.info.id}`)).status).toBe(404);
  });

  it("restricts non-owners to their own tokens", async () => {
    expect((await srv.as("claude").post("/api/v1/tokens", { user: "codex", name: "x" })).status).toBe(403);
    const own = await srv.as("claude").post("/api/v1/tokens", { name: "self" });
    expect(own.body.info.user.username).toBe("claude");
    const visible = (await srv.as("claude").get("/api/v1/tokens")).body.tokens;
    expect(visible.every((t: any) => t.user.username === "claude")).toBe(true);
    const ownerSees = (await srv.as("owner").get("/api/v1/tokens")).body.tokens.map((t: any) => t.user.username);
    expect(new Set(ownerSees)).toEqual(new Set(["owner", "alice", "claude", "codex"]));
    const codexToken = (await srv.as("owner").get("/api/v1/tokens")).body.tokens.find((t: any) => t.user.username === "codex");
    expect((await srv.as("claude").delete(`/api/v1/tokens/${codexToken.id}`)).status).toBe(404);
  });

  it("lets the owner register agents", async () => {
    const res = await srv.as("owner").post("/api/v1/users", { kind: "agent", username: "gemini", display_name: "Gemini", agent_model: "gemini" });
    expect(res.status).toBe(201);
    expect(res.body).toMatchObject({ kind: "agent", username: "gemini", is_owner: false });
    expect((await srv.as("owner").post("/api/v1/users", { kind: "agent", username: "gemini" })).status).toBe(409);
    expect((await srv.as("owner").post("/api/v1/users", { kind: "robot", username: "x" })).status).toBe(400);
    expect((await srv.as("claude").post("/api/v1/users", { kind: "agent", username: "y" })).status).toBe(403);
  });

  it("shows a token again later; only the owner or the token's user may see it", async () => {
    const issued = await srv.as("owner").post("/api/v1/tokens", { user: "claude", name: "later" });
    const id = issued.body.info.id;
    const again = await srv.as("owner").post(`/api/v1/tokens/${id}/reveal`);
    expect(again.status).toBe(200);
    assertOperation("revealToken", 200, again.body);
    expect(again.body.token).toBe(issued.body.token);
    expect((await srv.as("claude").post(`/api/v1/tokens/${id}/reveal`)).body.token).toBe(issued.body.token);
    expect((await srv.as("codex").post(`/api/v1/tokens/${id}/reveal`)).status).toBe(404);
    expect((await srv.raw().post(`/api/v1/tokens/${id}/reveal`)).status).toBe(401);
    // Tokens from the CLI (seed, `taskapp token create`) are kept too.
    const seeded = (await srv.as("owner").get("/api/v1/tokens")).body.tokens.find((t: any) => t.name === "test" && t.user.username === "alice");
    expect((await srv.as("owner").post(`/api/v1/tokens/${seeded.id}/reveal`)).body.token).toBe(TOKENS.alice);

    await srv.as("owner").delete(`/api/v1/tokens/${id}`);
    expect((await srv.as("owner").post(`/api/v1/tokens/${id}/reveal`)).status).toBe(404);
  });
});

describe("tokens when token.key is replaced", () => {
  it("still authenticates but can no longer show the token (not_available)", async () => {
    const other = await startServer({ setup: (_cli, dataDir) => rmSync(join(dataDir, "token.key")) });
    try {
      expect((await other.as("owner").get("/api/v1/me")).status).toBe(200);
      const t = (await other.as("owner").get("/api/v1/tokens")).body.tokens.find((t: any) => t.user.username === "owner");
      const res = await other.as("owner").post(`/api/v1/tokens/${t.id}/reveal`);
      expect(res.status).toBe(409);
      expect(res.body.error.code).toBe("not_available");
    } finally {
      await other.stop();
    }
  });
});
