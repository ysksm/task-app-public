import { useNavigate } from "@tanstack/react-router";
import { Bot, ChevronRight, ListTree, Maximize2, Plus, Trash2, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import Markdown from "react-markdown";
import type { Activity, Checkpoint, Comment, Issue } from "../api/client";
import { useActivity, useAddComment, useCheckpoints, useComments, useDeleteIssue, useIssue, useLabels, useUpdateIssue } from "../api/hooks";
import { CheckpointCard } from "./CheckpointCard";
import { QAPanel } from "./QAPanel";
import { AcceptanceSection } from "./AcceptancePanel";
import { IssueStories } from "./IssueStories";
import { allowedChildTypes, relativeTime, TYPE_META } from "../lib/meta";
import { useCreateIssueModal } from "./CreateIssue";
import { Avatar, PriorityIcon, ProgressPie, StateIcon, TypeIcon } from "./icons";
import { IssueRow, LabelChip } from "./IssueRow";
import { OptionList, Popover } from "./Menu";
import { AssigneePicker, PriorityPicker, StatePicker } from "./pickers";

export function IssueDetail({ issueKey, onClose, onOpen, mode }: {
  issueKey: string;
  onClose?: () => void;
  onOpen: (key: string) => void;
  mode: "peek" | "page";
}) {
  const { data: issue, error } = useIssue(issueKey);
  const navigate = useNavigate();
  const del = useDeleteIssue();
  const openCreate = useCreateIssueModal();

  if (error) return <div className="p-6 text-muted">{(error as Error).message}</div>;
  if (!issue) return <div className="p-6 text-faint">読み込み中...</div>;

  const childTypes = allowedChildTypes(issue.type);
  return (
    <div className="flex h-full flex-col" data-testid="issue-detail" data-issue-key={issue.key}>
      <div className="flex h-11 shrink-0 items-center gap-1 border-b border-line px-4 text-[12.5px] text-muted">
        <button onClick={() => navigate({ to: "/p/$key/issues", params: { key: issue.project_key } })} className="hover:text-fg">{issue.project_key}</button>
        {(issue.ancestors ?? []).map((a) => (
          <span key={a.key} className="flex min-w-0 shrink items-center gap-1">
            <ChevronRight size={12} className="text-faint" />
            <button onClick={() => onOpen(a.key)} className="flex min-w-0 items-center gap-1 hover:text-fg" title={a.title}>
              <TypeIcon type={a.type} size={12} /><span className="max-w-[140px] truncate">{a.title}</span>
            </button>
          </span>
        ))}
        <ChevronRight size={12} className="text-faint" />
        <span className="shrink-0 whitespace-nowrap font-mono text-fg">{issue.key}</span>
        <span className="flex-1" />
        <IconButton title="配下をツリー表示" onClick={() => navigate({ to: "/p/$key/issues", params: { key: issue.project_key }, search: { view: "tree", f: { under: issue.key } } })}><ListTree size={15} /></IconButton>
        {mode === "peek" && (
          <IconButton title="全画面で開く" onClick={() => navigate({ to: "/p/$key/issues/$issueKey", params: { key: issue.project_key, issueKey: issue.key } })}><Maximize2 size={14} /></IconButton>
        )}
        <IconButton title="削除" onClick={() => {
          if (window.confirm(`${issue.key} と配下のイシューを削除しますか？`)) del.mutate(issue.key, { onSuccess: () => onClose?.() });
        }}><Trash2 size={14} /></IconButton>
        {onClose && <IconButton title="閉じる (Esc)" onClick={onClose}><X size={16} /></IconButton>}
      </div>

      <div className="flex min-h-0 flex-1">
        <div className="min-w-0 flex-1 overflow-y-auto px-6 py-5">
          <div className="mb-2 flex items-center gap-2 text-[12px] text-muted">
            <TypeIcon type={issue.type} /> {TYPE_META[issue.type].label}
          </div>
          <EditableTitle issue={issue} />
          {issue.type === "qa" ? (
            <section className="mb-2 mt-2"><QAPanel qaKey={issue.key} onOpenIssue={onOpen} /></section>
          ) : (
            <EditableDescription issue={issue} />
          )}
          <IssueStories projectKey={issue.project_key} stories={issue.stories ?? []} />
          {(issue.type === "story" || issue.type === "feature") && (
            <AcceptanceSection issueKey={issue.key} onOpen={onOpen} onOpenInbox={() => navigate({ to: "/inbox/acceptance", search: { key: issue.key } })}
              storyCriteria={(issue.stories ?? []).flatMap((st) => st.criteria)} />
          )}

          {(issue.children?.length || childTypes.length > 0) && (
            <section className="mt-8">
              <div className="mb-1 flex items-center gap-2 text-[13px] font-medium">
                子イシュー
                <span className="text-faint">{issue.children?.length ?? 0}</span>
                <ProgressPie total={issue.progress.total} done={issue.progress.done} />
                <span className="flex-1" />
                {childTypes.length > 0 && (
                  <button onClick={() => openCreate({ projectKey: issue.project_key, parentKey: issue.key, type: childTypes.includes("task") ? "task" : childTypes[0] })}
                    className="flex items-center gap-1 rounded-md px-2 py-1 text-[12px] text-muted hover:bg-hover hover:text-fg">
                    <Plus size={13} /> 追加
                  </button>
                )}
              </div>
              <div className="rounded-md border border-line">
                {(issue.children ?? []).map((c) => <IssueRow key={c.key} issue={c} onOpen={onOpen} compact />)}
                {!issue.children?.length && <div className="px-3 py-2 text-[12px] text-faint">子イシューはありません</div>}
              </div>
            </section>
          )}

          <Timeline issueKey={issue.key} />
        </div>

        <aside className="w-[260px] shrink-0 overflow-y-auto border-l border-line bg-sidebar/50 px-4 py-4 text-[13px]">
          <Prop label="ステータス">
            <StatePicker issue={issue}><button className="prop-btn flex items-center gap-2 rounded-md px-1.5 py-1 hover:bg-hover"><StateIcon state={issue.state} />{issue.state.name}</button></StatePicker>
          </Prop>
          <Prop label="優先度">
            <PriorityPicker issue={issue}><button className="flex items-center gap-2 rounded-md px-1.5 py-1 hover:bg-hover"><PriorityIcon priority={issue.priority} />{["優先度なし", "緊急", "高", "中", "低"][issue.priority]}</button></PriorityPicker>
          </Prop>
          <Prop label="担当">
            <AssigneePicker issue={issue}><button className="flex items-center gap-2 rounded-md px-1.5 py-1 hover:bg-hover"><Avatar user={issue.assignee} />{issue.assignee?.display_name ?? "未割り当て"}</button></AssigneePicker>
          </Prop>
          <Prop label="起票者">
            <span className="flex items-center gap-2 px-1.5 py-1"><Avatar user={issue.reporter} />{issue.reporter.display_name}</span>
          </Prop>
          <Prop label="ラベル"><LabelEditor issue={issue} /></Prop>
          <Prop label="親"><ParentEditor issue={issue} onOpen={onOpen} /></Prop>
          <Prop label="期日"><DateEditor issue={issue} /></Prop>
          <Prop label="見積り"><EstimateEditor issue={issue} /></Prop>
          <div className="mt-4 space-y-1 border-t border-line pt-3 text-[12px] text-faint">
            <div>作成 {new Date(issue.created_at).toLocaleString("ja-JP")}</div>
            <div>更新 {relativeTime(issue.updated_at)}</div>
            {issue.started_at && <div>着手 {new Date(issue.started_at).toLocaleString("ja-JP")}</div>}
            {issue.completed_at && <div>完了 {new Date(issue.completed_at).toLocaleString("ja-JP")}</div>}
          </div>
        </aside>
      </div>
    </div>
  );
}

function IconButton({ children, title, onClick }: { children: ReactNode; title: string; onClick: () => void }) {
  return <button title={title} onClick={onClick} className="rounded-md p-1.5 text-muted hover:bg-hover hover:text-fg">{children}</button>;
}

function Prop({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="mb-1.5 flex items-center gap-2">
      <span className="w-16 shrink-0 text-[12px] text-faint">{label}</span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

function EditableTitle({ issue }: { issue: Issue }) {
  const update = useUpdateIssue();
  const [value, setValue] = useState(issue.title);
  useEffect(() => setValue(issue.title), [issue.title]);
  const save = () => value.trim() && value !== issue.title && update.mutate({ key: issue.key, title: value });
  return (
    <input value={value} onChange={(e) => setValue(e.target.value)} onBlur={save}
      onKeyDown={(e) => { if (e.key === "Enter") (e.target as HTMLInputElement).blur(); if (e.key === "Escape") setValue(issue.title); }}
      className="mb-3 w-full bg-transparent text-[20px] font-semibold outline-none" />
  );
}

function EditableDescription({ issue }: { issue: Issue }) {
  const update = useUpdateIssue();
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(issue.description);
  useEffect(() => setValue(issue.description), [issue.description]);
  if (editing) {
    return (
      <div>
        <textarea autoFocus value={value} onChange={(e) => setValue(e.target.value)} rows={Math.max(6, value.split("\n").length + 1)}
          onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) { update.mutate({ key: issue.key, description: value }); setEditing(false); } }}
          className="w-full rounded-md border border-line bg-bg p-3 font-mono text-[13px] leading-relaxed outline-none focus:border-accent" placeholder="Markdown で説明を書く" />
        <div className="mt-2 flex justify-end gap-2">
          <button onClick={() => (setValue(issue.description), setEditing(false))} className="rounded-md px-3 py-1 text-[12px] text-muted hover:bg-hover">キャンセル</button>
          <button onClick={() => (update.mutate({ key: issue.key, description: value }), setEditing(false))} className="rounded-md bg-accent px-3 py-1 text-[12px] text-white">保存 (⌘↵)</button>
        </div>
      </div>
    );
  }
  return (
    <div onClick={() => setEditing(true)} className="markdown min-h-[48px] cursor-text rounded-md text-[14px] leading-relaxed text-fg/90 hover:bg-hover/50">
      {issue.description ? <Markdown>{issue.description}</Markdown> : <span className="text-faint">説明を追加...</span>}
    </div>
  );
}

