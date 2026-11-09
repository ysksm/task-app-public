import { Bot, Check, Lock, MessageCircleQuestion, Star } from "lucide-react";
import { useEffect, useState } from "react";
import type { QA, QAItem } from "../api/client";
import { useAnswerQA, useCloseQA, useQA } from "../api/hooks";
import { relativeTime } from "../lib/meta";
import { Avatar } from "./icons";

/** Draft answer for one question. For choice questions `text` is the free "other" answer. */
interface Draft {
  selected: string[];
  text: string;
  note: string;
}

const TYPE_LABEL: Record<QAItem["answer_type"], string> = {
  yes_no: "はい / いいえ",
  ok_ng: "OK / NG",
  single_choice: "1 つ選択",
  multi_choice: "複数選択可",
  text: "テキスト",
};

const STATUS_LABEL: Record<QA["status"], { label: string; cls: string }> = {
  open: { label: "未回答", cls: "bg-[#e0823d22] text-[#e0823d]" },
  answered: { label: "回答済", cls: "bg-[#2f7d8c22] text-[#3aa0b3]" },
  closed: { label: "クローズ", cls: "bg-[#3fae6a22] text-[#3fae6a]" },
};

function draftFrom(item: QAItem): Draft {
  const a = item.answer;
  return { selected: a?.selected_option_ids ?? [], text: a?.text ?? "", note: a?.note ?? "" };
}

function isComplete(item: QAItem, d: Draft): boolean {
  switch (item.answer_type) {
    case "yes_no":
    case "ok_ng":
      return d.selected.length === 1;
    case "single_choice":
      return d.selected.length + (d.text.trim() ? 1 : 0) === 1;
    case "multi_choice":
      return d.selected.length > 0 || !!d.text.trim();
    case "text":
      return !!d.text.trim();
  }
}

/** Shows a QA ticket and lets a human answer every question in one go. */
export function QAPanel({ qaKey, onOpenIssue }: { qaKey: string; onOpenIssue?: (key: string) => void }) {
  const { data: qa, error } = useQA(qaKey);
  if (error) return <div className="text-[13px] text-danger">{(error as Error).message}</div>;
  if (!qa) return <div className="text-[13px] text-faint">読み込み中...</div>;
  return <QAForm key={qa.key + qa.answered_at + qa.status} qa={qa} onOpenIssue={onOpenIssue} />;
}

