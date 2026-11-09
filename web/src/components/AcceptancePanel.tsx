import { AlertTriangle, Check, ClipboardCheck, ArrowUp, Copy, Minus, Pencil, Plus, RotateCcw, Trash2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { ApiError, type Acceptance, type AcceptanceReview, type Criterion } from "../api/client";
import { useAcceptance, useReviewAcceptance, useSetCriteria } from "../api/hooks";
import { relativeTime } from "../lib/meta";
import { Avatar, StateIcon, TypeIcon } from "./icons";
import { criterionText } from "./IssueStories";

type Result = "ok" | "ng" | "na";

const RESULT: Record<Result, { label: string; on: string }> = {
  ok: { label: "OK", on: "bg-[#3fae6a] text-white border-[#3fae6a]" },
  ng: { label: "NG", on: "bg-[#e5484d] text-white border-[#e5484d]" },
  na: { label: "対象外", on: "bg-[#8a8f98] text-white border-[#8a8f98]" },
};

/** Readiness warnings (ACC-05) with the issues behind them. */
export function Readiness({ a }: { a: Acceptance }) {
  const r = a.readiness;
  if (r.warnings.length === 0) {
    return <div className="flex items-center gap-1.5 text-[12.5px] text-[#2e9a5b]"><Check size={14} />子イシューはすべて完了、未回答の QA はありません</div>;
  }
  return (
    <div className="space-y-1">
      {r.warnings.map((w) => (
        <div key={w} className="flex items-center gap-1.5 text-[12.5px] text-[#c2700f]"><AlertTriangle size={13} />{w}</div>
      ))}
    </div>
  );
}

/** The unfinished descendants and unanswered QA, each opening its detail. */
export function ReadinessItems({ a, onOpen, selected }: { a: Acceptance; onOpen: (key: string) => void; selected?: string }) {
  const items = a.readiness.items;
  if (items.length === 0) return null;
  const more = a.readiness.open_children + a.readiness.open_qa - items.length;
  return (
    <div className="overflow-hidden rounded-lg border border-[#c2700f44]">
      <div className="bg-[#c2700f11] px-3 py-1.5 text-[12px] text-[#c2700f]">確認が必要なイシュー（クリックで詳細）</div>
      {items.map((it) => (
        <button key={it.key} onClick={() => onOpen(it.key)}
          className={`flex w-full items-center gap-2 border-t border-line/60 px-3 py-1.5 text-left text-[12.5px] hover:bg-hover ${selected === it.key ? "bg-active" : ""}`}>
          <TypeIcon type={it.type} size={13} />
          <span className="w-[58px] shrink-0 font-mono text-[11.5px] text-faint">{it.key}</span>
          <span className="min-w-0 flex-1 truncate">{it.title}</span>
          {it.parent_key && <span className="shrink-0 font-mono text-[11px] text-faint">↑ {it.parent_key}</span>}
          <span className="flex w-[96px] shrink-0 items-center gap-1 truncate text-muted"><StateIcon state={it.state} size={12} />{it.state.name}</span>
          <span className="w-4 shrink-0"><Avatar user={it.assignee} size={15} /></span>
        </button>
      ))}
      {more > 0 && <div className="border-t border-line/60 px-3 py-1.5 text-[12px] text-faint">ほか {more} 件</div>}
    </div>
  );
}

/** Editable acceptance checklist (ACC-01). */
export function CriteriaEditor({ issueKey, a, onDone }: { issueKey: string; a: Acceptance; onDone?: () => void }) {
  const save = useSetCriteria(issueKey);
  const [rows, setRows] = useState(() => a.criteria.map((c) => ({ id: c.id as string | null, description: c.description })));
  const [error, setError] = useState<string>();
  const update = (i: number, description: string) => setRows(rows.map((r, j) => (j === i ? { ...r, description } : r)));
  const move = (i: number, d: number) => {
    const j = i + d;
    if (j < 0 || j >= rows.length) return;
    const next = [...rows];
    [next[i], next[j]] = [next[j], next[i]];
    setRows(next);
  };
  const submit = () => {
    const criteria = rows.map((r) => ({ ...r, description: r.description.trim() })).filter((r) => r.description);
    save.mutate(criteria, { onSuccess: () => onDone?.(), onError: (e) => setError((e as Error).message) });
  };
  return (
    <div className="space-y-1.5">
      {rows.map((r, i) => (
        <div key={i} className="group flex items-center gap-1.5">
          <span className="w-5 text-right text-[12px] text-faint">{i + 1}.</span>
          <input value={r.description} onChange={(e) => update(i, e.target.value)} autoFocus={!r.id && i === rows.length - 1}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.nativeEvent.isComposing) setRows([...rows, { id: null, description: "" }]); }}
            placeholder="例: 正しい資格情報でログインできる"
            className="min-w-0 flex-1 rounded-md border border-line bg-bg px-2 py-1 text-[13px] outline-none focus:border-accent" />
          <button title="上へ" onClick={() => move(i, -1)} className="rounded p-1 text-faint opacity-0 hover:bg-hover group-hover:opacity-100"><ArrowUp size={13} /></button>
          <button title="削除" onClick={() => setRows(rows.filter((_, j) => j !== i))} className="rounded p-1 text-faint hover:bg-hover hover:text-danger"><Trash2 size={13} /></button>
        </div>
      ))}
      <div className="flex items-center gap-2 pl-6 pt-1">
        <button onClick={() => setRows([...rows, { id: null, description: "" }])} className="flex items-center gap-1 text-[12px] text-muted hover:text-fg"><Plus size={13} />基準を追加</button>
        <span className="flex-1" />
        {error && <span className="text-[12px] text-danger">{error}</span>}
        {onDone && <button onClick={onDone} className="rounded-md px-2.5 py-1 text-[12px] text-muted hover:bg-hover">キャンセル</button>}
        <button onClick={submit} disabled={save.isPending} className="rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white disabled:opacity-50">保存</button>
      </div>
      {a.reviews.length > 0 && <p className="pl-6 text-[11.5px] text-faint">削除した基準も、過去の判定履歴には当時の文言で残ります。</p>}
    </div>
  );
}

