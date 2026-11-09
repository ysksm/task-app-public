// MCP server tests over both transports, using the official TypeScript client.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { callTool, startServer, type Server } from "../src/harness.js";

let srv: Server;
let story: string, task: string;

beforeAll(async () => {
  srv = await startServer();
  const owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
  const epic = (await owner.post("/api/v1/issues", { project_key: "APP", type: "epic", title: "Auth" })).body.key;
  story = (await owner.post("/api/v1/issues", { parent_key: epic, type: "story", title: "Log in" })).body.key;
  task = (await owner.post("/api/v1/issues", { parent_key: story, type: "task", title: "Login form", assignee: "claude" })).body.key;
});
afterAll(() => srv?.stop());

const TOOLS = [
  "add_comment", "ask_question", "close_question", "create_activity", "create_draft", "create_issue", "create_persona", "create_step", "create_story", "get_import_plan", "get_issue", "get_issue_tree",
  "get_question_answers", "get_storymap", "list_my_issues", "list_projects", "record_checkpoint", "request_acceptance", "search_issues", "transition_issue", "update_activity", "update_issue",
  "update_persona", "update_release", "update_step", "update_story",
];

describe("MCP over HTTP", () => {
  it("rejects requests without a valid token", async () => {
    const body = JSON.stringify({ jsonrpc: "2.0", id: 1, method: "initialize", params: { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "x", version: "1" } } });
    for (const auth of [undefined, "Bearer tk_nope"]) {
      const res = await fetch(srv.baseUrl + "/mcp", {
        method: "POST", body,
        headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream", ...(auth ? { Authorization: auth } : {}) },
      });
      expect(res.status).toBe(401);
    }
  });

  it("lists the tools with instructions", async () => {
    const client = await srv.mcpHttp("claude");
    const { tools } = await client.listTools();
    expect(tools.map((t) => t.name).sort()).toEqual(TOOLS);
    const ask = tools.find((t) => t.name === "ask_question")!;
    expect(Object.keys((ask.inputSchema as any).properties)).toEqual(expect.arrayContaining(["target_key", "questions", "is_blocking"]));
    expect(tools.find((t) => t.name === "get_issue")!.annotations?.readOnlyHint).toBe(true);
    expect(client.getInstructions()).toMatch(/record_checkpoint/);
    expect(client.getInstructions()).toMatch(/update_story/);
  });

  it("acts as the token's user", async () => {
    const codex = await srv.mcpHttp("codex");
    expect((await callTool(codex, "list_my_issues")).issues).toEqual([]);
    const claude = await srv.mcpHttp("claude");
    const mine = await callTool(claude, "list_my_issues");
    expect(mine.issues.map((i: any) => i.key)).toEqual([task]);
    await callTool(claude, "add_comment", { key: task, body: "hello from MCP" });
    const comments = (await srv.as("owner").get(`/api/v1/issues/${task}/comments`)).body.comments;
    expect(comments.at(-1)).toMatchObject({ body: "hello from MCP", source: "mcp", author: { username: "claude" } });
  });

  it("returns rule violations as tool errors the model can read", async () => {
    const client = await srv.mcpHttp("claude");
    const res: any = await client.callTool({ name: "create_issue", arguments: { parent_key: task, type: "story", title: "x" } });
    expect(res.isError).toBe(true);
    expect(res.content[0].text).toMatch(/stories can only be placed under epics/);
    const bad: any = await client.callTool({ name: "transition_issue", arguments: { key: task, state: "Doing" } });
    expect(bad.isError).toBe(true);
    expect(bad.content[0].text).toMatch(/valid states: Backlog, Todo, In Progress/);
  });
});

