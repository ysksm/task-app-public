// Jira REST API v3 compatibility (docs/01-requirements.md §4.10).
// Requests mirror what ORCA sends in Jira Cloud mode (stablyai/orca
// src/main/jira/*): Basic email:token, User-Agent "Orca", ADF bodies.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { basicAuth, callTool, startServer, TOKENS, type Client, type Server } from "../src/harness.js";

let srv: Server;
let jira: Client;
let owner: Client;
let projectId: string;
let epic: string, story: string, feature: string, task: string, subtask: string, done: string, qa: string;

// Field lists ORCA sends (jira-issue-mapping.ts).
const ISSUE_FIELDS = ["summary", "description", "project", "issuetype", "status", "assignee", "reporter", "priority", "labels", "created", "updated"];
const ISSUE_LIST_FIELDS = ISSUE_FIELDS.filter((f) => f !== "description");

// ORCA's textToAdf(): one paragraph per line.
const textToAdf = (text: string) => ({
  type: "doc", version: 1,
  content: text.split(/\r?\n/).map((line) => ({ type: "paragraph", content: line ? [{ type: "text", text: line }] : [] })),
});

const search = (jql: string, extra: Record<string, unknown> = {}) =>
  jira.post("/rest/api/3/search/jql", { jql, maxResults: 30, fields: ISSUE_LIST_FIELDS, ...extra });
const keys = (res: any) => res.body.issues.map((i: any) => i.key);

beforeAll(async () => {
  srv = await startServer();
  jira = srv.jira("owner");
  owner = srv.as("owner");
  await owner.post("/api/v1/projects", { key: "APP", name: "App" });
  const mk = async (body: Record<string, unknown>) => (await owner.post("/api/v1/issues", { project_key: "APP", ...body })).body.key;
  epic = await mk({ type: "epic", title: "Auth epic" });
  story = await mk({ type: "story", title: "Log in story", parent_key: epic, labels: ["frontend"] });
  feature = await mk({ type: "feature", title: "Login screen", parent_key: story, priority: 2 });
  task = await mk({ type: "task", title: "Build login form", parent_key: feature, assignee: "owner", priority: 1,
    labels: ["frontend", "urgent"], description: "## 手順\n\n- **フォーム**を作る\n- `zod` で検証" });
  subtask = await mk({ type: "task", title: "Password rule", parent_key: task, assignee: "claude" });
  done = await mk({ type: "task", title: "Finished task", assignee: "owner" });
  const states = (await owner.get("/api/v1/projects/APP/states")).body.states;
  await owner.patch(`/api/v1/issues/${done}`, { state_id: states.find((s: any) => s.name === "Done").id });
  const agent = await srv.mcpHttp("claude");
  qa = (await callTool(agent, "ask_question", { target_key: task, questions: [{ question: "OK?", answer_type: "ok_ng" }] })).key;
  await callTool(agent, "record_checkpoint", { key: task, summary: "Form skeleton done", rationale: "Matches the design", confidence: "low", progress: 30 });
  projectId = (await jira.get("/rest/api/3/project/search")).body.values[0].id;
});
afterAll(() => srv?.stop());

describe("authentication", () => {
  it("accepts Basic email:token (Cloud mode), username:token and Bearer", async () => {
    for (const auth of [basicAuth("owner@taskapp.local", TOKENS.owner), basicAuth("owner", TOKENS.owner), `Bearer ${TOKENS.owner}`]) {
      const res = await srv.raw(auth).get("/rest/api/3/myself");
      expect(res.status).toBe(200);
      expect(res.body.emailAddress).toBe("owner@taskapp.local");
    }
  });

  it("rejects missing, invalid or mismatched credentials with 401 and Jira's error shape", async () => {
    for (const auth of [undefined, basicAuth("owner@taskapp.local", "tk_nope"), basicAuth("alice@taskapp.local", TOKENS.owner), "Bearer tk_nope"]) {
      const res = await srv.raw(auth).get("/rest/api/3/myself");
      expect(res.status).toBe(401);
      expect(res.body).toEqual({ errorMessages: [expect.stringContaining("not authenticated")], errors: {} });
    }
  });

  it("describes the caller and the server", async () => {
    const me = (await jira.get("/rest/api/3/myself")).body;
    expect(me).toMatchObject({ displayName: expect.any(String), emailAddress: "owner@taskapp.local", accountType: "atlassian", active: true });
    expect(me.accountId).toMatch(/^[0-9A-Z]{26}$/);
    expect(me.avatarUrls["48x48"]).toMatch(/^http:\/\/.+\.svg$/);
    const agent = (await srv.jira("claude").get("/rest/api/3/myself")).body;
    expect(agent.accountType).toBe("app");
    const info = (await jira.get("/rest/api/3/serverInfo")).body;
    expect(info).toMatchObject({ deploymentType: "Cloud", baseUrl: srv.baseUrl });
  });
});

