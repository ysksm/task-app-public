import { Link } from "@tanstack/react-router";
import { ArrowDown, ArrowUp, ClipboardCheck, Plus, Settings, Star, Trash2 } from "lucide-react";
import { useState } from "react";
import type { ProjectTier, RequirementsMode, State } from "../api/client";
import {
  useCreateLabel, useCreateState, useDeleteLabel, useDeleteState, useLabels, useProject, useStates,
  useUpdateLabel, useUpdateProject, useUpdateState,
} from "../api/hooks";
import { StateIcon } from "../components/icons";
import { CATEGORY_LABEL, REQ_MODE_HINT, REQ_MODE_LABEL, REQ_MODES, TIER_HINT, TIER_LABEL, TIERS } from "../lib/meta";
import { TierIcon } from "../components/pickers";
import { Header } from "./agentPages";

// S11: project settings — name, hierarchy rule, workflow states, labels (PRJ-04, HIE-07).

const CATEGORIES: State["category"][] = ["backlog", "unstarted", "started", "completed", "canceled"];
const input = "rounded-md border border-line bg-bg px-2 py-1 text-[13px] outline-none focus:border-accent";

function useError() {
  const [error, setError] = useState<string>();
  return { error, onError: (e: unknown) => setError((e as Error).message), clear: () => setError(undefined) };
}

export function ProjectSettingsPage({ projectKey }: { projectKey: string }) {
  const { data: project } = useProject(projectKey);
  if (!project) return <div className="p-6 text-faint">読み込み中...</div>;
  return (
    <div className="h-full overflow-y-auto">
      <Header icon={<Settings size={15} className="text-muted" />} title={`${project.name} の設定`}>
        <Link to="/p/$key/issues" params={{ key: projectKey }} className="ml-2 text-[12px] text-muted hover:text-fg">← イシュー一覧</Link>
      </Header>
      <div className="mx-auto max-w-[820px] space-y-9 px-6 py-6">
        <General projectKey={projectKey} name={project.name} description={project.description} allowSkip={project.allow_skip_levels} tier={project.tier ?? "normal"}
          mode={project.requirements_mode ?? "structured"} />
        <States projectKey={projectKey} />
        <Labels projectKey={projectKey} />
      </div>
    </div>
  );
}

function General({ projectKey, name, description, allowSkip, tier, mode }: { projectKey: string; name: string; description: string; allowSkip: boolean; tier: ProjectTier; mode: RequirementsMode }) {
  const update = useUpdateProject(projectKey);
  const [n, setN] = useState(name);
  const [d, setD] = useState(description);
  const err = useError();
  return (
    <section>
      <h2 className="mb-3 text-[14px] font-semibold">基本情報</h2>
      <div className="grid grid-cols-[120px_1fr] items-center gap-x-3 gap-y-2.5 text-[13px]">
        <span className="text-muted">キー</span>
        <span className="font-mono">{projectKey} <span className="ml-2 font-sans text-[12px] text-faint">（イシューのキーと Jira 連携に使うため変更できません）</span></span>
        <span className="text-muted">名前</span>
        <input value={n} onChange={(e) => setN(e.target.value)} onBlur={() => n.trim() && n !== name && update.mutate({ name: n.trim() }, { onError: err.onError })} className={input} />
        <span className="self-start pt-1 text-muted">説明</span>
        <textarea value={d} onChange={(e) => setD(e.target.value)} rows={2} onBlur={() => d !== description && update.mutate({ description: d }, { onError: err.onError })} className={`${input} resize-y`} />
        <span className="text-muted">ティア</span>
        <div className="flex flex-wrap items-center gap-1" data-testid="tier-setting">
          {TIERS.map((t) => (
            <button key={t} onClick={() => t !== tier && update.mutate({ tier: t }, { onError: err.onError })} title={TIER_HINT[t]}
              className={`flex items-center gap-1 rounded-md border px-2.5 py-1 text-[12.5px] ${t === tier ? "border-accent bg-active text-fg" : "border-line text-muted hover:bg-hover"}`}>
              <TierIcon tier={t} />{TIER_LABEL[t]}
            </button>
          ))}
          <span className="ml-2 text-[12px] text-faint">{TIER_HINT[tier]}。全体ボードとプロジェクト一覧はこの順に並びます</span>
        </div>
        <span className="text-muted">要求の書き方</span>
        <div className="flex flex-wrap items-center gap-1" data-testid="requirements-mode-setting">
          {REQ_MODES.map((m) => (
            <button key={m} onClick={() => m !== mode && update.mutate({ requirements_mode: m }, { onError: err.onError })} title={REQ_MODE_HINT[m]}
              data-testid={`requirements-mode-${m}`} aria-pressed={m === mode}
              className={`rounded-md border px-2.5 py-1 text-[12.5px] ${m === mode ? "border-accent bg-active text-fg" : "border-line text-muted hover:bg-hover"}`}>
              {REQ_MODE_LABEL[m]}
            </button>
          ))}
          <span className="ml-2 text-[12px] text-faint">{REQ_MODE_HINT[mode]}。ストーリーマップの画面が切り替わります（データはそのまま）</span>
        </div>
        <span className="text-muted">階層のスキップ</span>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={allowSkip} onChange={(e) => update.mutate({ allow_skip_levels: e.target.checked }, { onError: err.onError })} className="accent-[var(--accent)]" />
          <span>エピック直下のタスクなど、途中の階層を飛ばした配置を許可する</span>
        </label>
      </div>
      {err.error && <p className="mt-2 text-[12.5px] text-danger">{err.error}</p>}
    </section>
  );
}