describe("agent workflow over stdio", () => {
  it("works an issue end to end: start, checkpoint, ask, read answers, split, finish", async () => {
    const agent = await srv.mcpStdio("claude");

    const started = await callTool(agent, "transition_issue", { key: task, state: "in progress" });
    expect(started.state).toBe("In Progress");

    const cp = await callTool(agent, "record_checkpoint", {
      key: task, summary: "Built the form", decision: "React Hook Form", rationale: "Already a dependency",
      alternatives: ["Formik"], confidence: "low", progress: 40, artifacts: { branch: "feat/login", commits: ["abc"] },
    });
    expect(cp).toMatchObject({ seq: 1, confidence: "low", source: "mcp", artifacts: { branch: "feat/login", commits: ["abc"] } });

    const qa = await callTool(agent, "ask_question", {
      target_key: task, is_blocking: true, context: "Spec is silent", provisional_decision: "8+ chars",
      questions: [
        { question: "Rule?", answer_type: "single_choice", options: [{ label: "8+", recommended: true }, { label: "12+" }, { label: "16+" }] },
        { question: "Show SSO?", answer_type: "yes_no" },
        { question: "Which extras?", answer_type: "multi_choice", options: [{ label: "Remember me" }, { label: "Forgot password" }] },
      ],
    });
    expect(qa).toMatchObject({ status: "open", target_key: task, is_blocking: true, asked_by: { username: "claude" } });
    expect((await srv.as("owner").get(`/api/v1/issues/${task}`)).body.blocking_qa_count).toBe(1);

    // Nothing answered yet.
    const pending = await callTool(agent, "get_question_answers");
    expect(pending.open.map((q: any) => q.key)).toEqual([qa.key]);
    expect(pending.answered).toEqual([]);

    // A human answers in the UI (REST).
    const ids = qa.items.map((i: any) => i.id);
    await srv.as("owner").post(`/api/v1/qa/${qa.key}/answers`, {
      answers: [
        { item_id: ids[0], selected_option_ids: ["b"], note: "security policy" },
        { item_id: ids[1], selected_option_ids: ["no"] },
        { item_id: ids[2], selected_option_ids: ["a", "b"] },
      ],
    });

    const answered = await callTool(agent, "get_question_answers", { key: qa.key });
    expect(answered.status).toBe("answered");
    expect(answered.items.map((i: any) => i.answer.selected_options.map((o: any) => o.label))).toEqual([["12+"], ["いいえ"], ["Remember me", "Forgot password"]]);
    expect(answered.items[0].answer).toMatchObject({ note: "security policy", answered_by: { username: "owner", kind: "human" } });

    expect((await callTool(agent, "close_question", { key: qa.key })).status).toBe("closed");

    const sub = await callTool(agent, "create_issue", { parent_key: task, type: "task", title: "Apply 12+ rule", assignee: "me" });
    expect(sub).toMatchObject({ type: "task", parent_key: task, assignee: "claude" });
    await callTool(agent, "transition_issue", { key: sub.key, state: "Done" });

    const tree = await callTool(agent, "get_issue_tree", { key: story });
    expect(tree.children[0]).toMatchObject({ key: task, state: "In Progress" });
    expect(tree.children[0].children.map((c: any) => c.type).sort()).toEqual(["qa", "task"]);

    const detail = await callTool(agent, "get_issue", { key: task });
    expect(detail.checkpoints).toHaveLength(1);
    expect(detail.issue.progress).toEqual({ total: 1, done: 1 });

    const accepted = await callTool(agent, "request_acceptance", { key: story, note: "Please try logging in" });
    expect(accepted.state).toBe("受入待ち");

    // Every change is attributed to the agent and to the MCP channel.
    const activity = (await srv.as("owner").get(`/api/v1/issues/${task}/activity`)).body.activity;
    const byAgent = activity.filter((a: any) => a.actor.username === "claude");
    expect(byAgent.length).toBeGreaterThan(0);
    expect(byAgent.every((a: any) => a.source === "mcp")).toBe(true);
  });

  it("refuses to start without a valid token", async () => {
    const { execFileSync } = await import("node:child_process");
    const { BIN } = await import("../src/harness.js");
    expect(() => execFileSync(BIN, ["mcp", "--data", srv.dataDir], { env: { ...process.env, TASKAPP_TOKEN: "" }, stdio: "pipe" })).toThrow(/TASKAPP_TOKEN/);
    expect(() => execFileSync(BIN, ["mcp", "--data", srv.dataDir], { env: { ...process.env, TASKAPP_TOKEN: "tk_nope" }, stdio: "pipe" })).toThrow(/invalid token/);
  });

  it("forwards stdio changes to the running server's live updates", async () => {
    const res = await fetch(srv.baseUrl + "/api/v1/events", { headers: { Authorization: "Bearer tk_test_owner" } });
    const reader = res.body!.getReader();
    const received = (async () => {
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) return "";
        buf += new TextDecoder().decode(value);
        if (buf.includes(`"issue_key":"${task}"`)) return buf;
      }
    })();
    const agent = await srv.mcpStdio("claude");
    await callTool(agent, "update_issue", { key: task, title: "Login form (renamed)" });
    const stream = await Promise.race([received, new Promise<string>((r) => setTimeout(() => r("timeout"), 5000))]);
    await reader.cancel();
    expect(stream).toContain('"type":"issue.updated"');
  });

  it("serves issues as resources and a workflow prompt", async () => {
    const agent = await srv.mcpStdio("codex");
    const { resourceTemplates } = await agent.listResourceTemplates();
    expect(resourceTemplates.map((t) => t.uriTemplate)).toEqual(["taskapp://issue/{key}"]);
    const res = await agent.readResource({ uri: `taskapp://issue/${task}` });
    expect((res.contents[0] as any).text).toMatch(new RegExp(`^# ${task} `));
    const prompt = await agent.getPrompt({ name: "work_on_issue", arguments: { issue_key: task } });
    expect((prompt.messages[0].content as any).text).toContain(`transition_issue ${task}`);
  });
});
