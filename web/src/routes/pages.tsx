import { Link, useNavigate } from "@tanstack/react-router";
import { Bot, FolderKanban, Kanban, Map as MapIcon, Plus, Rows3, Settings } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import type { Filter, Project, ProjectTier, RequirementsMode, SavedView } from "../api/client";
import { ViewControls } from "./viewsPage";
import { useCreateProject, useIssues, useMe, useProject, useProjects, useStates } from "../api/hooks";
import { useCreateIssueModal } from "../components/CreateIssue";
import { FilterBar } from "../components/FilterBar";
import { IssueDetail } from "../components/IssueDetail";
import { OptionList, Popover } from "../components/Menu";
import { BoardView, CategoryBoardView, ListView, TreeView } from "../components/views";
import { MapView } from "../components/MapView";
import { RoadmapView } from "../components/RoadmapView";

const VIEW_LABEL: Record<ViewMode, string> = { list: "リスト", board: "ボード", tree: "ツリー", map: "マップ", roadmap: "ロードマップ" };
import { countFilters, toApiFilter, type FilterState, type GroupBy, type SortBy, type ViewMode } from "../lib/filters";
import { REQ_MODE_HINT, REQ_MODE_LABEL, REQ_MODES, TIER_HINT, TIER_LABEL, TIERS, TYPE_META } from "../lib/meta";
import { TierIcon, TierPicker } from "../components/pickers";

export interface IssuesSearch {
  view?: ViewMode;
  group?: GroupBy;
  sort?: SortBy;
  f?: FilterState;
  peek?: string;
  root?: string; // mind map root issue
  lanes?: "none"; // cross-project board without project lanes
}