describe("projects", () => {
  it("pages /project/search the way ORCA reads it", async () => {
    await owner.post("/api/v1/projects", { key: "OPS", name: "Operations" });
    const all = (await jira.get("/rest/api/3/project/search?maxResults=100&startAt=0")).body;
    expect(all).toMatchObject({ startAt: 0, maxResults: 100, total: 2, isLast: true });
    expect(all.values.map((p: any) => p.key)).toEqual(["APP", "OPS"]);
    expect(all.values[0]).toMatchObject({ id: expect.stringMatching(/^\d+$/), key: "APP", name: "App", projectTypeKey: "software" });

    const first = (await jira.get("/rest/api/3/project/search?maxResults=1&startAt=0")).body;
    expect(first).toMatchObject({ total: 2, isLast: false, values: [{ key: "APP" }] });
    const second = (await jira.get("/rest/api/3/project/search?maxResults=1&startAt=1")).body;
    expect(second).toMatchObject({ isLast: true, values: [{ key: "OPS" }] });
  });

  it("returns a project by key or id with its issue types", async () => {
    const byKey = (await jira.get("/rest/api/3/project/APP")).body;
    const byId = (await jira.get(`/rest/api/3/project/${projectId}`)).body;
    expect(byId.key).toBe("APP");
    expect(byKey.issueTypes.map((t: any) => t.name)).toEqual(["Epic", "Story", "Feature", "Task", "Question"]);
    expect(byKey.lead.emailAddress).toBe("owner@taskapp.local");
    expect((await jira.get("/rest/api/3/project/NOPE")).status).toBe(404);
  });
});