function LabelEditor({ issue }: { issue: Issue }) {
  const { data: labels = [] } = useLabels(issue.project_key);
  const update = useUpdateIssue();
  const [newName, setNewName] = useState("");
  const current = issue.labels.map((l) => l.name);
  const setLabels = (names: string[]) => update.mutate({ key: issue.key, labels: names });
  return (
    <Popover trigger={() => (
      <button className="flex min-h-[26px] w-full flex-wrap items-center gap-1 rounded-md px-1.5 py-1 text-left hover:bg-hover">
        {issue.labels.length ? issue.labels.map((l) => <LabelChip key={l.id} name={l.name} color={l.color} />) : <span className="text-faint">なし</span>}
      </button>
    )}>
      {() => (
        <div>
          <OptionList multi options={labels.map((l) => ({ value: l.name, label: l.name, icon: <span className="h-2 w-2 rounded-full" style={{ background: l.color }} /> }))}
            selected={current} onSelect={(n) => setLabels(current.includes(n) ? current.filter((x) => x !== n) : [...current, n])} />
          <form className="mt-1 border-t border-line pt-1" onSubmit={(e) => { e.preventDefault(); if (newName.trim()) { setLabels([...current, newName.trim()]); setNewName(""); } }}>
            <input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="新しいラベル + Enter"
              className="w-full rounded-md border border-line bg-bg px-2 py-1 text-[12px] outline-none focus:border-accent" />
          </form>
        </div>
      )}
    </Popover>
  );
}

