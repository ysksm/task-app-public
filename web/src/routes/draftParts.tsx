// Drafts of a story map (docs/08-devcycle.md §5.9). On a project's map: the
// 下書き menu (make a draft, open one). On a draft's map: a banner saying what
// it is, and the import plan → import into the target in one go.
import { Link, useNavigate } from "@tanstack/react-router";
import { AlertTriangle, ArrowRight, Check, FlaskConical, Plus, X } from "lucide-react";
import { useState } from "react";
import type { ImportItem, ImportPlan } from "../api/client";
import { useDraftOps, useDrafts, useImportPlan } from "../api/hooks";
import { Popover } from "../components/Menu";
import { relativeTime } from "../lib/meta";

const KIND_LABEL: Record<ImportItem["kind"], string> = {
  persona: "ペルソナ", activity: "活動", step: "手順", job: "ジョブ", release: "リリース", story: "ストーリー", criterion: "受け入れ条件",
};

const ACTION_META: Record<ImportItem["action"], { label: string; hint: string; tone: string }> = {
  create: { label: "増える", hint: "下書きで足したもの。キーは取り込むときに付く", tone: "text-[#3fae6a]" },
  update: { label: "変わる", hint: "下書きで直したもの", tone: "text-[#d4a017]" },
  archive: { label: "消える", hint: "下書きで消したもの（アーカイブする。キーは使い回さない）", tone: "text-danger" },
  restore: { label: "戻す", hint: "本番では消されているが、下書きで手を入れたもの", tone: "text-accent" },
};

/** On a project's own map: make a draft or open one. */
export function DraftsMenu({ projectKey }: { projectKey: string }) {
  const { data: drafts } = useDrafts(projectKey);
  const ops = useDraftOps();
  const navigate = useNavigate();
  const [err, setErr] = useState<string | null>(null);
  const make = (from: "copy" | "empty", close: () => void) =>
    ops.create(projectKey, { from })
      .then((d) => { close(); navigate({ to: "/p/$key/storymap", params: { key: d.key } }); })
      .catch((e: Error) => setErr(e.message));
  return (
    <Popover align="right" fixed trigger={() => (
      <button data-testid="drafts" title="ストーリーマップの下書き（ここで試してから、まとめて取り込む）"
        className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] text-muted hover:bg-hover hover:text-fg">
        <FlaskConical size={13} />下書き{drafts?.length ? <span className="text-faint">{drafts.length}</span> : null}
      </button>
    )}>
      {(close) => (
        <div className="w-[340px] p-1.5 text-[12.5px]" data-testid="drafts-panel">
          <p className="px-1.5 pb-2 text-[12px] leading-relaxed text-faint">
            下書きでは自由に足したり消したりできます。{projectKey} は変わりません。決まったら、取り込みの予定を見てからまとめて入れます。
          </p>
          <button onClick={() => make("copy", close)} data-testid="create-draft" className="flex w-full items-center gap-2 rounded px-1.5 py-1 text-left hover:bg-hover">
            <Plus size={13} className="text-accent" />いまのマップから下書きを作る
          </button>
          <button onClick={() => make("empty", close)} data-testid="create-draft-empty" className="flex w-full items-center gap-2 rounded px-1.5 py-1 text-left text-muted hover:bg-hover hover:text-fg">
            <Plus size={13} />空の下書きを作る
          </button>
          {err && <div className="px-1.5 py-1 text-danger">{err}</div>}
          {(drafts ?? []).length > 0 && <div className="mt-1.5 border-t border-line pt-1.5">
            {drafts!.map((d) => (
              <Link key={d.key} to="/p/$key/storymap" params={{ key: d.key }} onClick={close} data-testid={`draft-${d.key}`}
                className="flex items-center gap-2 rounded px-1.5 py-1 hover:bg-hover">
                <span className="font-mono text-[11.5px] text-faint">{d.key}</span><span className="truncate">{d.name}</span>
                <span className="ml-auto shrink-0 text-[11px] text-faint">{relativeTime(d.created_at)}</span>
              </Link>
            ))}
          </div>}
        </div>
      )}
    </Popover>
  );
}

