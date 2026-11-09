import { createRootRoute, createRoute, createRouter, Link, Outlet, redirect, useNavigate, useRouterState } from "@tanstack/react-router";
import { Bookmark, Bot, ClipboardCheck, FolderKanban, Inbox, Kanban, LogOut, MessageCircleQuestion, Plus, Search, Settings } from "lucide-react";
import { markLoggedOut } from "../api/client";
import type { ReactNode } from "react";
import { useAcceptanceQueue, useIssues, useLiveUpdates, useMe, useProjects, useViews } from "../api/hooks";
import { CommandPalette, openCommandPalette } from "../components/CommandPalette";
import { CreateIssueProvider, useCreateIssueModal } from "../components/CreateIssue";
import { Avatar } from "../components/icons";
import type { FilterState } from "../lib/filters";
import { tierRank } from "../lib/meta";
import { TierIcon } from "../components/pickers";
import { AcceptanceInboxPage } from "./acceptancePage";
import { SavedViewPage } from "./viewsPage";
import { ProjectSettingsPage } from "./projectSettings";
import { AgentsPage, QAInboxPage, SettingsPage, type InboxTab } from "./agentPages";
import { LoginPage } from "./loginPage";
import { StoryMapPage, type StoryMapSearch } from "./storyMapPage";
import { AllBoardPage, IssuePage, MyIssuesPage, ProjectIssuesPage, ProjectsPage, type IssuesSearch, type MyTab } from "./pages";

function parseIssuesSearch(s: Record<string, unknown>): IssuesSearch {
  const pick = <T extends string>(v: unknown, allowed: readonly T[]) => (allowed.includes(v as T) ? (v as T) : undefined);
  return {
    view: pick(s.view, ["list", "board", "tree", "map", "roadmap"] as const),
    group: pick(s.group, ["state", "assignee", "type", "priority", "none"] as const),
    sort: pick(s.sort, ["updated", "created", "priority", "manual", "key", "due_date"] as const),
    f: s.f && typeof s.f === "object" ? (s.f as FilterState) : undefined,
    peek: typeof s.peek === "string" ? s.peek : undefined,
    root: typeof s.root === "string" ? s.root : undefined,
    lanes: s.lanes === "none" ? "none" : undefined,
  };
}

function Shell() {
  useLiveUpdates();
  const navigate = useNavigate();
  const params = useRouterState({ select: (s) => s.matches.at(-1)?.params as { key?: string } | undefined });
  const projectKey = params?.key;
  return (
    <CreateIssueProvider currentProject={projectKey} onCreated={(key) => navigate({ to: "/p/$key/issues", params: { key: key.split("-")[0] }, search: (s: IssuesSearch) => ({ ...s, peek: key }) })}>
      <div className="flex h-full">
        <Sidebar />
        <main className="min-w-0 flex-1"><Outlet /></main>
        <CommandPalette />
      </div>
    </CreateIssueProvider>
  );
}