describe("search", () => {
  it("answers ORCA's built-in filters", async () => {
    // filterToJql('assigned' | 'reported' | 'done' | 'all')
    // The QA is assigned to the project owner by default, so it is "mine" too.
    expect(keys(await search("assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC")).sort()).toEqual([task, qa].sort());
    expect(keys(await search("assignee = currentUser() AND resolution IS NOT EMPTY ORDER BY updated DESC"))).toEqual([done]);
    const reported = keys(await search("reporter = currentUser() AND resolution = Unresolved ORDER BY updated DESC"));
    expect(reported).toEqual(expect.arrayContaining([epic, story, feature, task, subtask]));
    expect(reported).not.toContain(qa); // asked by the agent
    expect(keys(await search("resolution = Unresolved ORDER BY updated DESC"))).toHaveLength(6);
  });

  it("renders issues in v3 form", async () => {
    const issue = (await search(`key = "${task}"`)).body.issues[0];
    expect(issue).toMatchObject({ id: expect.stringMatching(/^\d+$/), key: task, self: expect.stringContaining("/rest/api/3/issue/") });
    const f = issue.fields;
    expect(f.summary).toBe("Build login form");
    expect(f.description).toBeUndefined(); // not requested
    expect(f.project).toMatchObject({ id: projectId, key: "APP", name: "App" });
    expect(f.issuetype).toMatchObject({ id: "10003", name: "Task", subtask: false, hierarchyLevel: 0 });
    expect(f.status).toMatchObject({ name: "Todo", id: expect.stringMatching(/^\d+$/), statusCategory: { key: "new", name: "To Do", id: 2 } });
    expect(f.assignee).toMatchObject({ emailAddress: "owner@taskapp.local", displayName: expect.any(String) });
    expect(f.reporter.accountId).toMatch(/^[0-9A-Z]{26}$/);
    expect(f.priority).toMatchObject({ id: "1", name: "Highest" });
    expect(f.labels).toEqual(["frontend", "urgent"]);
    expect(f.created).toMatch(/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}\+0000$/);
    expect(f.updated).toMatch(/\+0000$/);
  });

  it("always returns the fields clients rely on, even when not requested", async () => {
    const issue = (await search(`key = ${task}`, { fields: ["summary"] })).body.issues[0];
    expect(Object.keys(issue.fields).sort()).toEqual(["created", "issuetype", "project", "status", "summary", "updated"]);
  });

  it("supports the JQL subset", async () => {
    const cases: [string, string[]][] = [
      [`project = APP AND issuetype = Epic`, [epic]],
      [`issuetype in (Story, Feature) ORDER BY key`, [story, feature]],
      [`type = Question`, [qa]],
      [`status = "Done"`, [done]],
      [`statusCategory = Done`, [done]],
      [`statusCategory != Done AND assignee = claude ORDER BY key`, [subtask]],
      [`assignee = "claude@taskapp.local"`, [subtask]],
      [`assignee is EMPTY AND issuetype != Question ORDER BY key`, [epic, story, feature]],
      [`parent = ${task}`, [subtask, qa]],
      [`labels = urgent`, [task]],
      [`labels in (frontend) ORDER BY key DESC`, [task, story]],
      [`priority = High`, [feature]],
      [`priority in (Highest, High) ORDER BY priority DESC`, [task, feature]],
      [`text ~ "login*" ORDER BY key`, [feature, task]], // "Log in story" does not contain "login",
      [`summary ~ "form"`, [task]],
      [`key in (${epic}, ${done}) ORDER BY key`, [epic, done]],
      [`created >= -1d AND issuetype = Epic`, [epic]],
      [`project = ${projectId} AND issuetype = Epic`, [epic]],
      [`NOT (issuetype = Task OR issuetype = Question) ORDER BY key`, [epic, story, feature]],
    ];
    for (const [jql, want] of cases) {
      const res = await search(jql, { maxResults: 100 });
      expect(res.status, `${jql}: ${JSON.stringify(res.body)}`).toBe(200);
      const got = keys(res);
      expect(jql.includes("ORDER BY") ? got : [...got].sort(), jql).toEqual(jql.includes("ORDER BY") ? want : [...want].sort());
    }
  });

  it("pages with nextPageToken and supports GET", async () => {
    const p1 = (await search("project = APP ORDER BY key", { maxResults: 3 })).body;
    expect(p1.issues).toHaveLength(3);
    expect(p1.isLast).toBe(false);
    expect(p1.nextPageToken).toEqual(expect.any(String));
    const p2 = (await search("project = APP ORDER BY key", { maxResults: 3, nextPageToken: p1.nextPageToken })).body;
    const p3 = (await search("project = APP ORDER BY key", { maxResults: 3, nextPageToken: p2.nextPageToken })).body;
    expect(p3.isLast).toBe(true);
    expect(p3.nextPageToken).toBeUndefined();
    const all = [...p1.issues, ...p2.issues, ...p3.issues].map((i: any) => i.key);
    expect(new Set(all).size).toBe(7);

    const get = await jira.get(`/rest/api/3/search/jql?jql=${encodeURIComponent(`key = ${task}`)}&fields=summary,labels`);
    expect(get.status).toBe(200);
    expect(get.body.issues[0].fields.labels).toEqual(["frontend", "urgent"]);
  });

  it("serves the classic /search with startAt and total", async () => {
    const res = (await jira.post("/rest/api/3/search", { jql: "project = APP ORDER BY key", startAt: 2, maxResults: 2 })).body;
    expect(res).toMatchObject({ startAt: 2, maxResults: 2, total: 7 });
    expect(res.issues).toHaveLength(2);
  });

  it("reports JQL errors the way Jira does", async () => {
    for (const jql of ["assignee = ", "sprint = 5", "issuetype = Bug", "project = APP ORDER BY nope", 'summary ~ "unterminated']) {
      const res = await search(jql);
      expect(res.status, jql).toBe(400);
      expect(res.body.errorMessages[0], jql).toMatch(/^Error in the JQL Query: /);
      expect(res.body.errors).toEqual({});
    }
  });
});

