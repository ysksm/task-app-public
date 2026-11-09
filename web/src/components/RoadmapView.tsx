import { ChevronDown, ChevronRight, ClipboardCheck, MessageCircleQuestion } from "lucide-react";
import { useState } from "react";
import type { Issue } from "../api/client";
import { Avatar, StateIcon, TypeIcon } from "./icons";

// Roadmap (VIEW-07): epics and their stories with progress.

function ProgressBar({ issue }: { issue: Issue }) {
  const { total, done } = issue.progress;
  const finished = issue.state.category === "completed";
  const ratio = total ? done / total : finished ? 1 : 0;
  return (
    <div className="flex w-[180px] shrink-0 items-center gap-2">
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-hover">
        <div className="h-full rounded-full bg-[#3fae6a]" style={{ width: `${Math.round(ratio * 100)}%` }} />
      </div>
      <span className="w-[64px] text-right text-[11.5px] tabular-nums text-muted">{total ? `${done}/${total}` : finished ? "完了" : "—"} {total ? `${Math.round(ratio * 100)}%` : ""}</span>
    </div>
  );
}

function Row({ issue, depth, onOpen, selected, expander }: { issue: Issue; depth: number; onOpen: (k: string) => void; selected: boolean; expander?: React.ReactNode }) {
  return (
    <div onClick={() => onOpen(issue.key)}
      className={`flex h-10 cursor-pointer items-center gap-2 border-b border-line/60 px-4 text-[13px] hover:bg-hover ${selected ? "bg-active" : ""}`}>
      <span style={{ width: depth * 22 }} className="shrink-0" />
      <span className="flex w-4 shrink-0 justify-center">{expander}</span>
      <TypeIcon type={issue.type} size={14} />
      <span className="w-[64px] shrink-0 font-mono text-[11.5px] text-faint">{issue.key}</span>
      <span className={`min-w-0 flex-1 truncate ${depth === 0 ? "font-medium" : ""}`}>{issue.title}</span>
      {issue.open_qa_count > 0 && (
        <span className={`flex items-center gap-0.5 rounded px-1 text-[11px] ${issue.blocking_qa_count ? "bg-[#e5484d22] text-[#e5484d]" : "bg-[#e0823d22] text-[#c2700f]"}`}>
          <MessageCircleQuestion size={11} />{issue.open_qa_count}
        </span>
      )}
      {issue.state.is_acceptance && <ClipboardCheck size={13} className="text-accent" />}
      <span className="flex w-[110px] shrink-0 items-center gap-1.5 truncate text-[12px] text-muted"><StateIcon state={issue.state} size={12} />{issue.state.name}</span>
      <ProgressBar issue={issue} />
      <span className="w-[76px] shrink-0 text-right text-[12px] text-faint">{issue.due_date ?? ""}</span>
      <span className="flex w-5 shrink-0 justify-center"><Avatar user={issue.assignee} size={17} /></span>
    </div>
  );
}

export function RoadmapView({ issues, matched, onOpen, selectedKey }: { issues: Issue[]; matched: Set<string> | null; onOpen: (k: string) => void; selectedKey?: string }) {
  const [closed, setClosed] = useState<Set<string>>(new Set());
  const epics = issues.filter((i) => i.type === "epic").sort((a, b) => a.sort_order - b.sort_order || a.number - b.number);
  const storiesOf = (key: string | null) =>
    issues.filter((i) => i.type === "story" && (key ? i.parent_key === key : !i.parent_key || !issues.some((e) => e.key === i.parent_key && e.type === "epic")))
      .sort((a, b) => a.sort_order - b.sort_order || a.number - b.number);
  const visible = (i: Issue) => !matched || matched.has(i.key);
  const toggle = (k: string) => { const n = new Set(closed); n.has(k) ? n.delete(k) : n.add(k); setClosed(n); };

  const groups = epics
    .map((e) => ({ epic: e, stories: storiesOf(e.key).filter(visible) }))
    .filter((g) => visible(g.epic) || g.stories.length > 0);
  const loose = storiesOf(null).filter(visible);

  if (groups.length === 0 && loose.length === 0) return <div className="p-10 text-center text-faint">エピック・ストーリーがありません</div>;
  return (
    <div className="min-w-[860px]">
      <div className="sticky top-0 z-10 flex h-8 items-center gap-2 border-b border-line bg-bg px-4 text-[11.5px] text-faint">
        <span className="flex-1 pl-6">エピック / ストーリー</span>
        <span className="w-[110px]">ステータス</span>
        <span className="w-[180px]">進捗（子孫の完了数）</span>
        <span className="w-[76px] text-right">期日</span>
        <span className="w-5" />
      </div>
      {groups.map(({ epic, stories }) => (
        <div key={epic.key}>
          <Row issue={epic} depth={0} onOpen={onOpen} selected={epic.key === selectedKey}
            expander={stories.length > 0 && (
              <button onClick={(e) => { e.stopPropagation(); toggle(epic.key); }} className="text-faint hover:text-fg">
                {closed.has(epic.key) ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
              </button>
            )} />
          {!closed.has(epic.key) && stories.map((s) => <Row key={s.key} issue={s} depth={1} onOpen={onOpen} selected={s.key === selectedKey} />)}
        </div>
      ))}
      {loose.length > 0 && (
        <>
          <div className="border-b border-line/60 px-4 py-2 text-[12px] text-faint">エピックに属さないストーリー</div>
          {loose.map((s) => <Row key={s.key} issue={s} depth={1} onOpen={onOpen} selected={s.key === selectedKey} />)}
        </>
      )}
    </div>
  );
}
