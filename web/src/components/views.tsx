import { DndContext, PointerSensor, useDraggable, useDroppable, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core";
import { ChevronDown, ChevronRight } from "lucide-react";
import { useMemo, useState } from "react";
import type { Issue, ProjectTier, State, UserRef } from "../api/client";
import { useProjects, useStatesOf, useUpdateIssue } from "../api/hooks";
import type { GroupBy } from "../lib/filters";
import { CATEGORY_LABEL, ISSUE_TYPES, PRIORITY_META, TIER_HINT, TIER_LABEL, TIERS, tierRank, TYPE_META } from "../lib/meta";
import { Avatar, PriorityIcon, ProgressPie, StateIcon, TypeIcon } from "./icons";
import { IssueRow, LabelChip, QABadge } from "./IssueRow";
import { AssigneePicker, TierIcon, TierPicker } from "./pickers";

interface ViewProps {
  compact?: boolean;
  issues: Issue[];
  states: State[];
  selectedKey?: string;
  onOpen: (key: string) => void;
  showProject?: boolean;
}

interface Group {
  id: string;
  label: string;
  icon: React.ReactNode;
  issues: Issue[];
}

function groupIssues(issues: Issue[], by: GroupBy, states: State[]): Group[] {
  if (by === "none") return [{ id: "all", label: "すべて", icon: null, issues }];
  const map = new Map<string, Group>();
  const add = (id: string, label: string, icon: React.ReactNode, is: Issue) => {
    if (!map.has(id)) map.set(id, { id, label, icon, issues: [] });
    map.get(id)!.issues.push(is);
  };
  for (const is of issues) {
    if (by === "state") add(String(is.state.id), is.state.name, <StateIcon state={is.state} />, is);
    if (by === "type") add(is.type, TYPE_META[is.type].label, <TypeIcon type={is.type} />, is);
    if (by === "priority") add(String(is.priority), PRIORITY_META[is.priority].label, <PriorityIcon priority={is.priority} />, is);
    if (by === "assignee") add(is.assignee?.username ?? "", is.assignee?.display_name ?? "未割り当て", <Avatar user={is.assignee} size={16} />, is);
  }
  const groups = [...map.values()];
  const order = (g: Group) => {
    if (by === "state") {
      const s = states.find((s) => String(s.id) === g.id);
      // Workflow order, QA states after the others.
      return s ? (s.applies_to === "qa" ? 100 : 0) + s.position : 999;
    }
    if (by === "type") return ISSUE_TYPES.indexOf(g.id as never);
    if (by === "priority") return g.id === "0" ? 9 : Number(g.id);
    return g.id === "" ? "~" : g.label;
  };
  return groups.sort((a, b) => (order(a) < order(b) ? -1 : order(a) > order(b) ? 1 : 0));
}

export function ListView({ issues, states, selectedKey, onOpen, groupBy, showProject, compact }: ViewProps & { groupBy: GroupBy }) {
  const groups = useMemo(() => groupIssues(issues, groupBy, states), [issues, groupBy, states]);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  return (
    <div role="table">
      {groups.map((g) => (
        <div key={g.id}>
          {groupBy !== "none" && (
            <button onClick={() => setCollapsed((c) => ({ ...c, [g.id]: !c[g.id] }))}
              className="sticky top-0 z-10 flex h-9 w-full items-center gap-2 border-b border-line bg-elev px-3 text-left text-[13px] font-medium">
              {collapsed[g.id] ? <ChevronRight size={13} className="text-faint" /> : <ChevronDown size={13} className="text-faint" />}
              {g.icon}
              {g.label}
              <span className="text-faint">{g.issues.length}</span>
            </button>
          )}
          {!collapsed[g.id] && g.issues.map((is) => (
            <IssueRow key={is.key} issue={is} selected={is.key === selectedKey} onOpen={onOpen} showProject={showProject} compact={compact} />
          ))}
        </div>
      ))}
    </div>
  );
}

/** Hierarchy view. `context` are ancestors that did not match the filter. */
export function TreeView({ issues, context, selectedKey, onOpen, compact }: ViewProps & { context: Issue[] }) {
  const matched = useMemo(() => new Set(issues.map((i) => i.key)), [issues]);
  const all = useMemo(() => [...issues, ...context], [issues, context]);
  const children = useMemo(() => {
    const m = new Map<string | null, Issue[]>();
    const keys = new Set(all.map((i) => i.key));
    for (const is of all) {
      const parent = is.parent_key && keys.has(is.parent_key) ? is.parent_key : null;
      if (!m.has(parent)) m.set(parent, []);
      m.get(parent)!.push(is);
    }
    for (const list of m.values()) list.sort((a, b) => a.sort_order - b.sort_order || a.number - b.number);
    return m;
  }, [all]);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  const render = (parent: string | null, depth: number): React.ReactNode =>
    (children.get(parent) ?? []).map((is) => {
      const kids = children.get(is.key);
      const isCollapsed = collapsed[is.key];
      return (
        <div key={is.key}>
          <IssueRow issue={is} depth={depth} selected={is.key === selectedKey} onOpen={onOpen} dimmed={!matched.has(is.key)} compact={compact}
            expander={kids ? { expanded: !isCollapsed, toggle: () => setCollapsed((c) => ({ ...c, [is.key]: !c[is.key] })) } : null} />
          {kids && !isCollapsed && render(is.key, depth + 1)}
        </div>
      );
    });
  return <div role="tree">{render(null, 0)}</div>;
}

const CLOSED: State["category"][] = ["completed", "canceled"];

/** `hideClosed` ("完了を隠す") drops the completed and canceled columns too, not only their cards. */
export function BoardView({ issues, states, onOpen, selectedKey, hideClosed }: ViewProps & { hideClosed?: boolean }) {
  const update = useUpdateIssue();
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));
  const hasQA = issues.some((i) => i.type === "qa");
  const columns = states.filter((s) => (s.applies_to === "all" || hasQA) && !(hideClosed && CLOSED.includes(s.category)));
  const onDragEnd = (e: DragEndEvent) => {
    const issue = issues.find((i) => i.key === e.active.id);
    const stateId = Number(e.over?.id);
    if (!issue || !stateId || issue.state.id === stateId) return;
    const target = states.find((s) => s.id === stateId);
    // QA issues only move between QA states and vice versa.
    if (!target || (target.applies_to === "qa") !== (issue.type === "qa")) return;
    update.mutate({ key: issue.key, state_id: stateId });
  };
  return (
    <DndContext sensors={sensors} onDragEnd={onDragEnd}>
      <div className="flex h-full gap-3 overflow-x-auto p-3">
        {columns.map((s) => (
          <BoardColumn key={s.id} state={s} issues={issues.filter((i) => i.state.id === s.id)} onOpen={onOpen} selectedKey={selectedKey} />
        ))}
      </div>
    </DndContext>
  );
}