function States({ projectKey }: { projectKey: string }) {
  const { data: all = [] } = useStates(projectKey);
  const states = all.filter((s) => s.applies_to === "all");
  const qaStates = all.filter((s) => s.applies_to === "qa");
  const create = useCreateState(projectKey);
  const update = useUpdateState(projectKey);
  const remove = useDeleteState(projectKey);
  const [name, setName] = useState("");
  const [category, setCategory] = useState<State["category"]>("started");
  const err = useError();
  const run = <T,>(m: { mutate: (a: T, o: { onError: (e: unknown) => void; onSuccess?: () => void }) => void }, arg: T, onSuccess?: () => void) => {
    err.clear();
    m.mutate(arg, { onError: err.onError, onSuccess });
  };

  return (
    <section>
      <h2 className="mb-1 text-[14px] font-semibold">ステータス</h2>
      <p className="mb-3 text-[12.5px] text-muted">
        カテゴリ（未着手・進行中・完了など）は進捗や Jira の statusCategory の計算に使うため、作成後は変更できません。並べ替えは同じカテゴリの中で行えます。
      </p>
      <div className="overflow-hidden rounded-lg border border-line">
        {states.map((s) => {
          const sameCat = states.filter((x) => x.category === s.category);
          const idx = sameCat.findIndex((x) => x.id === s.id);
          return (
            <div key={s.id} className="flex items-center gap-2 border-b border-line/60 px-3 py-1.5 last:border-0">
              <input type="color" value={s.color || "#8a8f98"} onChange={(e) => run(update, { id: s.id, color: e.target.value })}
                className="h-5 w-5 cursor-pointer rounded border-0 bg-transparent p-0" title="色" />
              <StateIcon state={s} size={14} />
              <NameInput value={s.name} onSave={(v) => run(update, { id: s.id, name: v })} />
              <span className="w-[86px] text-[12px] text-faint">{CATEGORY_LABEL[s.category]}</span>
              <span className="flex w-[150px] gap-1">
                {s.is_default ? <span className="rounded bg-accent-soft px-1.5 text-[11.5px] text-accent">新規の既定</span>
                  : (s.category === "backlog" || s.category === "unstarted") && (
                    <button onClick={() => run(update, { id: s.id, is_default: true })} className="flex items-center gap-1 rounded px-1 text-[11.5px] text-faint hover:bg-hover hover:text-fg"><Star size={11} />既定にする</button>
                  )}
                {s.is_acceptance ? <span className="flex items-center gap-1 rounded bg-accent-soft px-1.5 text-[11.5px] text-accent"><ClipboardCheck size={11} />受入待ち</span>
                  : s.category === "started" && (
                    <button onClick={() => run(update, { id: s.id, is_acceptance: true })} className="rounded px-1 text-[11.5px] text-faint hover:bg-hover hover:text-fg">受入待ちにする</button>
                  )}
              </span>
              <button disabled={idx === 0} onClick={() => run(update, { id: s.id, position: s.position - 1 })} className="rounded p-1 text-faint hover:bg-hover disabled:opacity-25" title="上へ"><ArrowUp size={13} /></button>
              <button disabled={idx === sameCat.length - 1} onClick={() => run(update, { id: s.id, position: s.position + 1 })} className="rounded p-1 text-faint hover:bg-hover disabled:opacity-25" title="下へ"><ArrowDown size={13} /></button>
              <button onClick={() => run(remove, s.id)} className="rounded p-1 text-faint hover:bg-hover hover:text-danger" title="削除（使われていない場合のみ）"><Trash2 size={13} /></button>
            </div>
          );
        })}
      </div>
      <form className="mt-2 flex items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (name.trim()) run(create, { name: name.trim(), category }, () => setName("")); }}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="新しいステータス（例: Blocked）" className={`${input} w-60`} />
        <select value={category} onChange={(e) => setCategory(e.target.value as State["category"])} className={input}>
          {CATEGORIES.map((c) => <option key={c} value={c}>{CATEGORY_LABEL[c]}</option>)}
        </select>
        <button disabled={!name.trim()} className="flex items-center gap-1 rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white disabled:opacity-40"><Plus size={13} />追加</button>
      </form>
      {err.error && <p className="mt-2 text-[12.5px] text-danger">{err.error}</p>}
      <p className="mt-3 text-[12px] text-faint">
        QA のステータス（{qaStates.map((s) => s.name).join(" / ")}）は回答の流れと連動しているため固定です。
      </p>
    </section>
  );
}

