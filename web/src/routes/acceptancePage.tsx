import { useNavigate } from "@tanstack/react-router";
import { AlertTriangle, ArrowRight, ClipboardCheck } from "lucide-react";
import { useAcceptance, useAcceptanceQueue, useIssue } from "../api/hooks";
import { CriteriaEditor, Readiness, ReadinessItems, ReviewForm, ReviewHistory } from "../components/AcceptancePanel";
import { IssueDetail } from "../components/IssueDetail";
import { Avatar, StateIcon, TypeIcon } from "../components/icons";
import { relativeTime } from "../lib/meta";
import { Header } from "./agentPages";

/** S03: issues waiting for acceptance and the judgement form. */
export function AcceptanceInboxPage({ selected, peek, setSearch }: { selected?: string; peek?: string; setSearch: (s: { key?: string; peek?: string }) => void }) {
  const setSelected = (key?: string) => setSearch({ key });
  const { data: queue = [], isLoading } = useAcceptanceQueue();
  const current = selected ?? queue[0]?.issue.key;
  return (
    <div className="flex h-full flex-col">
      <Header icon={<ClipboardCheck size={15} className="text-muted" />} title="受入検査">
        <span className="text-[12px] text-faint">受入待ち {queue.length} 件</span>
      </Header>
      <div className="flex min-h-0 flex-1">
        <div className={`w-[340px] shrink-0 overflow-y-auto border-r border-line ${peek ? "hidden" : ""}`}>
          {isLoading && <div className="p-4 text-faint">読み込み中...</div>}
          {!isLoading && queue.length === 0 && (
            <div className="p-6 text-center text-[13px] leading-relaxed text-faint">受入待ちのイシューはありません。<br />ストーリー / フィーチャーを「受入待ち」にすると、ここに並びます。</div>
          )}
          {queue.map((q) => (
            <button key={q.issue.key} onClick={() => setSelected(q.issue.key)}
              className={`block w-full border-b border-line/60 px-4 py-3 text-left hover:bg-hover ${q.issue.key === current ? "bg-active" : ""}`}>
              <div className="mb-1 flex items-center gap-1.5 text-[11.5px] text-faint">
                <TypeIcon type={q.issue.type} size={13} />
                <span className="font-mono">{q.issue.key}</span>
                {q.rounds > 0 && <span>· {q.rounds + 1} 回目</span>}
                <span className="ml-auto">{relativeTime(q.issue.updated_at)}</span>
              </div>
              <div className="line-clamp-2 text-[13px] leading-snug">{q.issue.title}</div>
              <div className="mt-1 flex items-center gap-2 text-[11.5px] text-faint">
                <span>基準 {q.criteria_count}</span>
                {q.issue.progress.total > 0 && <span>· 子 {q.issue.progress.done}/{q.issue.progress.total} 完了</span>}
                {q.readiness.warnings.length > 0 && <AlertTriangle size={12} className="text-[#c2700f]" />}
                {q.last_review?.verdict === "rejected" && <span className="ml-auto text-[#e5484d]">前回 差し戻し</span>}
              </div>
            </button>
          ))}
        </div>
        <div className="min-w-0 flex-1 overflow-y-auto">
          {current && <AcceptanceDetail key={current} issueKey={current} onReviewed={() => setSelected(undefined)}
            peek={peek} onPeek={(k) => setSearch({ key: current, peek: k })} />}
        </div>
        {peek && (
          <div className="w-[min(720px,55%)] shrink-0 border-l border-line bg-bg">
            <IssueDetail key={peek} issueKey={peek} mode="peek" onClose={() => setSearch({ key: current })} onOpen={(k) => setSearch({ key: current, peek: k })} />
          </div>
        )}
      </div>
    </div>
  );
}

function AcceptanceDetail({ issueKey, onReviewed, peek, onPeek }: { issueKey: string; onReviewed: () => void; peek?: string; onPeek: (key: string) => void }) {
  const { data: a } = useAcceptance(issueKey);
  const { data: issue } = useIssue(issueKey);
  const navigate = useNavigate();
  if (!a || !issue) return <div className="p-6 text-faint">読み込み中...</div>;
  const round = a.reviews.length + 1;
  const open = () => navigate({ to: "/p/$key/issues", params: { key: issue.project_key }, search: { peek: issueKey } });
  return (
    <div className="mx-auto max-w-[760px] space-y-5 px-6 py-5">
      <div>
        <div className="mb-1 flex items-center gap-2 text-[12px] text-muted">
          <TypeIcon type={issue.type} size={14} />
          <span className="font-mono">{issue.key}</span>
          <StateIcon state={issue.state} size={13} /> {issue.state.name}
          {issue.assignee && <><span>·</span><Avatar user={issue.assignee} size={14} />{issue.assignee.display_name}</>}
          <button onClick={open} className="ml-auto flex items-center gap-1 hover:text-fg">詳細を開く <ArrowRight size={12} /></button>
        </div>
        <h2 className="text-[19px] font-semibold">{issue.title}</h2>
        <div className="mt-2 flex items-center gap-3">
          <span className="rounded bg-accent-soft px-1.5 text-[12px] font-medium text-accent">受入 {round} 回目</span>
          <Readiness a={a} />
        </div>
        <div className="mt-3"><ReadinessItems a={a} onOpen={onPeek} selected={peek} /></div>
      </div>

      {a.in_acceptance ? (
        <section>
          <h3 className="mb-2 text-[13px] font-medium">判定</h3>
          <ReviewForm issueKey={issueKey} a={a} onReviewed={onReviewed} />
        </section>
      ) : (
        <p className="rounded-md bg-hover px-3 py-2 text-[12.5px] text-muted">このイシューは受入待ちではありません（{issue.state.name}）。</p>
      )}

      <details className="group">
        <summary className="cursor-pointer text-[12.5px] text-muted hover:text-fg">受入基準を編集</summary>
        <div className="mt-2"><CriteriaEditor key={a.criteria.map((c) => c.id).join()} issueKey={issueKey} a={a} /></div>
      </details>

      {a.reviews.length > 0 && (
        <section>
          <h3 className="mb-2 text-[13px] font-medium">これまでの判定</h3>
          <ReviewHistory reviews={a.reviews} />
        </section>
      )}
    </div>
  );
}