function QAForm({ qa, onOpenIssue }: { qa: QA; onOpenIssue?: (key: string) => void }) {
  const answer = useAnswerQA(qa.key);
  const close = useCloseQA(qa.key);
  const [drafts, setDrafts] = useState<Record<string, Draft>>(() => Object.fromEntries(qa.items.map((i) => [i.id, draftFrom(i)])));
  const [editing, setEditing] = useState(qa.status === "open");
  useEffect(() => setEditing(qa.status === "open"), [qa.status]);

  const closed = qa.status === "closed";
  const readOnly = closed || !editing;
  const set = (id: string, patch: Partial<Draft>) => setDrafts((d) => ({ ...d, [id]: { ...d[id], ...patch } }));
  const missing = qa.items.filter((i) => i.required && !isComplete(i, drafts[i.id]));
  const changed = qa.items.filter((i) => isComplete(i, drafts[i.id]) && JSON.stringify(drafts[i.id]) !== JSON.stringify(draftFrom(i)));

  const submit = () => {
    const answers = qa.items
      .filter((i) => isComplete(i, drafts[i.id]))
      .map((i) => {
        const d = drafts[i.id];
        const isText = i.answer_type === "text" || ((i.answer_type === "single_choice" || i.answer_type === "multi_choice") && d.text.trim());
        return {
          item_id: i.id,
          selected_option_ids: i.answer_type === "text" ? [] : d.selected,
          text: isText ? d.text.trim() : null,
          note: d.note.trim() || null,
        };
      });
    answer.mutate({ answers }, { onSuccess: () => setEditing(false) });
  };

  return (
    <div className="@container space-y-4">
      <div className="flex flex-wrap items-center gap-2 text-[12px]">
        <span className={`rounded-full px-2 py-0.5 font-medium ${STATUS_LABEL[qa.status].cls}`}>{STATUS_LABEL[qa.status].label}</span>
        {qa.is_blocking && <span className="rounded-full bg-[#e5484d22] px-2 py-0.5 font-medium text-[#e5484d]">ブロッキング</span>}
        <span className="flex items-center gap-1 text-muted">
          <Avatar user={qa.asked_by} size={14} /> {qa.asked_by.display_name}
          {qa.asked_by.kind === "agent" && <Bot size={12} />} が {relativeTime(qa.created_at)} に質問
        </span>
        {qa.target_key && (
          <button onClick={() => onOpenIssue?.(qa.target_key!)} className="rounded bg-hover px-1.5 font-mono text-[11.5px] text-muted hover:text-fg">
            対象: {qa.target_key}
          </button>
        )}
        {qa.assignee && <span className="text-faint">回答者: {qa.assignee.display_name}</span>}
      </div>

      {qa.context && (
        <div className="rounded-md bg-hover/60 px-3 py-2 text-[13px] leading-relaxed">
          <div className="mb-0.5 text-[11px] font-medium text-faint">背景</div>
          {qa.context}
        </div>
      )}

      <ol className="space-y-3">
        {qa.items.map((item, idx) => (
          <li key={item.id} className="rounded-lg border border-line bg-elev p-4">
            <div className="mb-3 flex items-start gap-2">
              <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e0823d] text-[11px] font-bold text-white">{idx + 1}</span>
              <div className="flex-1 text-[14px] font-medium leading-snug">{item.question}</div>
              <span className="shrink-0 rounded bg-hover px-1.5 py-0.5 text-[11px] text-faint">
                {TYPE_LABEL[item.answer_type]}{!item.required && "・任意"}
              </span>
            </div>
            <AnswerWidget item={item} draft={drafts[item.id]} readOnly={readOnly} onChange={(p) => set(item.id, p)} />
            {(!readOnly || drafts[item.id].note) && item.answer_type !== "text" && (
              <input value={drafts[item.id].note} disabled={readOnly} onChange={(e) => set(item.id, { note: e.target.value })}
                placeholder="補足（任意）"
                className="mt-2 w-full rounded-md border border-line bg-bg px-2.5 py-1.5 text-[12.5px] outline-none focus:border-accent disabled:border-transparent disabled:bg-transparent disabled:px-0" />
            )}
            {item.answer && (
              <div className="mt-2 flex items-center gap-1.5 text-[11.5px] text-faint">
                <Check size={12} className="text-[#3fae6a]" />
                {item.answer.answered_by?.display_name} が {relativeTime(item.answer.answered_at)} に回答
              </div>
            )}
          </li>
        ))}
      </ol>

      {qa.provisional_decision && (
        <div className="flex gap-2 rounded-md border border-dashed border-line-strong px-3 py-2 text-[12.5px] text-muted">
          <Bot size={14} className="mt-0.5 shrink-0" />
          <span><span className="font-medium text-fg/80">エージェントの暫定判断:</span> {qa.provisional_decision}</span>
        </div>
      )}

      <div className="flex items-center gap-2">
        {answer.error && <span className="text-[12px] text-danger">{(answer.error as Error).message}</span>}
        <span className="flex-1" />
        {closed ? (
          <span className="flex items-center gap-1 text-[12px] text-faint"><Lock size={12} /> クローズ済み（回答は変更できません）</span>
        ) : editing ? (
          <>
            {missing.length > 0 && <span className="text-[12px] text-faint">必須の質問があと {missing.length} 件</span>}
            {qa.status !== "open" && (
              <button onClick={() => setEditing(false)} className="rounded-md px-3 py-1.5 text-[12px] text-muted hover:bg-hover">キャンセル</button>
            )}
            <button onClick={submit} disabled={changed.length === 0 || answer.isPending}
              className="rounded-md bg-accent px-4 py-1.5 text-[12.5px] font-medium text-white disabled:opacity-40">
              {missing.length > 0 && changed.length > 0 ? "途中まで保存" : "回答する"}
            </button>
          </>
        ) : (
          <>
            <button onClick={() => setEditing(true)} className="rounded-md px-3 py-1.5 text-[12px] text-muted hover:bg-hover">回答を修正</button>
            <button onClick={() => close.mutate()} className="rounded-md border border-line px-3 py-1.5 text-[12px] hover:bg-hover">クローズ</button>
          </>
        )}
      </div>
    </div>
  );
}