function ParentEditor({ issue, onOpen }: { issue: Issue; onOpen: (k: string) => void }) {
  const update = useUpdateIssue();
  const [value, setValue] = useState("");
  const [err, setErr] = useState("");
  return (
    <div className="flex items-center gap-1">
      {issue.parent_key ? (
        <button onClick={() => onOpen(issue.parent_key!)} className="truncate rounded-md px-1.5 py-1 font-mono text-[12px] hover:bg-hover">{issue.parent_key}</button>
      ) : <span className="px-1.5 text-faint">なし</span>}
      <Popover align="right" trigger={() => <button className="rounded-md px-1.5 py-1 text-[12px] text-faint hover:bg-hover hover:text-fg">変更</button>}>
        {(close) => (
          <form className="w-[220px] p-1" onSubmit={(e) => {
            e.preventDefault();
            setErr("");
            update.mutate({ key: issue.key, parent_key: value.trim() ? value.trim().toUpperCase() : null }, {
              onSuccess: () => (setValue(""), close()),
              onError: (e) => setErr((e as Error).message),
            });
          }}>
            <input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder={`${issue.project_key}-123（空でルートへ）`}
              className="w-full rounded-md border border-line bg-bg px-2 py-1 font-mono text-[12px] outline-none focus:border-accent" />
            {err && <div className="mt-1 text-[11px] text-danger">{err}</div>}
          </form>
        )}
      </Popover>
    </div>
  );
}