function NameInput({ value, onSave }: { value: string; onSave: (v: string) => void }) {
  const [v, setV] = useState(value);
  return (
    <input value={v} onChange={(e) => setV(e.target.value)}
      onBlur={() => (v.trim() && v !== value ? onSave(v.trim()) : setV(value))}
      onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
      className="min-w-0 flex-1 rounded border border-transparent bg-transparent px-1 py-0.5 text-[13px] outline-none hover:border-line focus:border-accent" />
  );
}

function Labels({ projectKey }: { projectKey: string }) {
  const { data: labels = [] } = useLabels(projectKey);
  const create = useCreateLabel(projectKey);
  const update = useUpdateLabel(projectKey);
  const remove = useDeleteLabel(projectKey);
  const [name, setName] = useState("");
  const err = useError();
  return (
    <section>
      <h2 className="mb-1 text-[14px] font-semibold">ラベル</h2>
      <p className="mb-3 text-[12.5px] text-muted">削除すると、付いているすべてのイシューから外れます。</p>
      {labels.length > 0 && (
        <div className="overflow-hidden rounded-lg border border-line">
          {labels.map((l) => (
            <div key={l.id} className="flex items-center gap-2 border-b border-line/60 px-3 py-1.5 last:border-0">
              <input type="color" value={l.color || "#6b7285"} onChange={(e) => update.mutate({ id: l.id, color: e.target.value }, { onError: err.onError })}
                className="h-5 w-5 cursor-pointer rounded border-0 bg-transparent p-0" title="色" />
              <NameInput value={l.name} onSave={(v) => update.mutate({ id: l.id, name: v }, { onError: err.onError })} />
              <button onClick={() => remove.mutate(l.id, { onError: err.onError })} className="rounded p-1 text-faint hover:bg-hover hover:text-danger" title="削除"><Trash2 size={13} /></button>
            </div>
          ))}
        </div>
      )}
      <form className="mt-2 flex items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (name.trim()) create.mutate({ name: name.trim() }, { onSuccess: () => setName(""), onError: err.onError }); }}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="新しいラベル" className={`${input} w-60`} />
        <button disabled={!name.trim()} className="flex items-center gap-1 rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white disabled:opacity-40"><Plus size={13} />追加</button>
      </form>
      {err.error && <p className="mt-2 text-[12.5px] text-danger">{err.error}</p>}
    </section>
  );
}