function AnswerWidget({ item, draft, readOnly, onChange }: { item: QAItem; draft: Draft; readOnly: boolean; onChange: (p: Partial<Draft>) => void }) {
  if (item.answer_type === "text") {
    return (
      <textarea value={draft.text} disabled={readOnly} rows={3} onChange={(e) => onChange({ text: e.target.value })} placeholder="回答を入力"
        className="w-full rounded-md border border-line bg-bg p-2.5 text-[13px] outline-none focus:border-accent disabled:border-transparent disabled:bg-transparent disabled:p-0" />
    );
  }
  if (item.answer_type === "yes_no" || item.answer_type === "ok_ng") {
    return (
      <div className="flex gap-2">
        {item.options.map((o) => {
          const on = draft.selected.includes(o.id);
          const positive = o.id === "yes" || o.id === "ok";
          return (
            <button key={o.id} disabled={readOnly} onClick={() => onChange({ selected: [o.id] })}
              className={`flex min-w-[120px] items-center justify-center gap-1.5 rounded-lg border-2 px-5 py-2.5 text-[14px] font-semibold transition-colors
                ${on ? (positive ? "border-[#3fae6a] bg-[#3fae6a1f] text-[#3fae6a]" : "border-[#e5484d] bg-[#e5484d1a] text-[#e5484d]") : "border-line text-muted"}
                ${readOnly ? "cursor-default" : "hover:border-line-strong hover:text-fg"} ${readOnly && !on ? "opacity-40" : ""}`}>
              {on && <Check size={15} />}
              {o.label}
              {o.recommended && <Star size={11} className="fill-current text-[#d9a21b]" aria-label="推奨" />}
            </button>
          );
        })}
      </div>
    );
  }
  const multi = item.answer_type === "multi_choice";
  const otherOn = draft.text !== "";
  const toggle = (id: string) => {
    if (multi) onChange({ selected: draft.selected.includes(id) ? draft.selected.filter((x) => x !== id) : [...draft.selected, id] });
    else onChange({ selected: [id], text: "" });
  };
  return (
    <div className="grid gap-2 @xl:grid-cols-2">
      {item.options.map((o) => {
        const on = draft.selected.includes(o.id);
        return (
          <button key={o.id} disabled={readOnly} onClick={() => toggle(o.id)}
            className={`flex items-start gap-2.5 rounded-lg border-2 px-3 py-2.5 text-left transition-colors
              ${on ? "border-accent bg-accent-soft" : "border-line"} ${readOnly ? "cursor-default" : "hover:border-line-strong"} ${readOnly && !on ? "opacity-40" : ""}`}>
            <span className={`mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center border-2 ${multi ? "rounded-[4px]" : "rounded-full"} ${on ? "border-accent bg-accent" : "border-line-strong"}`}>
              {on && (multi ? <Check size={10} className="text-white" /> : <span className="h-1.5 w-1.5 rounded-full bg-white" />)}
            </span>
            <span className="flex-1">
              <span className="flex items-center gap-1.5 text-[13.5px] font-medium">
                {o.label}
                {o.recommended && <span className="flex shrink-0 items-center gap-0.5 whitespace-nowrap rounded bg-[#d9a21b22] px-1 text-[10.5px] font-semibold text-[#d9a21b]"><Star size={9} className="fill-current" />推奨</span>}
              </span>
              {o.description && <span className="mt-0.5 block text-[12px] text-muted">{o.description}</span>}
            </span>
          </button>
        );
      })}
      {item.allow_other && (
        <label className={`flex items-center gap-2.5 rounded-lg border-2 px-3 py-2 @xl:col-span-2 ${otherOn ? "border-accent bg-accent-soft" : "border-line"} ${readOnly && !otherOn ? "opacity-40" : ""}`}>
          <MessageCircleQuestion size={15} className="shrink-0 text-faint" />
          <input value={draft.text} disabled={readOnly} placeholder="その他（自由記述）"
            onChange={(e) => onChange(multi ? { text: e.target.value } : { text: e.target.value, selected: e.target.value ? [] : draft.selected })}
            className="flex-1 bg-transparent text-[13px] outline-none" />
        </label>
      )}
    </div>
  );
}
