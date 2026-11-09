import { request } from "node:http";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { assertOperation } from "../src/contract.js";
import { TOKENS, startServer, type Server } from "../src/harness.js";

// Serving through a reverse proxy on the same machine (tailscale serve, Caddy):
// every request arrives from loopback, carrying the proxy's public Host name.

const PUBLIC_HOST = "task.example.ts.net";
const INITIALIZE = JSON.stringify({ jsonrpc: "2.0", id: 1, method: "initialize", params: { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "x", version: "1" } } });

/** POST with an explicit Host header (fetch does not allow setting Host). */
function post(srv: Server, path: string, host: string, headers: Record<string, string> = {}, body = ""): Promise<number> {
  const url = new URL(srv.baseUrl + path);
  return new Promise((resolve, reject) => {
    const req = request({ hostname: url.hostname, port: url.port, path, method: "POST", headers: { Host: host, ...headers } }, (res) => {
      res.resume();
      resolve(res.statusCode!);
    });
    req.on("error", reject);
    req.end(body);
  });
}

function mcpInitialize(srv: Server, host: string, token?: string): Promise<number> {
  return post(srv, "/mcp", host, {
    "Content-Type": "application/json",
    Accept: "application/json, text/event-stream",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }, INITIALIZE);
}

describe("without --behind-proxy", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer();
  });
  afterAll(() => srv?.stop());

  it("refuses MCP requests that reach loopback with another Host name (DNS rebinding)", async () => {
    const port = new URL(srv.baseUrl).port;
    expect(await mcpInitialize(srv, `127.0.0.1:${port}`, TOKENS.claude)).toBe(200);
    expect(await mcpInitialize(srv, PUBLIC_HOST, TOKENS.claude)).toBe(403);
  });
});

describe("serve --behind-proxy --public-url", () => {
  let srv: Server;
  beforeAll(async () => {
    srv = await startServer({ serveArgs: ["--behind-proxy", "--public-url", `https://${PUBLIC_HOST}/`] });
  });
  afterAll(() => srv?.stop());

  it("accepts MCP requests forwarded with the public Host name; the token is still required", async () => {
    expect(await mcpInitialize(srv, PUBLIC_HOST, TOKENS.claude)).toBe(200);
    expect(await mcpInitialize(srv, PUBLIC_HOST)).toBe(401);
    expect(await mcpInitialize(srv, PUBLIC_HOST, "tk_nope")).toBe(401);
  });

  it("turns automatic owner login off, since every request comes from loopback", async () => {
    const port = new URL(srv.baseUrl).port;
    expect(await post(srv, "/api/v1/auth/local", `127.0.0.1:${port}`)).toBe(403);
    expect(await post(srv, "/api/v1/auth/local", PUBLIC_HOST)).toBe(403);
    expect((await srv.raw().get("/api/v1/auth/options")).body.local_login).toBe(false);
  });

  it("shows the public URL in the server info", async () => {
    const res = await srv.as("owner").get("/api/v1/server");
    assertOperation("serverInfo", 200, res.body);
    expect(res.body.remote_url).toBe(`https://${PUBLIC_HOST}`);
  });
});