describe("issue detail", () => {
  it("returns an issue by key or numeric id, with ADF description and parent", async () => {
    const params = `fields=${[...ISSUE_FIELDS, "attachment"].join(",")}&expand=renderedFields`;
    const res = await jira.get(`/rest/api/3/issue/${task}?${params}`);
    expect(res.status).toBe(200);
    const byId = await jira.get(`/rest/api/3/issue/${res.body.id}?fields=summary`);
    expect(byId.body.key).toBe(task);

    const f = res.body.fields;
    expect(f.attachment).toEqual([]);
    expect(f.description).toEqual({
      type: "doc", version: 1, content: [
        { type: "heading", attrs: { level: 2 }, content: [{ type: "text", text: "手順" }] },
        { type: "bulletList", content: [
          { type: "listItem", content: [{ type: "paragraph", content: [{ type: "text", text: "フォーム", marks: [{ type: "strong" }] }, { type: "text", text: "を作る" }] }] },
          { type: "listItem", content: [{ type: "paragraph", content: [{ type: "text", text: "zod", marks: [{ type: "code" }] }, { type: "text", text: " で検証" }] }] },
        ] },
      ],
    });
    const parent = (await jira.get(`/rest/api/3/issue/${subtask}?fields=parent`)).body.fields.parent;
    expect(parent).toMatchObject({ key: task, fields: { summary: "Build login form", issuetype: { name: "Task" }, status: { name: "Todo" } } });
    expect((await jira.get(`/rest/api/3/issue/${epic}?fields=summary,description`)).body.fields.description).toBeNull();
  });

  it("returns resolution for finished issues", async () => {
    const f = (await jira.get(`/rest/api/3/issue/${done}?fields=resolution,resolutiondate,status`)).body.fields;
    expect(f.resolution).toMatchObject({ name: "Done" });
    expect(f.resolutiondate).toMatch(/\+0000$/);
    expect(f.status.statusCategory.key).toBe("done");
  });

  it("returns Jira's 404 for unknown issues", async () => {
    for (const ref of ["APP-999", "99999"]) {
      const res = await jira.get(`/rest/api/3/issue/${ref}`);
      expect(res.status).toBe(404);
      expect(res.body.errorMessages).toEqual(["Issue does not exist or you do not have permission to see it."]);
    }
  });
});