function Sidebar() {
  const { data: me } = useMe();
  const { data: projects = [] } = useProjects();
  const openCreate = useCreateIssueModal();
  const { data: qa } = useIssues({
    filter: { and: [{ field: "type", op: "eq", value: "qa" }, { field: "assignee", op: "eq", value: "me" }, { field: "state_category", op: "in", value: ["backlog", "unstarted"] }] },
    limit: 1,
  });
  const { data: acceptance } = useAcceptanceQueue();
  const { data: views = [] } = useViews();
  return (
    <nav className="flex w-[232px] shrink-0 flex-col border-r border-line bg-sidebar px-2 py-2 text-[13px]">
      <div className="mb-2 flex items-center gap-2 px-2 py-1">
        <span className="flex h-5 w-5 items-center justify-center rounded-md bg-accent text-[11px] font-bold text-white">t</span>
        <span className="font-semibold">taskapp</span>
        <span className="flex-1" />
        <button title="新規イシュー (C)" onClick={() => openCreate()} className="rounded-md p-1 text-muted hover:bg-hover hover:text-fg"><Plus size={15} /></button>
      </div>
      <button onClick={openCommandPalette}
        className="mb-2 flex items-center gap-2 rounded-md border border-line bg-bg px-2 py-1 text-left text-[12.5px] text-faint hover:text-muted">
        <Search size={13} /><span className="flex-1">検索・移動</span><kbd className="rounded border border-line px-1 text-[10.5px]">⌘K</kbd>
      </button>
      <NavLink to="/my" icon={<Inbox size={15} />}>マイタスク</NavLink>
      <NavLink to="/board" icon={<Kanban size={15} />}>全体ボード</NavLink>
      <NavLink to="/inbox/qa" icon={<MessageCircleQuestion size={15} />} badge={qa?.total || undefined}>QA インボックス</NavLink>
      <NavLink to="/inbox/acceptance" icon={<ClipboardCheck size={15} />} badge={acceptance?.length || undefined}>受入検査</NavLink>
      <NavLink to="/agents" icon={<Bot size={15} />}>エージェント活動</NavLink>
      <NavLink to="/projects" icon={<FolderKanban size={15} />}>プロジェクト</NavLink>
      <div className="mb-1 mt-4 px-2 text-[11px] font-medium text-faint">プロジェクト</div>
      {[...projects].sort((a, b) => tierRank(a.tier) - tierRank(b.tier)).map((p) => (
        <NavLink key={p.key} to="/p/$key/issues" params={{ key: p.key }} icon={<span className="font-mono text-[10.5px] text-faint">{p.key}</span>}>
          <span className={`flex items-center gap-1 ${p.tier === "side" ? "text-faint" : ""}`}>
            <span className="truncate">{p.name}</span>{p.tier !== "normal" && <TierIcon tier={p.tier} size={11} />}
          </span>
        </NavLink>
      ))}
      {views.length > 0 && <div className="mb-1 mt-4 px-2 text-[11px] font-medium text-faint">ビュー</div>}
      {views.map((v) => (
        <NavLink key={v.id} to="/views/$id" params={{ id: v.id }} icon={<Bookmark size={13} />}>
          {v.name}{v.project_key && <span className="ml-1.5 font-mono text-[10.5px] text-faint">{v.project_key}</span>}
        </NavLink>
      ))}
      <span className="flex-1" />
      <NavLink to="/settings" icon={<Settings size={15} />}>設定</NavLink>
      {me && (
        <div className="flex items-center gap-2 rounded-md px-2 py-1.5 text-muted">
          <Avatar user={me} size={18} /> <span className="flex-1 truncate">{me.display_name}</span>
          <button title="ログアウト" onClick={logout} className="rounded p-1 text-faint hover:bg-hover hover:text-fg"><LogOut size={14} /></button>
        </div>
      )}
    </nav>
  );
}

async function logout() {
  // Remember the choice so that this machine does not log straight back in.
  markLoggedOut(true);
  await fetch("/api/v1/auth/logout", { method: "POST" }).catch(() => null);
  window.location.assign("/login");
}

function NavLink({ to, params, search, icon, badge, active, children }: {
  to: string; params?: Record<string, string>; search?: Record<string, unknown>; icon: ReactNode; badge?: number;
  /** overrides the router's active detection (for tabs that share a path) */
  active?: boolean;
  children: ReactNode;
}) {
  const base = "flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-hover hover:text-fg";
  return (
    <Link to={to} params={params} search={search}
      className={active === undefined ? `${base} text-muted` : `${base} ${active ? "bg-active text-fg" : "text-muted"}`}
      activeProps={active === undefined ? { className: "!bg-active !text-fg" } : undefined}>
      <span className="flex w-5 justify-center">{icon}</span>
      <span className="flex-1 truncate">{children}</span>
      {badge ? <span className="rounded-full bg-[#e0823d] px-1.5 text-[10.5px] font-semibold text-white">{badge}</span> : null}
    </Link>
  );
}

const rootRoute = createRootRoute({ component: Outlet });

// Everything except the login page lives in the app shell (sidebar, live updates).
const appRoute = createRoute({ getParentRoute: () => rootRoute, id: "app", component: Shell });

const loginRoute = createRoute({ getParentRoute: () => rootRoute, path: "/login", component: LoginPage });

const indexRoute = createRoute({ getParentRoute: () => appRoute, path: "/", beforeLoad: () => { throw redirect({ to: "/my" }); } });

const myRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/my",
  validateSearch: (s: Record<string, unknown>): IssuesSearch & { tab?: MyTab } => ({
    ...parseIssuesSearch(s),
    tab: (["assigned", "created", "qa"] as const).includes(s.tab as MyTab) ? (s.tab as MyTab) : undefined,
  }),
  component: function My() {
    const search = myRoute.useSearch();
    const navigate = useNavigate();
    return <MyIssuesPage tab={search.tab ?? "assigned"} search={search} setSearch={(s) => navigate({ to: "/my", search: s, replace: true })} />;
  },
});

const boardRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/board",
  validateSearch: parseIssuesSearch,
  component: function Board() {
    const search = boardRoute.useSearch();
    const navigate = useNavigate();
    return <AllBoardPage search={search} setSearch={(s) => navigate({ to: "/board", search: s, replace: true })} />;
  },
});