function BoardColumn({ state, issues, onOpen, selectedKey }: { state: State; issues: Issue[]; onOpen: (k: string) => void; selectedKey?: string }) {
  const { setNodeRef, isOver } = useDroppable({ id: state.id });
  return (
    <div ref={setNodeRef} data-testid={`state-col-${state.name}`} className={`flex w-[280px] shrink-0 flex-col rounded-lg ${isOver ? "bg-active" : "bg-sidebar"}`}>
      <div className="flex items-center gap-2 px-3 py-2.5 text-[13px] font-medium">
        <StateIcon state={state} />
        {state.name}
        <span className="text-faint">{issues.length}</span>
      </div>
      <div className="flex min-h-[80px] flex-1 flex-col gap-2 overflow-y-auto px-2 pb-2">
        {issues.map((is) => <BoardCard key={is.key} issue={is} onOpen={onOpen} selected={is.key === selectedKey} />)}
      </div>
    </div>
  );
}

const CATEGORIES: State["category"][] = ["backlog", "unstarted", "started", "completed", "canceled"];

/**
 * Board across projects. Each project has its own states, so the columns are
 * the shared state categories and the rows (lanes) are projects. Dropping a
 * card on a column moves it to the first state of that category in its own
 * project (QA issues only to QA states, others only to the rest).
 */
