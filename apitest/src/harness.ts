// Starts the server under test as a black box. Only the CLI contract from
// docs/05-implementation-plan.md §3.1 is used, so the same tests run against
// any implementation: set TASKAPP_BIN to the binary to test.
import { spawn, execFileSync, type ChildProcess } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createInterface } from "node:readline";
import { Client as McpClient } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

export const BIN = resolve(process.env.TASKAPP_BIN ?? join(import.meta.dirname, "../../bin/taskapp"));

/** Fixed tokens created by `seed --preset test`. */
export const TOKENS = {
  owner: "tk_test_owner",
  alice: "tk_test_alice",
  claude: "tk_test_claude",
  codex: "tk_test_codex",
} as const;
export type Who = keyof typeof TOKENS;

export interface Server {
  baseUrl: string;
  dataDir: string;
  /** Client authenticated as the given seeded user. */
  as(who: Who): Client;
  /** Client with an arbitrary Authorization header (or none). */
  raw(authorization?: string, headers?: Record<string, string>): Client;
  /** Jira client the way ORCA (Cloud mode) calls: Basic email:token, User-Agent "Orca". */
  jira(who: Who): Client;
  /** Runs a CLI subcommand against this server's data directory. */
  cli(...args: string[]): string;
  /** Stops and starts `serve` again on the same data directory (a deploy). baseUrl changes. */
  restart(): Promise<void>;
  /** MCP client over Streamable HTTP (/mcp), authenticated as the seeded user. */
  mcpHttp(who: Who): Promise<McpClient>;
  /** MCP client over stdio (`taskapp mcp`), authenticated with TASKAPP_TOKEN. */
  mcpStdio(who: Who): Promise<McpClient>;
  stop(): Promise<void>;
}

export interface Response<T = any> {
  status: number;
  body: T;
  headers: Headers;
}

export interface Client {
  get<T = any>(path: string): Promise<Response<T>>;
  post<T = any>(path: string, body?: unknown): Promise<Response<T>>;
  patch<T = any>(path: string, body?: unknown): Promise<Response<T>>;
  put<T = any>(path: string, body?: unknown): Promise<Response<T>>;
  delete<T = any>(path: string): Promise<Response<T>>;
}

/** Starts a fresh server with `seed --preset <preset>` data. */
export async function startServer(opts: {
  preset?: "test" | "demo" | "perf" | "none";
  /** runs before the server starts, e.g. to import data into the empty data directory */
  setup?: (cli: (...args: string[]) => string, dataDir: string) => void;
  /** extra `serve` flags, e.g. ["--backup-interval", "1s"] */
  serveArgs?: string[];
} = {}): Promise<Server> {
  const dataDir = mkdtempSync(join(tmpdir(), "taskapp-apitest-"));
  const cli = (...args: string[]) =>
    execFileSync(BIN, [args[0], ...args.slice(1), "--data", dataDir], { encoding: "utf8" }).trim();
  const preset = opts.preset ?? "test";
  if (preset !== "none") cli("seed", "--preset", preset);
  opts.setup?.(cli, dataDir);

  const serve = async () => {
    const child = spawn(BIN, ["serve", "--data", dataDir, "--port", "0", ...(opts.serveArgs ?? [])], { stdio: ["ignore", "pipe", "pipe"] });
    let stderr = "";
    child.stderr!.on("data", (d) => (stderr += d));
    const addr = await waitForReady(child, () => stderr);
    return { child, baseUrl: `http://${addr}` };
  };
  let { child, baseUrl } = await serve();
  const mcpClients: McpClient[] = [];

  const client = (authorization?: string, extra: Record<string, string> = {}): Client => {
    const call = async (method: string, path: string, body?: unknown): Promise<Response> => {
      const headers: Record<string, string> = { ...extra };
      if (authorization) headers.Authorization = authorization;
      if (body !== undefined) headers["Content-Type"] = "application/json";
      const res = await fetch(baseUrl + path, { method, headers, redirect: "manual", body: body === undefined ? undefined : JSON.stringify(body) });
      const text = await res.text();
      let parsed: unknown = text;
      try {
        parsed = text ? JSON.parse(text) : null;
      } catch {
        /* keep text */
      }
      return { status: res.status, body: parsed, headers: res.headers };
    };
    return {
      get: (p) => call("GET", p),
      post: (p, b) => call("POST", p, b ?? {}),
      patch: (p, b) => call("PATCH", p, b ?? {}),
      put: (p, b) => call("PUT", p, b ?? {}),
      delete: (p) => call("DELETE", p),
    };
  };

  return {
    get baseUrl() {
      return baseUrl;
    },
    dataDir,
    async restart() {
      await stopChild(child);
      ({ child, baseUrl } = await serve());
    },
    as: (who) => client(`Bearer ${TOKENS[who]}`),
    raw: (authorization, headers) => client(authorization, headers),
    jira: (who) => client(basicAuth(`${who}@taskapp.local`, TOKENS[who]), { "User-Agent": "Orca", Accept: "application/json" }),
    cli,
    async mcpHttp(who) {
      const client = new McpClient({ name: "apitest", version: "1.0.0" });
      await client.connect(new StreamableHTTPClientTransport(new URL(baseUrl + "/mcp"), {
        requestInit: { headers: { Authorization: `Bearer ${TOKENS[who]}` } },
      }));
      mcpClients.push(client);
      return client;
    },
    async mcpStdio(who) {
      const client = new McpClient({ name: "apitest", version: "1.0.0" });
      await client.connect(new StdioClientTransport({
        command: BIN, args: ["mcp", "--data", dataDir],
        env: { ...(process.env as Record<string, string>), TASKAPP_TOKEN: TOKENS[who] }, stderr: "pipe",
      }));
      mcpClients.push(client);
      return client;
    },
    async stop() {
      await Promise.all(mcpClients.map((c) => c.close().catch(() => {})));
      await stopChild(child);
      rmSync(dataDir, { recursive: true, force: true });
    },
  };
}

function waitForReady(child: ChildProcess, stderr: () => string): Promise<string> {
  return new Promise((resolvePromise, reject) => {
    const timer = setTimeout(() => reject(new Error(`server did not become ready:\n${stderr()}`)), 10_000);
    const rl = createInterface({ input: child.stdout! });
    rl.on("line", (line) => {
      try {
        const msg = JSON.parse(line);
        if (msg.event === "ready" && msg.addr) {
          clearTimeout(timer);
          resolvePromise(msg.addr);
        }
      } catch {
        /* ignore non-JSON output */
      }
    });
    child.on("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`server exited with ${code}:\n${stderr()}`));
    });
  });
}

function stopChild(child: ChildProcess): Promise<void> {
  if (child.exitCode !== null) return Promise.resolve();
  return new Promise((done) => {
    child.removeAllListeners("exit");
    child.once("exit", () => done());
    child.kill("SIGTERM");
    setTimeout(() => child.kill("SIGKILL"), 3000);
  });
}

/** Builds a Basic Authorization header value. */
export function basicAuth(user: string, password: string): string {
  return "Basic " + Buffer.from(`${user}:${password}`).toString("base64");
}

/** Encodes a filter for the ?filter= query parameter. */
export function filterParam(filter: unknown): string {
  return encodeURIComponent(JSON.stringify(filter));
}

/** Calls an MCP tool and parses its JSON text result; throws on tool errors. */
export async function callTool<T = any>(client: McpClient, name: string, args: Record<string, unknown> = {}): Promise<T> {
  const res: any = await client.callTool({ name, arguments: args });
  const text = res.content?.[0]?.text ?? "";
  if (res.isError) throw new Error(text);
  return JSON.parse(text);
}