/** Shared layout: header, filter bar, issue view and side-peek detail. */
export function IssuesScreen({ title, icon, projectKey, baseFilter, search, setSearch, views = ["list", "board", "tree"], defaultView = "list", headerExtra, showProject, savedView }: {
  title: ReactNode;
  icon?: ReactNode;
  projectKey?: string;
  baseFilter?: Filter;
  search: IssuesSearch;
  setSearch: (s: IssuesSearch) => void;
  views?: ViewMode[];
  defaultView?: ViewMode;
  headerExtra?: ReactNode;
  showProject?: boolean;
  /** the saved view being shown, if any */
  savedView?: SavedView;
}) {
  const view = search.view ?? defaultView;
  const group = search.group ?? (view === "list" ? "state" : "none");
  const sort = search.sort ?? (view === "tree" ? "manual" : "updated");
  const f = search.f ?? {};
  const filter = useMemo(() => {
    const ui = toApiFilter(f);
    const parts = [baseFilter, ui].filter(Boolean) as Filter[];
    return parts.length === 0 ? undefined : parts.length === 1 ? parts[0] : { and: parts };
  }, [f, baseFilter]);
  const { data, isLoading, error } = useIssues({ project: projectKey, filter, sort, includeAncestors: view === "tree" });
  // The map and the roadmap show the whole hierarchy and use the filter only to dim / pick rows.
  const whole = view === "map" || view === "roadmap";
  const { data: all } = useIssues({ project: projectKey, limit: 5000, enabled: whole && !!projectKey });
  const matched = useMemo(() => (filter && data ? new Set(data.issues.map((i) => i.key)) : null), [filter, data]);
  const { data: project } = useProject(projectKey ?? "");
  const epics = (all?.issues ?? []).filter((i) => i.type === "epic");
  const { data: states = [] } = useStates(projectKey);
  const openCreate = useCreateIssueModal();

  const open = (key: string) => setSearch({ ...search, peek: key });
  const close = () => setSearch({ ...search, peek: undefined });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName);
      if (e.key === "Escape" && search.peek && !typing) close();
      if (e.key === "/" && !typing) {
        e.preventDefault();
        document.getElementById("filter-search")?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const issues = data?.issues ?? [];
  return (
    <div className="flex h-full min-w-0">
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-11 shrink-0 items-center gap-2 overflow-x-auto border-b border-line px-4 [scrollbar-width:none]">
          {icon}
          <h1 className="truncate whitespace-nowrap text-[14px] font-medium">{title}</h1>
          <span className="min-w-[4.5em] whitespace-nowrap text-[12px] tabular-nums text-faint">{data && !whole ? `${data.total} 件` : ""}</span>
          <button onClick={() => openCreate({ projectKey })} className="ml-1 flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white">
            <Plus size={13} /> 新規 <kbd className="ml-1 rounded bg-white/20 px-1 text-[10px]">C</kbd>
          </button>
          {views.length > 1 && (
            <div className="flex shrink-0 rounded-md border border-line p-0.5">
              {views.map((v) => (
                <button key={v} onClick={() => setSearch({ ...search, view: v, group: undefined, sort: undefined })}
                  className={`whitespace-nowrap rounded px-2.5 py-0.5 text-[12px] ${view === v ? "bg-active text-fg" : "text-muted hover:text-fg"}`}>
                  {VIEW_LABEL[v]}
                </button>
              ))}
            </div>
          )}
          {view === "board" && !projectKey && (
            <button onClick={() => setSearch({ ...search, lanes: search.lanes ? undefined : "none" })} title="プロジェクトごとに行を分ける"
              className={`flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] ${search.lanes ? "text-muted hover:bg-hover hover:text-fg" : "bg-active text-fg"}`}>
              <Rows3 size={13} />プロジェクトで分ける
            </button>
          )}
          {headerExtra}
          {view === "map" && (
            <label className="ml-2 flex items-center gap-1.5 whitespace-nowrap text-[12px] text-muted">
              ルート
              <select value={search.root ?? ""} onChange={(e) => setSearch({ ...search, root: e.target.value || undefined })}
                className="max-w-[200px] rounded-md border border-line bg-bg px-1.5 py-0.5 text-[12px] outline-none">
                <option value="">プロジェクト全体</option>
                {epics.map((e) => <option key={e.key} value={e.key}>{e.key} {e.title}</option>)}
              </select>
            </label>
          )}
        </header>
        <FilterBar value={f} projectKey={projectKey} onChange={(nf) => setSearch({ ...search, f: countFilters(nf) ? nf : undefined })}
          extra={<>{view === "list" && (
            <div className="flex items-center gap-1 text-[12px] text-muted">
              <SelectMenu label="グループ" value={group} onChange={(g) => setSearch({ ...search, group: g })}
                options={[["state", "ステータス"], ["assignee", "担当者"], ["type", "種別"], ["priority", "優先度"], ["none", "なし"]]} />
              <SelectMenu label="並び" value={sort} onChange={(s) => setSearch({ ...search, sort: s })}
                options={[["updated", "更新日"], ["created", "作成日"], ["priority", "優先度"], ["due_date", "期日"], ["key", "キー"], ["manual", "手動"]]} />
            </div>
          )}
          <ViewControls projectKey={projectKey} search={search} filter={filter} baseFilter={baseFilter} savedView={savedView} /></>} />
        <div className={`min-h-0 flex-1 ${view === "map" ? "overflow-hidden" : "overflow-auto"}`}>
          {error && <div className="p-6 text-danger">{(error as Error).message}</div>}
          {isLoading && <div className="p-6 text-faint">読み込み中...</div>}
          {view === "map" && all && (
            <MapView issues={all.issues} matched={matched} projectName={project?.name ?? projectKey ?? ""} rootKey={search.root}
              selectedKey={search.peek} onOpen={open} />
          )}
          {view === "roadmap" && all && <RoadmapView issues={all.issues} matched={matched} onOpen={open} selectedKey={search.peek} />}
          {data && !whole && data.total > issues.length && (
            <div data-testid="truncated" className="border-b border-line bg-[#d4a0171a] px-4 py-1.5 text-[12px] text-muted">
              全 {data.total.toLocaleString("ja-JP")} 件のうち、先頭 {issues.length.toLocaleString("ja-JP")} 件だけを表示しています。「完了を隠す」やフィルタで絞ってください。
            </div>
          )}
          {data && !whole && issues.length === 0 && <div className="p-10 text-center text-faint">該当するイシューはありません</div>}
          {data && issues.length > 0 && view === "list" && <ListView issues={issues} states={states} groupBy={group} onOpen={open} selectedKey={search.peek} showProject={showProject} compact={!!search.peek} />}
          {data && issues.length > 0 && view === "tree" && <TreeView issues={issues} context={data.ancestors ?? []} states={states} onOpen={open} selectedKey={search.peek} compact={!!search.peek} />}
          {data && view === "board" && (projectKey
            ? <BoardView issues={issues} states={states} onOpen={open} selectedKey={search.peek} hideClosed={!!f.hideClosed} />
            : <CategoryBoardView issues={issues} onOpen={open} selectedKey={search.peek} lanes={!search.lanes} hideClosed={!!f.hideClosed} />)}
        </div>
      </div>
      {search.peek && (
        <div className="w-[min(720px,55%)] shrink-0 border-l border-line bg-bg">
          <IssueDetail key={search.peek} issueKey={search.peek} mode="peek" onClose={close} onOpen={open} />
        </div>
      )}
    </div>
  );
}

