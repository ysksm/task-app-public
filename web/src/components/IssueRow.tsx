import { CalendarDays, ChevronDown, ChevronRight, MessageCircleQuestion } from "lucide-react";
import type { Issue } from "../api/client";
import { relativeTime } from "../lib/meta";
import { ProgressPie, TypeIcon } from "./icons";
import { AssigneePicker, PriorityPicker, StatePicker } from "./pickers";

export function LabelChip({ name, color }: { name: string; color: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-line px-1.5 py-[1px] text-[11px] text-muted">
      <span className="h-1.5 w-1.5 rounded-full" style={{ background: color }} />
      {name}
    </span>
  );
}

export function QABadge({ count, blocking = 0 }: { count: number; blocking?: number }) {
  if (!count) return null;
  return (
    <span title={`未回答の QA ${count} 件${blocking ? `（うちブロッキング ${blocking} 件）` : ""}`}
      className={`inline-flex items-center gap-0.5 rounded-full px-1.5 py-[1px] text-[11px] font-medium ${blocking ? "bg-[#e5484d22] text-[#e5484d]" : "bg-[#e0823d22] text-[#e0823d]"}`}>
      <MessageCircleQuestion size={11} />
      {count}
    </span>
  );
}

export function IssueRow({ issue, depth = 0, selected, onOpen, dimmed, expander, showProject, compact }: {
  issue: Issue;
  depth?: number;
  selected?: boolean;
  onOpen: (key: string) => void;
  /** ancestor shown only for context in tree view */
  dimmed?: boolean;
  expander?: { expanded: boolean; toggle: () => void } | null;
  showProject?: boolean;
  /** narrow container: hide labels, dates and progress text */
  compact?: boolean;
}) {
  const closed = issue.state.category === "completed" || issue.state.category === "canceled";
  return (
    <div role="row" tabIndex={-1} onClick={() => onOpen(issue.key)}
      className={`group flex h-9 cursor-default items-center gap-2 border-b border-line/60 pr-3 text-[13px] hover:bg-hover ${selected ? "bg-active" : ""} ${dimmed ? "opacity-50" : ""}`}
      style={{ paddingLeft: 12 + depth * 20 }}>
      <span className="w-4 shrink-0">
        {expander && (
          <button onClick={(e) => (e.stopPropagation(), expander.toggle())} className="rounded p-0.5 text-faint hover:bg-active hover:text-fg">
            {expander.expanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
          </button>
        )}
      </span>
      <span onClick={(e) => e.stopPropagation()}><PriorityPicker issue={issue} /></span>
      <span className="w-[68px] shrink-0 font-mono text-[11.5px] text-faint">{issue.key}</span>
      <span onClick={(e) => e.stopPropagation()}><StatePicker issue={issue} /></span>
      <TypeIcon type={issue.type} />
      <span className={`min-w-0 flex-1 truncate ${closed ? "text-muted" : ""}`}>{issue.title}</span>
      <QABadge count={issue.open_qa_count} blocking={issue.blocking_qa_count} />
      {!compact && (
        <span className="hidden items-center gap-1 md:flex">
          {issue.labels.slice(0, 3).map((l) => <LabelChip key={l.id} name={l.name} color={l.color} />)}
        </span>
      )}
      {showProject && <span className="text-[11px] text-faint">{issue.project_key}</span>}
      <ProgressPie total={issue.progress.total} done={issue.progress.done} />
      {!compact && issue.due_date && (
        <span className="hidden items-center gap-1 text-[11px] text-muted lg:flex"><CalendarDays size={12} />{issue.due_date.slice(5)}</span>
      )}
      {!compact && <span className="hidden w-14 text-right text-[11px] text-faint lg:block">{relativeTime(issue.updated_at)}</span>}
      <span onClick={(e) => e.stopPropagation()}><AssigneePicker issue={issue} /></span>
    </div>
  );
}