describe("create and edit", () => {
  it("creates an issue from ORCA's request (project id, issue type id, ADF description)", async () => {
    const meta = (await jira.get(`/rest/api/3/issue/createmeta/${projectId}/issuetypes?maxResults=100&startAt=0`)).body;
    expect(meta.issueTypes.map((t: any) => t.name)).toEqual(["Epic", "Story", "Feature", "Task"]);
    const taskType = meta.issueTypes.find((t: any) => t.name === "Task");

    const res = await jira.post("/rest/api/3/issue", {
      fields: { project: { id: projectId }, issuetype: { id: taskType.id }, summary: "Created from ORCA", description: textToAdf("line 1\nline 2") },
    });
    expect(res.status).toBe(201);
    expect(res.body).toEqual({ id: expect.stringMatching(/^\d+$/), key: expect.stringMatching(/^APP-\d+$/), self: expect.stringContaining("/rest/api/3/issue/") });

    const created = (await owner.get(`/api/v1/issues/${res.body.key}`)).body;
    expect(created).toMatchObject({ type: "task", title: "Created from ORCA", description: "line 1\n\nline 2", parent_key: null });
    const activity = (await owner.get(`/api/v1/issues/${res.body.key}/activity`)).body.activity;
    expect(activity[0]).toMatchObject({ action: "created", source: "jira" });
  });

  it("accepts parent, assignee, priority, labels and due date on create", async () => {
    const me = (await jira.get("/rest/api/3/myself")).body;
    const res = await jira.post("/rest/api/3/issue", {
      fields: {
        project: { key: "APP" }, issuetype: { name: "Task" }, summary: "Child via Jira", parent: { key: feature },
        assignee: { accountId: me.accountId }, priority: { id: "3" }, labels: ["jira"], duedate: "2026-12-01",
      },
    });
    expect(res.status).toBe(201);
    const is = (await owner.get(`/api/v1/issues/${res.body.key}`)).body;
    expect(is).toMatchObject({ parent_key: feature, assignee: { username: "owner" }, priority: 3, due_date: "2026-12-01" });
    expect(is.labels.map((l: any) => l.name)).toEqual(["jira"]);
  });

  it("validates create requests with field errors", async () => {
    const res = await jira.post("/rest/api/3/issue", { fields: { project: { id: "1" }, issuetype: { id: "999" } } });
    expect(res.status).toBe(400);
    expect(Object.keys(res.body.errors).sort()).toEqual(["issuetype", "project", "summary"]);

    for (const issuetype of [{ id: "10004" }, { name: "Question" }]) {
      const q = await jira.post("/rest/api/3/issue", { fields: { project: { key: "APP" }, issuetype, summary: "x" } });
      expect(q.status).toBe(400);
      expect(q.body.errors.issuetype).toMatch(/ask_question/);
    }
    expect((await jira.put(`/rest/api/3/issue/${epic}`, { fields: { issuetype: { id: "10004" } } })).status).toBe(400);

    const bad = await jira.post("/rest/api/3/issue", { fields: { project: { key: "APP" }, issuetype: { name: "Story" }, summary: "x", parent: { key: task } } });
    expect(bad.status).toBe(400);
    expect(bad.body.errorMessages[0]).toMatch(/stories can only be placed under epics/);
  });

  it("describes create fields so that ORCA shows no extra inputs", async () => {
    const res = (await jira.get(`/rest/api/3/issue/createmeta/${projectId}/issuetypes/10003?maxResults=100&startAt=0`)).body;
    const required = res.fields.filter((f: any) => f.required).map((f: any) => f.key).sort();
    expect(required).toEqual(["issuetype", "project", "summary"]);
    expect(res.fields.map((f: any) => f.key)).toContain("parent");
    const epicFields = (await jira.get(`/rest/api/3/issue/createmeta/APP/issuetypes/10000`)).body.fields;
    expect(epicFields.map((f: any) => f.key)).not.toContain("parent");
  });

  it("edits fields and answers 204 with an empty body", async () => {
    const res = await jira.put(`/rest/api/3/issue/${feature}`, { fields: { summary: "Login screen v2", labels: ["ui"], priority: { id: "4" } } });
    expect(res.status).toBe(204);
    expect(res.body).toBeNull();
    let is = (await owner.get(`/api/v1/issues/${feature}`)).body;
    expect(is).toMatchObject({ title: "Login screen v2", priority: 4 });
    expect(is.labels.map((l: any) => l.name)).toEqual(["ui"]);

    expect((await jira.put(`/rest/api/3/issue/${feature}`, { fields: { priority: null } })).status).toBe(204);
    expect((await jira.put(`/rest/api/3/issue/${feature}`, { update: { labels: [{ add: "backend" }, { remove: "ui" }] } })).status).toBe(204);
    expect((await jira.put(`/rest/api/3/issue/${feature}`, { fields: { description: textToAdf("New *spec*") } })).status).toBe(204);
    is = (await owner.get(`/api/v1/issues/${feature}`)).body;
    expect(is).toMatchObject({ priority: 0, description: "New *spec*" });
    expect(is.labels.map((l: any) => l.name)).toEqual(["backend"]);
  });

  it("assigns through /assignee", async () => {
    const claude = (await srv.jira("claude").get("/rest/api/3/myself")).body;
    expect((await jira.put(`/rest/api/3/issue/${feature}/assignee`, { accountId: claude.accountId })).status).toBe(204);
    expect((await owner.get(`/api/v1/issues/${feature}`)).body.assignee.username).toBe("claude");
    expect((await jira.put(`/rest/api/3/issue/${feature}/assignee`, { accountId: null })).status).toBe(204);
    expect((await owner.get(`/api/v1/issues/${feature}`)).body.assignee).toBeNull();
    const bad = await jira.put(`/rest/api/3/issue/${feature}/assignee`, { accountId: "nobody" });
    expect(bad.status).toBe(400);
  });
});