export function CategoryBoardView({ issues, onOpen, selectedKey, lanes, hideClosed }: Omit<ViewProps, "states"> & { lanes: boolean; hideClosed?: boolean }) {
  const columns = CATEGORIES.filter((c) => !(hideClosed && CLOSED.includes(c)));
  const update = useUpdateIssue();
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));
  const { data: projects = [] } = useProjects();
  const tierOf = (k: string): ProjectTier => projects.find((p) => p.key === k)?.tier ?? "normal";
  const keys = useMemo(() => {
    const present = new Set(issues.map((i) => i.project_key));
    const ordered = projects.map((p) => p.key).filter((k) => present.has(k));
    const all = [...ordered, ...[...present].filter((k) => !ordered.includes(k)).sort()];
    // stable sort: 注力 → 通常 → その他, keeping the key order inside a tier
    return all.map((k, i) => ({ k, i, r: tierRank(projects.find((p) => p.key === k)?.tier) })).sort((a, b) => a.r - b.r || a.i - b.i).map((x) => x.k);
  }, [issues, projects]);
  const states = useStatesOf(keys);
  // Lanes the user opened or closed by hand; その他 lanes start closed, the rest open.
  const [toggled, setToggled] = useState<Set<string>>(new Set());
  const isClosed = (k: string) => (tierOf(k) === "side") !== toggled.has(k);
  const onDragEnd = (e: DragEndEvent) => {
    const issue = issues.find((i) => i.key === e.active.id);
    if (!issue || !e.over) return;
    const [lane, category] = String(e.over.id).split("|") as [string, State["category"]];
    if ((lane !== "*" && lane !== issue.project_key) || issue.state.category === category) return;
    const target = (states.get(issue.project_key) ?? [])
      .filter((s) => s.category === category && (s.applies_to === "qa") === (issue.type === "qa"))
      .sort((a, b) => a.position - b.position)[0];
    if (target) update.mutate({ key: issue.key, state_id: target.id });
  };
  const byCategory = (list: Issue[]) => new Map(columns.map((c) => [c, list.filter((i) => i.state.category === c)]));
  const all = byCategory(issues);
  const name = (k: string) => projects.find((p) => p.key === k)?.name ?? k;
  const lane = (k: string) => {
    const mine = issues.filter((i) => i.project_key === k);
    const cells = byCategory(mine);
    const closed = isClosed(k);
    return (
      <div key={k} className="mb-3 flex gap-3" data-testid={`lane-${k}`} data-closed={closed || undefined}>
        <div className="flex w-[120px] shrink-0 items-start gap-0.5 pt-2">
          <button onClick={() => setToggled((s) => { const n = new Set(s); if (n.has(k)) n.delete(k); else n.add(k); return n; })}
            className="flex min-w-0 flex-1 items-start gap-1 text-left text-[12.5px] hover:text-fg">
            {closed ? <ChevronRight size={14} className="mt-0.5 shrink-0 text-faint" /> : <ChevronDown size={14} className="mt-0.5 shrink-0 text-faint" />}
            <span className="min-w-0">
              <span className="block truncate font-medium">{name(k)}</span>
              <span className="font-mono text-[11px] text-faint">{k} · {mine.length}</span>
            </span>
          </button>
          <TierPicker projectKey={k} tier={tierOf(k)} />
        </div>
        {columns.map((c) => closed
          ? <div key={c} className="w-[260px] shrink-0 rounded-lg bg-sidebar px-2 py-2"><CellCount category={c} issues={cells.get(c)!} /></div>
          : <CategoryCell key={c} id={`${k}|${c}`} category={c} issues={cells.get(c)!} onOpen={onOpen} selectedKey={selectedKey} counted />)}
      </div>
    );
  };
  return (
    <DndContext sensors={sensors} onDragEnd={onDragEnd}>
      <div className="min-w-max p-3" data-testid="category-board">
        <div className="sticky top-0 z-10 flex gap-3 bg-bg pb-2">
          {lanes && <div className="w-[120px] shrink-0" />}
          {columns.map((c) => (
            <div key={c} data-testid={`col-${c}`} className="flex w-[260px] shrink-0 items-center gap-2 px-3 text-[13px] font-medium">
              <StateIcon state={{ category: c, color: "", name: CATEGORY_LABEL[c] }} />
              {CATEGORY_LABEL[c]}<span className="text-faint">{all.get(c)!.length}</span>
            </div>
          ))}
        </div>
        {lanes ? TIERS.map((t) => {
          const inTier = keys.filter((k) => tierOf(k) === t);
          if (inTier.length === 0) return null;
          const count = issues.filter((i) => tierOf(i.project_key) === t).length;
          return (
            <section key={t} data-testid={`tier-section-${t}`} className="mb-2">
              <div className="sticky left-0 mb-2 mt-1 flex w-max items-center gap-1.5 px-1 text-[12px] text-muted" title={TIER_HINT[t]}>
                <TierIcon tier={t} /><span className="font-medium text-fg">{TIER_LABEL[t]}</span>
                <span className="text-faint">{inTier.length} プロジェクト · {count} 件</span>
              </div>
              {inTier.map((k) => lane(k))}
            </section>
          );
        }) : (
          <div className="flex gap-3">
            {columns.map((c) => <CategoryCell key={c} id={`*|${c}`} category={c} issues={all.get(c)!} onOpen={onOpen} selectedKey={selectedKey} tall />)}
          </div>
        )}
      </div>
    </DndContext>
  );
}