/** On a draft's map: what this is, and the way into the import. */
export function DraftBanner({ draftKey, target }: { draftKey: string; target: string }) {
  const [planOpen, setPlanOpen] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const ops = useDraftOps();
  const navigate = useNavigate();
  return (
    <>
      <div className="flex shrink-0 items-center gap-2 border-b border-[#5b58d655] bg-[#5b58d614] px-4 py-1.5 text-[12.5px]" data-testid="draft-banner">
        <FlaskConical size={14} className="shrink-0 text-accent" />
        <span><b>下書き</b>: <Link to="/p/$key/storymap" params={{ key: target }} className="font-mono underline-offset-2 hover:underline" data-testid="draft-target">{target}</Link> に取り込む前の試し場所。ここで変えても {target} は変わりません</span>
        <span className="flex-1" />
        {discarding ? (
          <span className="flex items-center gap-1.5">
            <span className="text-faint">この下書きを捨てますか（{target} は変わりません）</span>
            <button data-testid="discard-draft-confirm" onClick={() => ops.discard(draftKey).then(() => navigate({ to: "/p/$key/storymap", params: { key: target } }))}
              className="rounded-md border border-line px-2 py-0.5 text-danger hover:bg-hover">捨てる</button>
            <button onClick={() => setDiscarding(false)} className="rounded-md px-2 py-0.5 text-muted hover:bg-hover">やめる</button>
          </span>
        ) : (
          <button data-testid="discard-draft" onClick={() => setDiscarding(true)} className="rounded-md px-2 py-0.5 text-muted hover:bg-hover hover:text-fg">下書きを捨てる</button>
        )}
        <button data-testid="open-import-plan" onClick={() => setPlanOpen(true)}
          className="flex items-center gap-1 rounded-md bg-accent px-2.5 py-0.5 font-medium text-white hover:opacity-90">
          {target} に取り込む<ArrowRight size={13} />
        </button>
      </div>
      {planOpen && <ImportPlanPanel draftKey={draftKey} target={target} onClose={() => setPlanOpen(false)} />}
    </>
  );
}

function ImportPlanPanel({ draftKey, target, onClose }: { draftKey: string; target: string; onClose: () => void }) {
  const { data: plan, error, isLoading } = useImportPlan(draftKey, true);
  const ops = useDraftOps();
  const [force, setForce] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [done, setDone] = useState<{ created: string[]; warnings: string[]; count: number } | null>(null);
  const apply = () => {
    if (!plan) return;
    setBusy(true);
    ops.apply(draftKey, force)
      .then((r) => setDone({ created: r.created, warnings: r.warnings, count: r.plan.items.length }))
      .catch((e: Error) => setErr(e.message))
      .finally(() => setBusy(false));
  };
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 pt-[8vh]" onMouseDown={onClose}>
      <div className="flex max-h-[80vh] w-[min(760px,92vw)] flex-col rounded-xl border border-line-strong bg-elev shadow-2xl" data-testid="import-plan" onMouseDown={(e) => e.stopPropagation()}>
        <header className="flex items-center gap-2 border-b border-line px-4 py-2.5">
          <h2 className="text-[14px] font-medium">取り込みの予定</h2>
          <span className="font-mono text-[12px] text-faint">{draftKey} → {target}</span>
          <span className="flex-1" />
          <button onClick={onClose} className="rounded p-1 text-faint hover:bg-hover hover:text-fg"><X size={14} /></button>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3 text-[12.5px]">
          {error && <div className="text-danger">{(error as Error).message}</div>}
          {isLoading && <div className="text-faint">読み込み中...</div>}
          {done ? <ImportDone target={target} {...done} onClose={onClose} /> : plan && <PlanBody plan={plan} />}
        </div>
        {!done && plan && (
          <footer className="flex items-center gap-3 border-t border-line px-4 py-2.5 text-[12.5px]">
            {plan.conflicts > 0 && (
              <label className="flex items-center gap-1.5 text-[#e0823d]">
                <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} data-testid="force-import" />
                {target} 側の変更 {plan.conflicts} 件を上書きして取り込む
              </label>
            )}
            {err && <span className="text-danger">{err}</span>}
            <span className="flex-1" />
            <button onClick={onClose} className="rounded-md px-3 py-1 text-muted hover:bg-hover">閉じる</button>
            <button data-testid="apply-import" onClick={apply} disabled={busy || plan.items.length === 0 || (plan.conflicts > 0 && !force)}
              className="rounded-md bg-accent px-3 py-1 font-medium text-white enabled:hover:opacity-90 disabled:opacity-40">
              {plan.items.length === 0 ? "取り込むものはありません" : `${plan.items.length} 件を取り込む`}
            </button>
          </footer>
        )}
      </div>
    </div>
  );
}