describe("transitions", () => {
  it("lists every state of the issue's workflow as a transition", async () => {
    const res = (await jira.get(`/rest/api/3/issue/${story}/transitions`)).body;
    expect(res.transitions.map((t: any) => t.name)).toEqual(["Backlog", "Todo", "In Progress", "In Review", "受入待ち", "Done", "Canceled"]);
    const inProgress = res.transitions.find((t: any) => t.name === "In Progress");
    expect(inProgress).toMatchObject({ id: inProgress.to.id, to: { name: "In Progress", statusCategory: { key: "indeterminate" } } });
    const qaStates = (await jira.get(`/rest/api/3/issue/${qa}/transitions`)).body.transitions.map((t: any) => t.name);
    expect(qaStates).toEqual(["未回答", "回答済", "クローズ"]);
  });

  it("moves an issue and answers 204", async () => {
    const tr = (await jira.get(`/rest/api/3/issue/${story}/transitions`)).body.transitions;
    const id = tr.find((t: any) => t.name === "In Progress").id;
    const res = await jira.post(`/rest/api/3/issue/${story}/transitions`, { transition: { id } });
    expect(res.status).toBe(204);
    expect(res.body).toBeNull();
    const is = (await owner.get(`/api/v1/issues/${story}`)).body;
    expect(is.state.name).toBe("In Progress");
    expect(is.started_at).not.toBeNull();

    const qaState = (await jira.get(`/rest/api/3/issue/${qa}/transitions`)).body.transitions[0].id;
    const bad = await jira.post(`/rest/api/3/issue/${story}/transitions`, { transition: { id: qaState } });
    expect(bad.status).toBe(400);
    expect(bad.body.errorMessages[0]).toMatch(/is not valid for this issue/);
  });
});

describe("comments and checkpoints", () => {
  it("adds an ADF comment and lists comments with checkpoints merged in", async () => {
    const res = await jira.post(`/rest/api/3/issue/${task}/comment`, { body: textToAdf("Looks good\nShip it") });
    expect(res.status).toBe(201);
    expect(res.body).toMatchObject({ id: expect.stringMatching(/^\d+$/), author: { emailAddress: "owner@taskapp.local" }, body: { type: "doc" } });
    const internal = (await owner.get(`/api/v1/issues/${task}/comments`)).body.comments;
    expect(internal.at(-1)).toMatchObject({ body: "Looks good\n\nShip it", source: "jira" });

    const list = (await jira.get(`/rest/api/3/issue/${task}/comment?maxResults=100&orderBy=created&startAt=0&expand=renderedBody`)).body;
    expect(list).toMatchObject({ startAt: 0, total: 2 });
    expect(list.comments).toHaveLength(2);
    const [cp, comment] = list.comments; // the checkpoint came first
    expect(JSON.stringify(cp.body)).toContain("[Checkpoint] #1 Form skeleton done");
    expect(JSON.stringify(cp.body)).toContain("Matches the design");
    expect(cp.author.accountType).toBe("app");
    expect(comment.id).toBe(res.body.id);

    const newest = (await jira.get(`/rest/api/3/issue/${task}/comment?orderBy=-created`)).body.comments;
    expect(newest[0].id).toBe(res.body.id);
    const paged = (await jira.get(`/rest/api/3/issue/${task}/comment?maxResults=1&startAt=1`)).body;
    expect(paged).toMatchObject({ total: 2, startAt: 1, comments: [{ id: res.body.id }] });
  });

  it("records a [Checkpoint] comment as a checkpoint", async () => {
    const body = textToAdf("[Checkpoint] Validation added\n判断: zod を使う\n判断基準: 既存の依存で済む\n代替案: yup / 手書き\n確信度: 高\n進捗: 60%\n次: エラー表示");
    const res = await srv.jira("claude").post(`/rest/api/3/issue/${task}/comment`, { body });
    expect(res.status).toBe(201);
    const cps = (await owner.get(`/api/v1/issues/${task}/checkpoints`)).body.checkpoints;
    expect(cps.at(-1)).toMatchObject({
      seq: 2, summary: "Validation added", decision: "zod を使う", rationale: "既存の依存で済む", alternatives: ["yup", "手書き"],
      confidence: "high", progress: 60, next_steps: "エラー表示", source: "jira", author: { username: "claude" },
    });
    const bad = await jira.post(`/rest/api/3/issue/${task}/comment`, { body: textToAdf("[Checkpoint] no rationale") });
    expect(bad.status).toBe(400);
    expect(bad.body.errors.comment).toMatch(/判断基準/);
  });

  it("includes comments in the issue when asked for the comment field", async () => {
    const f = (await jira.get(`/rest/api/3/issue/${task}?fields=summary,comment`)).body.fields;
    expect(f.comment.total).toBe(3); // checkpoint #1, the comment, checkpoint #2
    expect(f.comment.comments).toHaveLength(3);
  });
});