function SelectMenu<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: [T, string][]; onChange: (v: T) => void }) {
  return (
    <Popover trigger={() => (
      <button className="rounded-md px-2 py-1 hover:bg-hover">{label}: <span className="text-fg">{options.find((o) => o[0] === value)?.[1]}</span></button>
    )}>
      {(close) => <OptionList options={options.map(([v, l]) => ({ value: v, label: l }))} selected={[value]} onSelect={(v) => (onChange(v), close())} />}
    </Popover>
  );
}

// ---- pages ----

export function ProjectIssuesPage({ projectKey, search, setSearch }: { projectKey: string; search: IssuesSearch; setSearch: (s: IssuesSearch) => void }) {
  const { data: project } = useProject(projectKey);
  return (
    <IssuesScreen projectKey={projectKey} search={search} setSearch={setSearch} views={["list", "board", "tree", "map", "roadmap"]}
      icon={<FolderKanban size={15} className="text-muted" />}
      title={<>{project?.name ?? projectKey} <span className="ml-1 font-mono text-[12px] text-faint">{projectKey}</span></>}
      headerExtra={<>
        <Link to="/p/$key/storymap" params={{ key: projectKey }} data-testid="open-storymap" title="要求（ペルソナ・ジョブ・ストーリー）と、それを満たすイシューのつながり"
          className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] text-muted hover:bg-hover hover:text-fg"><MapIcon size={13} />ストーリーマップ</Link>
        <Link to="/p/$key/settings" params={{ key: projectKey }} title="プロジェクト設定" className="rounded p-1 text-faint hover:bg-hover hover:text-fg"><Settings size={14} /></Link>
      </>} />
  );
}

/** Every project on one board: columns are state categories, rows are projects. */
export function AllBoardPage({ search, setSearch }: { search: IssuesSearch; setSearch: (s: IssuesSearch) => void }) {
  return (
    <IssuesScreen title="全体ボード" icon={<Kanban size={15} className="text-muted" />} search={search} setSearch={setSearch}
      views={["board", "list"]} defaultView="board" showProject />
  );
}

export type MyTab = "assigned" | "created" | "qa";

export function MyIssuesPage({ tab, search, setSearch }: { tab: MyTab; search: IssuesSearch; setSearch: (s: IssuesSearch & { tab?: MyTab }) => void }) {
  const baseFilter: Filter = {
    assigned: { field: "assignee", op: "eq", value: "me" },
    created: { field: "reporter", op: "eq", value: "me" },
    qa: { and: [{ field: "type", op: "eq", value: "qa" }, { field: "assignee", op: "eq", value: "me" }, { field: "state_category", op: "in", value: ["backlog", "unstarted"] }] },
  }[tab];
  const tabs: [MyTab, string][] = [["assigned", "担当"], ["created", "起票したもの"], ["qa", "回答待ち QA"]];
  return (
    <IssuesScreen title="マイタスク" baseFilter={baseFilter} search={search} views={["list", "tree"]} showProject
      setSearch={(s) => setSearch({ ...s, tab })}
      headerExtra={(
        <div className="ml-3 flex gap-1">
          {tabs.map(([t, l]) => (
            <button key={t} onClick={() => setSearch({ tab: t })}
              className={`rounded-md px-2.5 py-1 text-[12px] ${tab === t ? "bg-active text-fg" : "text-muted hover:bg-hover hover:text-fg"}`}>{l}</button>
          ))}
        </div>
      )} />
  );
}