/** A project's count in one category, split by its own states when there are several (e.g. In Progress 2 · 受入待ち 1). */
function CellCount({ category, issues }: { category: State["category"]; issues: Issue[] }) {
  const byState = new Map<string, number>();
  for (const i of issues) byState.set(i.state.name, (byState.get(i.state.name) ?? 0) + 1);
  return (
    <div data-testid="cell-count" className={`flex flex-wrap items-center gap-x-1.5 px-1 text-[11.5px] ${issues.length ? "text-muted" : "text-faint"}`}>
      <StateIcon state={{ category, color: "", name: CATEGORY_LABEL[category] }} size={12} />
      <span className="font-medium">{issues.length}</span>
      {byState.size > 1 && <span className="text-faint">（{[...byState].map(([n, c]) => `${n} ${c}`).join("・")}）</span>}
    </div>
  );
}

function CategoryCell({ id, category, issues, onOpen, selectedKey, tall, counted }: {
  id: string; category: State["category"]; issues: Issue[]; onOpen: (k: string) => void; selectedKey?: string; tall?: boolean; counted?: boolean;
}) {
  const { setNodeRef, isOver } = useDroppable({ id });
  return (
    <div ref={setNodeRef} data-testid={`cell-${id}`}
      className={`flex w-[260px] shrink-0 flex-col gap-2 rounded-lg p-2 ${tall ? "min-h-[200px]" : "min-h-[56px]"} ${isOver ? "bg-active" : "bg-sidebar"}`}>
      {counted && <CellCount category={category} issues={issues} />}
      {issues.map((is) => <BoardCard key={is.key} issue={is} onOpen={onOpen} selected={is.key === selectedKey} showState />)}
    </div>
  );
}

function BoardCard({ issue, onOpen, selected, showState }: { issue: Issue; onOpen: (k: string) => void; selected: boolean; showState?: boolean }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: issue.key });
  const style = transform ? { transform: `translate(${transform.x}px, ${transform.y}px)` } : undefined;
  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners} onClick={() => onOpen(issue.key)}
      className={`rounded-md border bg-elev p-2.5 text-[13px] shadow-sm ${selected ? "border-accent" : "border-line"} ${isDragging ? "z-50 opacity-80 shadow-lg" : "hover:border-line-strong"}`}>
      <div className="mb-1 flex items-center gap-1.5 text-[11.5px] text-faint">
        <TypeIcon type={issue.type} size={12} />
        <span className="font-mono">{issue.key}</span>
        <span className="flex-1" />
        <span onClick={(e) => e.stopPropagation()} onPointerDown={(e) => e.stopPropagation()}><AssigneePicker issue={issue} /></span>
      </div>
      <div className="mb-2 line-clamp-2 leading-snug">{issue.title}</div>
      <div className="flex flex-wrap items-center gap-1.5">
        <PriorityIcon priority={issue.priority} />
        {showState && <span className="flex items-center gap-1 text-[11.5px] text-muted"><StateIcon state={issue.state} size={12} />{issue.state.name}</span>}
        <QABadge count={issue.open_qa_count} blocking={issue.blocking_qa_count} />
        <ProgressPie total={issue.progress.total} done={issue.progress.done} size={12} />
        {issue.labels.slice(0, 2).map((l) => <LabelChip key={l.id} name={l.name} color={l.color} />)}
      </div>
    </div>
  );
}

export type { UserRef };