/** Review history (ACC-04), newest first. */
export function ReviewHistory({ reviews }: { reviews: AcceptanceReview[] }) {
  if (reviews.length === 0) return null;
  return (
    <div className="space-y-2">
      {[...reviews].reverse().map((r) => (
        <div key={r.id} className={`rounded-lg border p-3 ${r.verdict === "accepted" ? "border-[#3fae6a55]" : "border-[#e5484d55]"}`}>
          <div className="mb-1.5 flex items-center gap-2 text-[12px]">
            <span className={`rounded px-1.5 font-medium ${r.verdict === "accepted" ? "bg-[#3fae6a22] text-[#2e9a5b]" : "bg-[#e5484d22] text-[#e5484d]"}`}>
              {r.round} 回目 · {r.verdict === "accepted" ? "合格" : "差し戻し"}
            </span>
            <Avatar user={r.reviewer} size={14} />
            <span className="text-muted">{r.reviewer.display_name}</span>
            <span className="text-faint">{relativeTime(r.created_at)}</span>
          </div>
          {r.summary && <p className="mb-1.5 text-[13px]">{r.summary}</p>}
          <ul className="space-y-0.5 text-[12.5px]">
            {r.items.map((it) => (
              <li key={it.criterion_id} className="flex gap-2">
                <ResultMark result={it.result as Result} />
                <span className="text-muted">{it.description}</span>
                {it.note && <span className="text-fg">— {it.note}</span>}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

function ResultMark({ result }: { result: Result }) {
  if (result === "ok") return <Check size={14} className="mt-0.5 shrink-0 text-[#2e9a5b]" />;
  if (result === "ng") return <X size={14} className="mt-0.5 shrink-0 text-[#e5484d]" />;
  return <Minus size={14} className="mt-0.5 shrink-0 text-faint" />;
}

/** The judgement form (ACC-03): per-criterion OK/NG, summary, reject or accept. */
export function ReviewForm({ issueKey, a, onReviewed }: { issueKey: string; a: Acceptance; onReviewed?: (verdict: string) => void }) {
  const review = useReviewAcceptance(issueKey);
  const [results, setResults] = useState<Record<string, { result?: Result; note: string }>>({});
  const [summary, setSummary] = useState("");
  const [error, setError] = useState<string>();
  const [needsForce, setNeedsForce] = useState(false);
  useEffect(() => { setResults({}); setSummary(""); setError(undefined); setNeedsForce(false); }, [issueKey]);

  const set = (id: string, patch: Partial<{ result: Result; note: string }>) =>
    setResults({ ...results, [id]: { ...{ note: "" }, ...results[id], ...patch } });
  const missing = a.criteria.filter((c) => !results[c.id]?.result).length;
  const hasNG = a.criteria.some((c) => results[c.id]?.result === "ng");

  const submit = (verdict: "accepted" | "rejected", force = false) => {
    setError(undefined);
    review.mutate(
      { verdict, summary, force, items: a.criteria.map((c) => ({ criterion_id: c.id, result: results[c.id]!.result!, note: results[c.id]?.note ?? "" })) },
      {
        onSuccess: () => { setNeedsForce(false); onReviewed?.(verdict); },
        onError: (e) => {
          if (e instanceof ApiError && e.status === 409 && verdict === "accepted") {
            setNeedsForce(true);
            setError("未完了の子イシューか未回答の QA があります。上の警告を確認したうえで、それでも合格にできます。");
            return;
          }
          setError((e as Error).message);
        },
      },
    );
  };

  return (
    <div className="space-y-3">
      {a.criteria.length === 0 && <p className="text-[12.5px] text-faint">受入基準がありません。基準なしでも総評を書いて判定できます。</p>}
      {a.criteria.map((c, i) => {
        const cur = results[c.id];
        return (
          <div key={c.id} className="rounded-lg border border-line p-2.5 @container">
            <div className="flex flex-col gap-2 @md:flex-row @md:items-center">
              <div className="flex min-w-0 flex-1 gap-2 text-[13px]"><span className="text-faint">{i + 1}.</span><span>{c.description}</span></div>
              <div className="flex shrink-0 gap-1">
                {(Object.keys(RESULT) as Result[]).map((r) => (
                  <button key={r} onClick={() => set(c.id, { result: r })}
                    className={`rounded-md border px-2.5 py-0.5 text-[12px] font-medium ${cur?.result === r ? RESULT[r].on : "border-line text-muted hover:bg-hover"}`}>
                    {RESULT[r].label}
                  </button>
                ))}
              </div>
            </div>
            {cur?.result && cur.result !== "ok" && (
              <input value={cur.note} onChange={(e) => set(c.id, { note: e.target.value })} placeholder={cur.result === "ng" ? "NG の理由" : "メモ（任意）"}
                className="mt-2 w-full rounded-md border border-line bg-bg px-2 py-1 text-[12.5px] outline-none focus:border-accent" />
            )}
          </div>
        );
      })}
      <textarea value={summary} onChange={(e) => setSummary(e.target.value)} rows={2}
        placeholder="総評（差し戻しのときは理由が必須。エージェントが読んで作業を続けます）"
        className="w-full resize-y rounded-md border border-line bg-bg px-2.5 py-1.5 text-[13px] outline-none focus:border-accent" />
      {error && <div className={`text-[12.5px] ${needsForce ? "text-[#c2700f]" : "text-danger"}`}>{error}</div>}
      <div className="flex flex-wrap items-center justify-end gap-2">
        {missing > 0 && <span className="mr-auto text-[12px] text-faint">未判定 {missing} 件</span>}
        <button onClick={() => submit("rejected")} disabled={missing > 0 || !summary.trim() || review.isPending}
          title={!summary.trim() ? "差し戻し理由を総評に書いてください" : undefined}
          className="flex items-center gap-1 rounded-md border border-[#e5484d66] px-3 py-1.5 text-[12.5px] font-medium text-[#e5484d] hover:bg-[#e5484d11] disabled:opacity-40">
          <RotateCcw size={13} />差し戻し（→ 作業中）
        </button>
        {needsForce ? (
          <button onClick={() => submit("accepted", true)} disabled={review.isPending}
            className="flex items-center gap-1 rounded-md bg-[#c2700f] px-3 py-1.5 text-[12.5px] font-medium text-white">
            <AlertTriangle size={13} />警告を承知で合格にする
          </button>
        ) : (
          <button onClick={() => submit("accepted")} disabled={missing > 0 || hasNG || review.isPending}
            title={hasNG ? "NG の基準があるときは合格にできません" : undefined}
            className="flex items-center gap-1 rounded-md bg-[#3fae6a] px-3 py-1.5 text-[12.5px] font-medium text-white disabled:opacity-40">
            <Check size={13} />合格（→ 完了）
          </button>
        )}
      </div>
    </div>
  );
}

/** Section in the issue detail for stories and features. */
export function AcceptanceSection({ issueKey, onOpenInbox, onOpen, storyCriteria = [] }: {
  issueKey: string; onOpenInbox?: () => void; onOpen?: (key: string) => void;
  /** acceptance criteria of the stories this issue is linked to (requirements layer) */
  storyCriteria?: Criterion[];
}) {
  const { data: a } = useAcceptance(issueKey);
  const setCriteria = useSetCriteria(issueKey);
  const [editing, setEditing] = useState(false);
  if (!a) return null;
  // Copy (not sync) the story's criteria that the checklist does not have yet (docs/08 §5.2, option A).
  const missing = storyCriteria.map(criterionText).filter((t) => !a.criteria.some((c) => c.description === t));
  const last = a.reviews.at(-1);
  return (
    <section className="mt-6">
      <div className="mb-2 flex items-center gap-2">
        <ClipboardCheck size={14} className="text-muted" />
        <h3 className="text-[13px] font-medium">受入基準</h3>
        <span className="text-[12px] text-faint">{a.criteria.length}</span>
        {last && (
          <span className={`rounded px-1.5 text-[11.5px] ${last.verdict === "accepted" ? "bg-[#3fae6a22] text-[#2e9a5b]" : "bg-[#e5484d22] text-[#e5484d]"}`}>
            {last.round} 回目 {last.verdict === "accepted" ? "合格" : "差し戻し"}
          </span>
        )}
        <span className="flex-1" />
        {a.in_acceptance && onOpenInbox && (
          <button onClick={onOpenInbox} className="rounded-md bg-accent px-2 py-0.5 text-[12px] font-medium text-white">受入検査する</button>
        )}
        {!editing && missing.length > 0 && (
          <button data-testid="copy-story-criteria" title="つながっているストーリーの受け入れ条件を、受入基準の末尾に写します（以後は同期しません）"
            onClick={() => setCriteria.mutate([...a.criteria.map((c) => ({ id: c.id, description: c.description })), ...missing.map((description) => ({ description }))])}
            className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[12px] text-muted hover:bg-hover"><Copy size={12} />ストーリーの条件から写す（{missing.length}）</button>
        )}
        {!editing && <button onClick={() => setEditing(true)} className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[12px] text-muted hover:bg-hover"><Pencil size={12} />編集</button>}
      </div>
      {editing ? (
        <CriteriaEditor issueKey={issueKey} a={a} onDone={() => setEditing(false)} />
      ) : a.criteria.length === 0 ? (
        <button onClick={() => setEditing(true)} className="w-full rounded-lg border border-dashed border-line px-3 py-3 text-[12.5px] text-faint hover:bg-hover">
          受入基準を追加（受入検査でチェックする項目）
        </button>
      ) : (
        <ol className="space-y-1 rounded-lg border border-line px-3 py-2 text-[13px]">
          {a.criteria.map((c, i) => {
            const res = last?.items.find((it) => it.criterion_id === c.id);
            return (
              <li key={c.id} className="flex gap-2">
                <span className="text-faint">{i + 1}.</span>
                <span className="flex-1">{c.description}</span>
                {res && <ResultMark result={res.result as Result} />}
              </li>
            );
          })}
        </ol>
      )}
      {a.in_acceptance && !editing && a.readiness.items.length > 0 && onOpen && (
        <div className="mt-2 space-y-1.5"><Readiness a={a} /><ReadinessItems a={a} onOpen={onOpen} /></div>
      )}
      {last?.verdict === "rejected" && !editing && (
        <p className="mt-2 rounded-md bg-[#e5484d11] px-2.5 py-1.5 text-[12.5px]"><span className="font-medium text-[#e5484d]">差し戻し理由: </span>{last.summary}</p>
      )}
    </section>
  );
}