const projectsRoute = createRoute({ getParentRoute: () => appRoute, path: "/projects", component: ProjectsPage });

const projectIssuesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/p/$key/issues",
  validateSearch: parseIssuesSearch,
  component: function ProjectIssues() {
    const { key } = projectIssuesRoute.useParams();
    const search = projectIssuesRoute.useSearch();
    const navigate = useNavigate();
    return <ProjectIssuesPage key={key} projectKey={key} search={search}
      setSearch={(s) => navigate({ to: "/p/$key/issues", params: { key }, search: s, replace: true })} />;
  },
});

const storyMapRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/p/$key/storymap",
  validateSearch: (s: Record<string, unknown>): StoryMapSearch => ({
    persona: typeof s.persona === "string" ? s.persona : undefined,
    peek: typeof s.peek === "string" ? s.peek : undefined,
    panel: s.panel === true || s.panel === "true" ? true : undefined,
  }),
  component: function StoryMapRoute() {
    const { key } = storyMapRoute.useParams();
    const search = storyMapRoute.useSearch();
    const navigate = useNavigate();
    return <StoryMapPage key={key} projectKey={key} search={search}
      setSearch={(s) => navigate({ to: "/p/$key/storymap", params: { key }, search: s, replace: true })} />;
  },
});

const issueRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/p/$key/issues/$issueKey",
  component: function IssueRoute() {
    const { issueKey } = issueRoute.useParams();
    return <IssuePage key={issueKey} issueKey={issueKey} />;
  },
});

const qaInboxRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/inbox/qa",
  validateSearch: (s: Record<string, unknown>): { tab?: InboxTab; all?: boolean; key?: string } => ({
    tab: (["open", "answered", "closed", "all"] as const).includes(s.tab as InboxTab) ? (s.tab as InboxTab) : undefined,
    all: s.all === true || s.all === "true" ? true : undefined,
    key: typeof s.key === "string" ? s.key : undefined,
  }),
  component: function QAInbox() {
    const s = qaInboxRoute.useSearch();
    const navigate = useNavigate();
    return <QAInboxPage tab={s.tab ?? "open"} mine={!s.all} selected={s.key} setSearch={(n) => navigate({ to: "/inbox/qa", search: n, replace: true })} />;
  },
});

const agentsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/agents",
  validateSearch: (s: Record<string, unknown>): { agent?: string; low?: boolean; peek?: string } => ({
    agent: typeof s.agent === "string" ? s.agent : undefined,
    low: s.low === true || s.low === "true" ? true : undefined,
    peek: typeof s.peek === "string" ? s.peek : undefined,
  }),
  component: function Agents() {
    const s = agentsRoute.useSearch();
    const navigate = useNavigate();
    return <AgentsPage agent={s.agent} low={!!s.low} peek={s.peek} setSearch={(n) => navigate({ to: "/agents", search: n, replace: true })} />;
  },
});

const acceptanceRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/inbox/acceptance",
  validateSearch: (s: Record<string, unknown>): { key?: string; peek?: string } => ({
    key: typeof s.key === "string" ? s.key : undefined,
    peek: typeof s.peek === "string" ? s.peek : undefined,
  }),
  component: function Acceptance() {
    const s = acceptanceRoute.useSearch();
    const navigate = useNavigate();
    return <AcceptanceInboxPage selected={s.key} peek={s.peek} setSearch={(n) => navigate({ to: "/inbox/acceptance", search: n, replace: true })} />;
  },
});

const projectSettingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/p/$key/settings",
  component: function ProjectSettings() {
    const { key } = projectSettingsRoute.useParams();
    return <ProjectSettingsPage key={key} projectKey={key} />;
  },
});

const viewRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/views/$id",
  validateSearch: parseIssuesSearch,
  component: function ViewRoute() {
    const { id } = viewRoute.useParams();
    const search = viewRoute.useSearch();
    const navigate = useNavigate();
    return <SavedViewPage key={id} id={id} search={search}
      setSearch={(s) => navigate({ to: "/views/$id", params: { id }, search: s, replace: true })} />;
  },
});

const settingsRoute = createRoute({ getParentRoute: () => appRoute, path: "/settings", component: SettingsPage });

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([indexRoute, myRoute, boardRoute, projectsRoute, projectIssuesRoute, storyMapRoute, issueRoute, qaInboxRoute, acceptanceRoute, agentsRoute, viewRoute, projectSettingsRoute, settingsRoute]),
]);

export const router = createRouter({ routeTree, defaultPreload: false });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