function DateEditor({ issue }: { issue: Issue }) {
  const update = useUpdateIssue();
  return (
    <input type="date" value={issue.due_date ?? ""} onChange={(e) => update.mutate({ key: issue.key, due_date: e.target.value || null })}
      className="rounded-md bg-transparent px-1.5 py-1 text-[13px] outline-none hover:bg-hover" />
  );
}

function EstimateEditor({ issue }: { issue: Issue }) {
  const update = useUpdateIssue();
  const [v, setV] = useState(issue.estimate?.toString() ?? "");
  useEffect(() => setV(issue.estimate?.toString() ?? ""), [issue.estimate]);
  return (
    <input value={v} inputMode="decimal" placeholder="—" onChange={(e) => setV(e.target.value)}
      onBlur={() => {
        const n = v.trim() === "" ? null : Number(v);
        if (n !== issue.estimate && (n === null || !Number.isNaN(n))) update.mutate({ key: issue.key, estimate: n });
      }}
      className="w-20 rounded-md bg-transparent px-1.5 py-1 text-[13px] outline-none hover:bg-hover" />
  );
}

const FIELD_LABEL: Record<string, string> = {
  title: "タイトル", description: "説明", priority: "優先度", assignee: "担当", state: "ステータス", type: "種別",
  parent: "親", labels: "ラベル", due_date: "期日", estimate: "見積り",
};
const SOURCE_LABEL: Record<string, string> = { ui: "", api: "API", jira: "Jira API", mcp: "MCP", system: "system" };