describe("reference data", () => {
  it("lists priorities, issue types and users", async () => {
    expect((await jira.get("/rest/api/3/priority")).body.map((p: any) => [p.id, p.name])).toEqual([["1", "Highest"], ["2", "High"], ["3", "Medium"], ["4", "Low"]]);
    expect((await jira.get("/rest/api/3/issuetype")).body.map((t: any) => t.id)).toEqual(["10000", "10001", "10002", "10003", "10004"]);
    const everyone = (await jira.get("/rest/api/3/user/search?maxResults=50&query=")).body;
    expect(everyone.map((u: any) => u.emailAddress).sort()).toEqual(["alice@taskapp.local", "claude@taskapp.local", "codex@taskapp.local", "owner@taskapp.local"]);
    expect((await jira.get("/rest/api/3/user/search?query=ali")).body.map((u: any) => u.displayName)).toHaveLength(1);
    const assignable = (await jira.get(`/rest/api/3/user/assignable/search?issueKey=${task}&maxResults=50&query=cla`)).body;
    expect(assignable.map((u: any) => u.emailAddress)).toEqual(["claude@taskapp.local"]);
  });

  it("serves avatars without authentication", async () => {
    const me = (await jira.get("/rest/api/3/myself")).body;
    const res = await fetch(me.avatarUrls["48x48"]);
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type")).toBe("image/svg+xml");
    expect((await fetch(srv.baseUrl + "/rest/api/3/avatar/issuetype/10003.svg")).status).toBe(200);
  });
});

describe("agile board", () => {
  it("exposes exactly one board per project with its columns in workflow order", async () => {
    const boards = (await jira.get("/rest/agile/1.0/board?projectKeyOrId=APP&maxResults=2")).body;
    expect(boards).toMatchObject({ total: 1, isLast: true, values: [{ type: "kanban", location: { projectKey: "APP" } }] });
    const config = (await jira.get(`/rest/agile/1.0/board/${boards.values[0].id}/configuration`)).body;
    const columns = config.columnConfig.columns;
    expect(columns.map((c: any) => c.name)).toEqual(["Backlog", "Todo", "In Progress", "In Review", "受入待ち", "Done", "Canceled"]);
    const transitions = (await jira.get(`/rest/api/3/issue/${story}/transitions`)).body.transitions;
    expect(columns.map((c: any) => c.statuses[0].id)).toEqual(transitions.map((t: any) => t.to.id));
    expect((await jira.get("/rest/agile/1.0/board/123/configuration")).status).toBe(404);
  });
});

describe("misc", () => {
  it("redirects /browse/KEY to the issue page", async () => {
    const res = await srv.raw().get(`/browse/${task}`);
    expect(res.status).toBe(302);
    expect(res.headers.get("location")).toBe(`/p/APP/issues/${task}`);
  });

  it("answers unknown endpoints with Jira's error shape", async () => {
    const res = await jira.get("/rest/api/3/dashboard");
    expect(res.status).toBe(404);
    expect(res.body.errorMessages[0]).toMatch(/does not implement/);
  });

  it("publishes live updates for changes made through Jira", async () => {
    const res = await fetch(srv.baseUrl + "/api/v1/events", { headers: { Authorization: `Bearer ${TOKENS.owner}` } });
    const reader = res.body!.getReader();
    const received = (async () => {
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) return "";
        buf += new TextDecoder().decode(value);
        if (buf.includes(`"issue_key":"${epic}"`)) return buf;
      }
    })();
    await jira.put(`/rest/api/3/issue/${epic}`, { fields: { summary: "Auth epic (renamed in ORCA)" } });
    const stream = await Promise.race([received, new Promise<string>((r) => setTimeout(() => r("timeout"), 5000))]);
    await reader.cancel();
    expect(stream).toContain('"type":"issue.updated"');
  });
});