export function ProjectsPage() {
  const { data: projects = [] } = useProjects();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  return (
    <div className="h-full overflow-auto">
      <header className="flex h-11 items-center gap-2 border-b border-line px-4">
        <h1 className="text-[14px] font-medium">プロジェクト</h1>
        <button onClick={() => setCreating(true)} className="ml-1 flex items-center gap-1 rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white"><Plus size={13} /> 新規プロジェクト</button>
      </header>
      {creating && <NewProjectForm onDone={(key) => (setCreating(false), key && navigate({ to: "/p/$key/issues", params: { key } }))} />}
      {TIERS.map((t) => {
        const list = projects.filter((p) => (p.tier ?? "normal") === t);
        if (list.length === 0) return null;
        return (
          <section key={t} data-testid={`projects-tier-${t}`} className="px-4 pt-4 last:pb-4">
            <h2 className="mb-2 flex items-center gap-1.5 text-[12.5px] text-muted" title={TIER_HINT[t]}>
              <TierIcon tier={t} /><span className="font-medium text-fg">{TIER_LABEL[t]}</span>
              <span className="text-faint">{list.length} · {TIER_HINT[t]}</span>
            </h2>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3">
              {list.map((p) => <ProjectCard key={p.key} project={p} onOpen={() => navigate({ to: "/p/$key/issues", params: { key: p.key } })} />)}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function ProjectCard({ project: p, onOpen }: { project: Project; onOpen: () => void }) {
  const stats = p.stats ?? { total: 0, done: 0, by_type: {} };
  const pct = stats.total ? Math.round((stats.done / stats.total) * 100) : 0;
  return (
    // a div, not a button: the tier picker inside is a button too
    <div role="link" tabIndex={0} onClick={onOpen} onKeyDown={(e) => e.key === "Enter" && e.target === e.currentTarget && onOpen()}
      className={`cursor-pointer rounded-lg border border-line bg-elev p-4 text-left hover:border-line-strong ${p.tier === "side" ? "opacity-80" : ""}`}>
      <div className="mb-1 flex items-center gap-2">
        <TierPicker projectKey={p.key} tier={p.tier ?? "normal"} />
        <span className="font-mono text-[12px] text-faint">{p.key}</span>
        <span className="truncate text-[15px] font-medium">{p.name}</span>
      </div>
      <div className="mb-3 line-clamp-2 min-h-[2.6em] text-[12px] text-muted">{p.description}</div>
      <div className="mb-1 flex justify-between text-[12px] text-muted"><span>進捗</span><span>{stats.done}/{stats.total}（{pct}%）</span></div>
      <div className="h-1.5 overflow-hidden rounded-full bg-hover"><div className="h-full bg-[#3fae6a]" style={{ width: `${pct}%` }} /></div>
      <div className="mt-3 flex flex-wrap gap-3 text-[11.5px] text-muted">
        {Object.entries(stats.by_type).map(([t, n]) => <span key={t}>{TYPE_META[t as keyof typeof TYPE_META]?.label ?? t} {n}</span>)}
      </div>
    </div>
  );
}

function NewProjectForm({ onDone }: { onDone: (key?: string) => void }) {
  const create = useCreateProject();
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [tier, setTier] = useState<ProjectTier>("normal");
  // Option E: the requirements mode is chosen explicitly (no preselection); the API alone defaults to quick.
  const [mode, setMode] = useState<RequirementsMode | "">("");
  return (
    <form className="flex items-center gap-2 border-b border-line px-4 py-3" onSubmit={(e) => {
      e.preventDefault();
      if (!mode) return;
      create.mutate({ key, name, tier, requirements_mode: mode }, { onSuccess: (p) => onDone(p.key) });
    }}>
      <input autoFocus value={key} onChange={(e) => setKey(e.target.value.toUpperCase())} placeholder="KEY" className="w-24 rounded-md border border-line bg-bg px-2 py-1 font-mono outline-none focus:border-accent" />
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="プロジェクト名" className="w-64 rounded-md border border-line bg-bg px-2 py-1 outline-none focus:border-accent" />
      <select value={tier} onChange={(e) => setTier(e.target.value as ProjectTier)} title="ティア" className="rounded-md border border-line bg-bg px-1.5 py-1 text-[12px] outline-none">
        {TIERS.map((t) => <option key={t} value={t}>{TIER_LABEL[t]}</option>)}
      </select>
      <select value={mode} onChange={(e) => setMode(e.target.value as RequirementsMode)} title={mode ? REQ_MODE_HINT[mode] : "ストーリーマップの書き方。あとから設定で変えられる"}
        data-testid="new-project-mode" className={`rounded-md border bg-bg px-1.5 py-1 text-[12px] outline-none ${mode ? "border-line" : "border-accent text-muted"}`}>
        <option value="" disabled>要求の書き方を選ぶ</option>
        {REQ_MODES.map((m) => <option key={m} value={m}>{REQ_MODE_LABEL[m]}</option>)}
      </select>
      <button disabled={!mode} data-testid="create-project" className="rounded-md bg-accent px-3 py-1 text-[12px] text-white disabled:opacity-40">作成</button>
      <button type="button" onClick={() => onDone()} className="rounded-md px-3 py-1 text-[12px] text-muted hover:bg-hover">キャンセル</button>
      {create.error && <span className="text-[12px] text-danger">{(create.error as Error).message}</span>}
    </form>
  );
}

export function IssuePage({ issueKey }: { issueKey: string }) {
  const navigate = useNavigate();
  return (
    <IssueDetail issueKey={issueKey} mode="page"
      onOpen={(k) => navigate({ to: "/p/$key/issues/$issueKey", params: { key: k.split("-")[0], issueKey: k } })} />
  );
}

export function AgentBadge() {
  const { data: me } = useMe();
  if (me?.kind !== "agent") return null;
  return <Bot size={14} />;
}