function Timeline({ issueKey }: { issueKey: string }) {
  const { data: comments = [] } = useComments(issueKey);
  const { data: activity = [] } = useActivity(issueKey);
  const { data: checkpoints = [] } = useCheckpoints(issueKey);
  const add = useAddComment(issueKey);
  const [body, setBody] = useState("");
  type Entry =
    | { at: string; kind: "comment"; c: Comment }
    | { at: string; kind: "activity"; a: Activity }
    | { at: string; kind: "checkpoint"; cp: Checkpoint };
  const entries: Entry[] = [
    ...comments.map((c) => ({ at: c.created_at, kind: "comment" as const, c })),
    ...checkpoints.map((cp) => ({ at: cp.created_at, kind: "checkpoint" as const, cp })),
    ...activity.filter((a) => a.action !== "commented" && a.action !== "checkpoint").map((a) => ({ at: a.created_at, kind: "activity" as const, a })),
  ].sort((x, y) => x.at.localeCompare(y.at));

  return (
    <section className="mt-8">
      <div className="mb-3 text-[13px] font-medium">アクティビティ</div>
      <div className="space-y-2.5">
        {entries.map((e) => e.kind === "checkpoint" ? (
          <CheckpointCard key={e.cp.id} cp={e.cp} />
        ) : e.kind === "comment" ? (
          <div key={e.c.id} className="rounded-md border border-line bg-elev p-3">
            <div className="mb-1.5 flex items-center gap-2 text-[12px]">
              <Avatar user={e.c.author} size={16} />
              <span className="font-medium">{e.c.author.display_name}</span>
              {e.c.author.kind === "agent" && <Bot size={12} className="text-faint" />}
              <span className="text-faint">{relativeTime(e.c.created_at)}</span>
              {SOURCE_LABEL[e.c.source] && <span className="rounded bg-hover px-1 text-[10.5px] text-faint">{SOURCE_LABEL[e.c.source]}</span>}
            </div>
            <div className="markdown text-[13px]"><Markdown>{e.c.body}</Markdown></div>
          </div>
        ) : (
          <div key={e.a.id} className="flex items-center gap-2 pl-1 text-[12px] text-muted">
            <Avatar user={e.a.actor} size={14} />
            <span className="text-fg/80">{e.a.actor.display_name}</span>
            <span>{describeActivity(e.a)}</span>
            <span className="text-faint">· {relativeTime(e.a.created_at)}</span>
            {SOURCE_LABEL[e.a.source] && <span className="rounded bg-hover px-1 text-[10.5px] text-faint">{SOURCE_LABEL[e.a.source]}</span>}
          </div>
        ))}
      </div>
      <form className="mt-4" onSubmit={(e) => { e.preventDefault(); if (body.trim()) add.mutate(body, { onSuccess: () => setBody("") }); }}>
        <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={3} placeholder="コメントを書く... (⌘↵ で送信)"
          onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && body.trim()) add.mutate(body, { onSuccess: () => setBody("") }); }}
          className="w-full rounded-md border border-line bg-bg p-3 text-[13px] outline-none focus:border-accent" />
        <div className="flex justify-end">
          <button disabled={!body.trim()} className="rounded-md bg-accent px-3 py-1 text-[12px] text-white disabled:opacity-40">コメント</button>
        </div>
      </form>
    </section>
  );
}

function describeActivity(a: Activity): string {
  if (a.action === "created") return "が作成";
  if (a.action === "deleted") return "が削除";
  if (a.action === "answered") return `が QA に回答（${a.new_value} 件）`;
  if (a.action === "accepted") return `が受入検査で合格にした${a.new_value ? `「${a.new_value}」` : ""}`;
  if (a.action === "rejected") return `が受入検査で差し戻した「${a.new_value}」`;
  if (a.field === "acceptance_criteria") return `が受入基準を更新（${a.old_value} → ${a.new_value} 件）`;
  const f = FIELD_LABEL[a.field] ?? a.field;
  if (a.field === "description") return "が説明を更新";
  if (a.field === "priority") return `が${f}を ${["なし", "緊急", "高", "中", "低"][+a.old_value] ?? a.old_value} → ${["なし", "緊急", "高", "中", "低"][+a.new_value] ?? a.new_value} に変更`;
  if (!a.old_value && a.new_value) return `が${f}を ${a.new_value} に設定`;
  if (a.old_value && !a.new_value) return `が${f}（${a.old_value}）を解除`;
  return `が${f}を ${a.old_value} → ${a.new_value} に変更`;
}