function PlanBody({ plan }: { plan: ImportPlan }) {
  const groups = (["create", "update", "archive", "restore"] as const).map((a) => [a, plan.items.filter((i) => i.action === a)] as const).filter(([, l]) => l.length);
  return (
    <div className="space-y-4">
      {plan.warnings.length > 0 && (
        <ul className="space-y-1 rounded-md border border-[#e0823d55] bg-[#e0823d0d] px-3 py-2 text-[#e0823d]" data-testid="plan-warnings">
          {plan.warnings.map((w) => <li key={w} className="flex gap-1.5"><AlertTriangle size={13} className="mt-0.5 shrink-0" />{w}</li>)}
        </ul>
      )}
      {plan.items.length === 0 && <p className="text-faint" data-testid="plan-empty">下書きと {plan.target_key} の間に違いはありません。</p>}
      {groups.map(([action, items]) => (
        <section key={action} data-testid={`plan-${action}`}>
          <h3 className={`mb-1 font-medium ${ACTION_META[action].tone}`}>{ACTION_META[action].label} {items.length}
            <span className="ml-2 font-normal text-faint">{ACTION_META[action].hint}</span></h3>
          <ul className="divide-y divide-line rounded-md border border-line">
            {items.map((it, i) => <PlanRow key={i} item={it} />)}
          </ul>
        </section>
      ))}
      {plan.target_only.length > 0 && (
        <details className="text-muted" data-testid="plan-target-only">
          <summary className="cursor-pointer">{plan.target_key} だけにあるもの {plan.target_only.length} 件（そのまま残る）</summary>
          <ul className="mt-1 space-y-0.5 pl-4">
            {plan.target_only.map((r) => <li key={r.key}><span className="font-mono text-[11.5px] text-faint">{r.key}</span> {KIND_LABEL[r.kind as ImportItem["kind"]] ?? r.kind} {r.label}</li>)}
          </ul>
        </details>
      )}
    </div>
  );
}

function PlanRow({ item }: { item: ImportItem }) {
  return (
    <li className={`px-3 py-1.5 ${item.conflict ? "bg-[#e0823d0d]" : ""}`} data-testid="plan-item" data-conflict={item.conflict || undefined}>
      <div className="flex items-center gap-2">
        <span className="w-[84px] shrink-0 text-[11.5px] text-faint">{KIND_LABEL[item.kind]}</span>
        <span className="shrink-0 font-mono text-[11.5px] text-faint">{item.target_key ?? `新規（${item.draft_key}）`}</span>
        <span className="truncate">{item.label}</span>
        {item.conflict && <span className="ml-auto flex shrink-0 items-center gap-1 text-[11.5px] text-[#e0823d]" data-testid="plan-conflict"><AlertTriangle size={12} />本番側でも変わっている</span>}
      </div>
      {item.action === "update" && item.changes.length > 0 && (
        <ul className="mt-0.5 space-y-0.5 pl-[92px] text-[12px]">
          {item.changes.map((c, i) => (
            <li key={i} className="flex gap-1.5"><span className="shrink-0 text-faint">{c.field}</span>
              <span className="truncate text-muted line-through decoration-faint">{c.from || "（なし）"}</span><ArrowRight size={11} className="mt-1 shrink-0 text-faint" /><span className="truncate">{c.to || "（なし）"}</span></li>
          ))}
        </ul>
      )}
    </li>
  );
}

function ImportDone({ target, created, warnings, count, onClose }: { target: string; created: string[]; warnings: string[]; count: number; onClose: () => void }) {
  return (
    <div className="space-y-3 py-2" data-testid="import-done">
      <p className="flex items-center gap-1.5 text-[13px]"><Check size={15} className="text-[#3fae6a]" />{target} に {count} 件を取り込みました。</p>
      {created.length > 0 && <p className="text-muted">新しいキー: <span className="font-mono text-[12px]">{created.join("、")}</span></p>}
      {warnings.map((w) => <p key={w} className="flex gap-1.5 text-[#e0823d]"><AlertTriangle size={13} className="mt-0.5 shrink-0" />{w}</p>)}
      <p className="text-faint">下書きはそのまま続けて使えます。もう一度取り込むと、その後の違いだけが入ります。</p>
      <div className="flex gap-2">
        <Link to="/p/$key/storymap" params={{ key: target }} onClick={onClose} data-testid="go-target" className="rounded-md bg-accent px-3 py-1 font-medium text-white hover:opacity-90">{target} のマップを見る</Link>
        <button onClick={onClose} className="rounded-md px-3 py-1 text-muted hover:bg-hover">下書きに戻る</button>
      </div>
    </div>
  );
}
