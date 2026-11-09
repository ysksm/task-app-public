// S15 story map and S16 story detail (docs/08-devcycle.md §5.5): the
// requirements layer of a project. Columns are the steps of each activity,
// rows are release slices (name, goal, status, progress; §5.8), cards are
// stories coloured by their linked issues.
// Option D (docs/09-requirements-loose.md): a one-line quick input puts a story
// into the first column 未整理 (no step); a story has a free body; criteria are
// one line with given/when behind 詳しく; personas and activities appear only
// when used.
// Option E (docs/09 §6): the project's requirements_mode picks the screen.
// quick = the option D screen above; structured = the option A screen (place
// under a step, Given/When/Then, gaps as warnings). The data is the same.
// Drafts (docs/08 §5.9): a draft project shows a banner and the import plan;
// a normal project has the 下書き menu (draftParts.tsx).
import { Link, useNavigate } from "@tanstack/react-router";
import { AlertTriangle, ArrowDown, ArrowUp, Bot, FileText, FolderKanban, Inbox, ListChecks, Map as MapIcon, Plus, Trash2, User, Users, X } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import Markdown from "react-markdown";
import type { Criterion, Job, Persona, ReleaseSlice, Step, Story, StoryActivity, StoryMap, StoryStatus } from "../api/client";
import { useProject, useStory, useStoryMap, useStoryMapOps } from "../api/hooks";
import { Popover } from "../components/Menu";
import { StateIcon } from "../components/icons";
import { relativeTime } from "../lib/meta";
import { DraftBanner, DraftsMenu } from "./draftParts";

export interface StoryMapSearch {
  persona?: string;
  peek?: string;
  panel?: boolean;
}

export const STATUS_META: Record<StoryStatus, { label: string; hint: string; card: string; dot: string }> = {
  draft: { label: "条件なし", hint: "受け入れ条件がまだない", card: "border-line bg-hover/60 text-muted", dot: "bg-[#8a90a2]" },
  ready: { label: "着手可能", hint: "条件がある。つながったイシューはまだ始まっていない", card: "border-line-strong bg-elev", dot: "bg-[#e6e7ea]" },
  in_progress: { label: "実装中", hint: "つながったイシューが進行中", card: "border-[#d4a017] bg-[#d4a0171f]", dot: "bg-[#d4a017]" },
  done: { label: "完了", hint: "つながったイシューがすべて終わった", card: "border-[#3fae6a] bg-[#3fae6a1f]", dot: "bg-[#3fae6a]" },
};

const UNPLANNED = "";

type Column = { activity: StoryActivity; step: Step | null };

function useErr() {
  const [err, setErr] = useState<string | null>(null);
  const run = (p: Promise<unknown>) => p.then(() => setErr(null)).catch((e: Error) => setErr(e.message));
  return [err, run] as const;
}

export function StoryMapPage({ projectKey, search, setSearch }: { projectKey: string; search: StoryMapSearch; setSearch: (s: StoryMapSearch) => void }) {
  const { data: project } = useProject(projectKey);
  const { data: map, error, isLoading } = useStoryMap(projectKey);
  const ops = useStoryMapOps(projectKey);
  const [err, run] = useErr();
  const persona = search.persona;
  const open = (key: string) => setSearch({ ...search, peek: key });

  const activities = useMemo(() => (map?.activities ?? []).filter((a) => !persona || a.persona_key === persona || !a.persona_key), [map, persona]);
  const stories = useMemo(() => (map?.stories ?? []).filter((s) => !persona || s.persona_key === persona || !s.persona_key), [map, persona]);
  const slices = map?.release_slices ?? [];
  // The first slice not shipped yet is what to build next (the walking skeleton until it ships).
  const nextKey = slices.find((r) => r.status !== "shipped")?.key;
  const releases = useMemo(() => {
    const all = map?.releases ?? [];
    return [...all, UNPLANNED];
  }, [map]);
  const draftOf = map?.draft_of ?? project?.draft_of ?? null;
  // A draft has no issues of its own; links are made after the import.
  const shown = useMemo(() => (map && draftOf ? { ...map, gaps: { ...map.gaps, stories_without_issues: [] } } : map), [map, draftOf]);
  const gapCount = shown ? Object.values(shown.gaps).reduce((n, l) => n + l.length, 0) : 0;
  const unsorted = stories.filter((s) => s.step_keys.length === 0);
  // Project first: it is refetched when the setting changes.
  const quick = (project?.requirements_mode ?? map?.mode ?? "quick") === "quick";

  // One grid column per step; an activity without steps still takes one column.
  const columns: Column[] = activities.flatMap((a): Column[] => (a.steps.length ? a.steps.map((s) => ({ activity: a, step: s })) : [{ activity: a, step: null }]));

  return (
    <div className="flex h-full min-w-0">
      {search.panel && map && <PersonaPanel map={map} projectKey={projectKey} onClose={() => setSearch({ ...search, panel: undefined })} />}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-11 shrink-0 items-center gap-2 overflow-x-auto border-b border-line px-4 [scrollbar-width:none]">
          <MapIcon size={15} className="text-muted" />
          <h1 className="whitespace-nowrap text-[14px] font-medium">ストーリーマップ</h1>
          <span className="whitespace-nowrap text-[12px] text-faint">{project?.name ?? projectKey} <span className="font-mono">{projectKey}</span></span>
          {(!quick || (map?.personas.length ?? 0) > 0) && (
            <div className="ml-2 flex shrink-0 rounded-md border border-line p-0.5" data-testid="persona-filter">
              <PersonaTab active={!persona} onClick={() => setSearch({ ...search, persona: undefined })}>すべて</PersonaTab>
              {(map?.personas ?? []).map((p) => (
                <PersonaTab key={p.key} active={persona === p.key} onClick={() => setSearch({ ...search, persona: p.key })}>
                  {p.kind === "agent" ? <Bot size={12} /> : <User size={12} />}{p.name}
                </PersonaTab>
              ))}
            </div>
          )}
          <button onClick={() => setSearch({ ...search, panel: search.panel ? undefined : true })} data-testid="toggle-panel"
            className={`flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] ${search.panel ? "bg-active text-fg" : "text-muted hover:bg-hover hover:text-fg"}`}>
            <Users size={13} />ペルソナ・ジョブ
          </button>
          {shown && <GapsButton map={shown} count={gapCount} onOpen={open} warn={!quick} />}
          <ModeBadge projectKey={projectKey} quick={quick} />
          <Legend />
          <span className="flex-1" />
          {map && !draftOf && <DraftsMenu projectKey={projectKey} />}
          {!draftOf && <Link to="/p/$key/issues" params={{ key: projectKey }} className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] text-muted hover:bg-hover hover:text-fg">
            <FolderKanban size={13} />イシュー一覧
          </Link>}
        </header>
        {draftOf && <DraftBanner draftKey={projectKey} target={draftOf} />}
        {err && <div className="border-b border-line bg-[#e5484d14] px-4 py-1.5 text-[12px] text-danger">{err}</div>}
        <div className="min-h-0 flex-1 overflow-auto p-4" data-testid="storymap" data-mode={quick ? "quick" : "structured"}>
          {error && <div className="text-danger">{(error as Error).message}</div>}
          {isLoading && <div className="text-faint">読み込み中...</div>}
          {map && quick && <QuickInput onAdd={(title) => run(ops.createStory({ title }))} />}
          {map && !quick && (
            // structured: stories without a step are a gap to fix, shown above the map
            <section className={`mb-4 max-w-[880px] rounded-lg border px-3 py-2 ${unsorted.length ? "border-[#e0823d55] bg-[#e0823d0d]" : "border-line"}`} data-testid="unplaced">
              <h2 className={`mb-2 flex items-center gap-1.5 text-[12.5px] font-medium ${unsorted.length ? "text-[#e0823d]" : "text-muted"}`}>
                {unsorted.length > 0 && <AlertTriangle size={13} />}未整理 {unsorted.length}件
                <span className="font-normal text-faint">手順につながっていないストーリー。詳細の「手順」から置き場所を決める</span>
              </h2>
              <div className="flex flex-wrap gap-2">
                {unsorted.map((s) => <StoryCard key={s.key} story={s} selected={search.peek === s.key} onOpen={open} />)}
                <AddInline label="ストーリー" testId="add-unplaced-story" onAdd={(title) => run(ops.createStory({ title, persona_key: persona ?? null }).then((s) => open(s.key)))} />
              </div>
            </section>
          )}
          {map && (
            <div className="inline-grid gap-x-2 gap-y-2" style={{ gridTemplateColumns: `168px ${quick ? "210px " : ""}${columns.length ? `repeat(${columns.length}, 210px) ` : ""}190px` }}>
              {/* row 1: activities; in quick mode the first column is 未整理 (stories without a step) */}
              <div />
              {quick && <div className="row-span-2 flex items-center gap-1.5 self-stretch rounded-lg border border-dashed border-[#d4a01788] bg-[#d4a01714] px-2.5 py-1.5 text-[13px] font-medium text-[#d4a017]" data-testid="unsorted-header"
                title="手順につながっていないストーリー。ひとこと入力で入ったものはここに置かれる">
                <Inbox size={14} />未整理<span className="text-[12px] font-normal">{unsorted.length}</span>
              </div>}
              {activities.map((a) => (
                <ActivityHeader key={a.key} activity={a} span={Math.max(1, a.steps.length)} projectKey={projectKey} personas={map.personas} run={run} />
              ))}
              <AddInline label="活動" testId="add-activity" onAdd={(name) => run(ops.createActivity({ name, persona_key: persona ?? null }))} className="row-span-2" />
              {/* row 2: steps */}
              <div />
              {columns.map(({ activity, step }) =>
                step ? <StepHeader key={step.key} stepKey={step.key} name={step.name} projectKey={projectKey} run={run} />
                  : <AddInline key={activity.key + "-add"} label="手順" testId={`add-step-${activity.key}`} onAdd={(name) => run(ops.createStep(activity.key, { name }))} />,
              )}
              {/* release rows */}
              {releases.map((rel, i) => (
                <ReleaseRow key={rel || "unplanned"} release={rel} slice={slices.find((r) => r.name === rel)} next={!!rel && slices.find((r) => r.name === rel)?.key === nextKey}
                  prev={slices[i - 1]} after={slices[i + 1]} projectKey={projectKey} run={run} columns={columns} stories={stories} unsorted={quick ? unsorted : null} selected={search.peek} onOpen={open}
                  onAdd={(stepKey, title) => run(ops.createStory({ title, step_keys: [stepKey], release: rel, persona_key: persona ?? null }).then((s) => open(s.key)))} />
              ))}
              <div className="col-span-full pt-1">
                <AddInline label="リリース" testId="add-release" placeholder={quick ? "例: いま / つぎ / あとで" : "例: R2"} onAdd={(name) => run(ops.createRelease({ name }))} />
              </div>
            </div>
          )}
          {map && quick && map.activities.length === 0 && (
            <p className="mt-4 max-w-[640px] text-[12px] leading-relaxed text-faint" data-testid="no-activities-hint">
              活動と手順は、まとめ方が見えてきたら「＋ 活動」で足せます。それまでは上の入力にひとことずつ書いて、未整理に置いておけば十分です。
            </p>
          )}
        </div>
      </div>
      {search.peek && map && (
        <div className="w-[min(560px,45%)] shrink-0 border-l border-line bg-bg">
          <StoryPeek key={search.peek} projectKey={projectKey} storyKey={search.peek} map={map} quick={quick} draftOf={draftOf} onClose={() => setSearch({ ...search, peek: undefined })} />
        </div>
      )}
    </div>
  );
}

/** E: which way this project writes requirements; the setting lives in project settings. */
function ModeBadge({ projectKey, quick }: { projectKey: string; quick: boolean }) {
  return (
    <Link to="/p/$key/settings" params={{ key: projectKey }} data-testid="mode-badge" title="要求の書き方（プロジェクト設定で変える）"
      className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md border border-line px-2 py-0.5 text-[11.5px] text-muted hover:bg-hover hover:text-fg">
      {quick ? "まず書く" : "整理して書く"}
    </Link>
  );
}

/** D1: one line, Enter, done. The story goes to 未整理 with nothing else chosen. */
function QuickInput({ onAdd }: { onAdd: (title: string) => void }) {
  const [v, setV] = useState("");
  return (
    <form className="mb-4 flex max-w-[880px] items-center gap-2 rounded-lg border border-line bg-elev px-3 py-2 focus-within:border-accent"
      onSubmit={(e) => { e.preventDefault(); const t = v.trim(); if (t) { onAdd(t); setV(""); } }}>
      <Plus size={14} className="shrink-0 text-accent" />
      <input value={v} onChange={(e) => setV(e.target.value)} data-testid="quick-story" placeholder="思いついたことをひとこと（Enter で未整理に置く。誰の・どこに・いつは後で）"
        onKeyDown={(e) => { if (e.key === "Enter" && e.nativeEvent.isComposing) e.preventDefault(); }}
        className="min-w-0 flex-1 bg-transparent text-[13px] outline-none placeholder:text-faint" />
      <span className="shrink-0 text-[11px] text-faint">Enter</span>
    </form>
  );
}

function PersonaTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button onClick={onClick} className={`flex items-center gap-1 whitespace-nowrap rounded px-2.5 py-0.5 text-[12px] ${active ? "bg-active text-fg" : "text-muted hover:text-fg"}`}>{children}</button>
  );
}

function Legend() {
  return (
    <div className="ml-1 flex shrink-0 items-center gap-2.5 text-[11.5px] text-faint">
      {(Object.keys(STATUS_META) as StoryStatus[]).map((s) => (
        <span key={s} className="flex items-center gap-1 whitespace-nowrap" title={STATUS_META[s].hint}><span className={`h-2 w-2 rounded-full ${STATUS_META[s].dot}`} />{STATUS_META[s].label}</span>
      ))}
    </div>
  );
}

const GAP_LABEL: Record<keyof StoryMap["gaps"], string> = {
  stories_without_steps: "手順につながっていないストーリー",
  steps_without_stories: "ストーリーのない手順",
  stories_without_criteria: "受け入れ条件のないストーリー",
  stories_without_issues: "イシューのつながっていないストーリー",
  jobs_without_activities: "活動につながっていないジョブ",
};

function GapsButton({ map, count, onOpen, warn }: { map: StoryMap; count: number; onOpen: (key: string) => void; warn: boolean }) {
  const names = new Map<string, string>([
    ...map.stories.map((s) => [s.key, s.title] as const),
    ...map.activities.flatMap((a) => a.steps.map((s) => [s.key, `${a.name} › ${s.name}`] as const)),
    ...map.jobs.map((j) => [j.key, j.motivation] as const),
  ]);
  return (
    <Popover fixed trigger={() => (
      warn ? (
        // structured: a gap is something to fix
        <button data-testid="gaps" data-warn="true" title="つながりの抜け（整理して書くプロジェクトでは直すもの）" className={`flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] ${count ? "text-[#e0823d] hover:bg-hover" : "text-faint"}`}>
          <AlertTriangle size={13} />抜け {count}
        </button>
      ) : (
        <button data-testid="gaps" title="つながりの抜け。警告ではなく、整理するときの手がかり" className="flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-2 py-1 text-[12px] text-muted hover:bg-hover hover:text-fg">
          <ListChecks size={13} />抜け {count}
        </button>
      )
    )}>
      {(close) => (
        <div className="max-h-[60vh] w-[380px] overflow-y-auto p-2 text-[12.5px]" data-testid="gaps-panel">
          {count === 0 && <div className="px-1 py-2 text-faint">抜けはありません</div>}
          {(Object.keys(GAP_LABEL) as (keyof StoryMap["gaps"])[]).filter((g) => map.gaps[g].length).map((g) => (
            <div key={g} className="mb-2">
              <div className="mb-1 font-medium text-muted">{GAP_LABEL[g]} <span className="text-faint">{map.gaps[g].length}</span></div>
              {map.gaps[g].map((k) => (
                <button key={k} disabled={!k.includes("-ST")} onClick={() => (onOpen(k), close())}
                  className="flex w-full gap-2 rounded px-1.5 py-0.5 text-left enabled:hover:bg-hover">
                  <span className="shrink-0 font-mono text-[11.5px] text-faint">{k}</span><span className="truncate">{names.get(k)}</span>
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
    </Popover>
  );
}

function ActivityHeader({ activity: a, span, projectKey, personas, run }: { activity: StoryActivity; span: number; projectKey: string; personas: Persona[]; run: (p: Promise<unknown>) => void }) {
  const ops = useStoryMapOps(projectKey);
  const persona = personas.find((p) => p.key === a.persona_key);
  return (
    <div className="group flex min-w-0 items-center gap-1 rounded-lg bg-accent px-2.5 py-1.5 text-white" style={{ gridColumn: `span ${span}` }} data-testid={`activity-${a.key}`}>
      <InlineText value={a.name} onSave={(name) => run(ops.updateActivity(a.key, { name }))} className="min-w-0 flex-1 truncate text-[13px] font-medium" inputClass="text-fg" />
      {persona && <span className="shrink-0 rounded bg-white/20 px-1 text-[10.5px]" title="ペルソナ">{persona.name}</span>}
      <span className="shrink-0 font-mono text-[10.5px] text-white/60">{a.key.split("-").slice(1).join("-")}</span>
      <AddInline label="" icon testId={`activity-add-step-${a.key}`} onAdd={(name) => run(ops.createStep(a.key, { name }))} compact />
      <button title="活動をアーカイブ" onClick={() => window.confirm(`活動「${a.name}」と手順をアーカイブしますか？`) && run(ops.archiveActivity(a.key))}
        className="hidden shrink-0 rounded p-0.5 text-white/70 hover:bg-white/20 group-hover:block"><Trash2 size={12} /></button>
    </div>
  );
}

function StepHeader({ stepKey, name, projectKey, run }: { stepKey: string; name: string; projectKey: string; run: (p: Promise<unknown>) => void }) {
  const ops = useStoryMapOps(projectKey);
  return (
    <div className="group flex min-w-0 items-center gap-1 rounded-md border border-[#e6d88a] bg-[#fff6c9] px-2 py-1 text-[#3b3520]" data-testid={`step-${stepKey}`}>
      <InlineText value={name} onSave={(n) => run(ops.updateStep(stepKey, { name: n }))} className="min-w-0 flex-1 text-[12.5px] font-medium" />
      <span className="shrink-0 font-mono text-[10px] text-[#8a8060]">{stepKey.split("-").slice(1).join("-")}</span>
      <button title="手順をアーカイブ" onClick={() => window.confirm(`手順「${name}」をアーカイブしますか？`) && run(ops.archiveStep(stepKey))}
        className="hidden shrink-0 rounded p-0.5 text-[#8a8060] hover:bg-black/10 group-hover:block"><Trash2 size={11} /></button>
    </div>
  );
}

const RELEASE_STATUS: Record<ReleaseSlice["status"], { label: string; cls: string }> = {
  planned: { label: "計画中", cls: "bg-[#9aa0b022] text-muted" },
  building: { label: "作っている", cls: "bg-[#d9a21b22] text-[#a87a0c]" },
  shipped: { label: "出した", cls: "bg-[#2b7a4b1f] text-[#2b7a4b]" },
};

/** §5.8: the row header of a release slice — name, outcome, status and progress. */
function SliceHeader({ slice: r, next, prev, after, projectKey, run }: {
  slice: ReleaseSlice; next: boolean; prev?: ReleaseSlice; after?: ReleaseSlice; projectKey: string; run: (p: Promise<unknown>) => void;
}) {
  const ops = useStoryMapOps(projectKey);
  const swap = (o?: ReleaseSlice) => o && run(Promise.all([ops.updateRelease(r.key, { position: o.position }), ops.updateRelease(o.key, { position: r.position })]));
  return (
    <div className={`group flex flex-col gap-1 pt-2 ${next ? "border-t-[3px] border-solid border-accent" : "border-t-2 border-dashed border-line"} ${r.status === "shipped" ? "opacity-60" : ""}`}
      data-testid={`release-${r.name}`} data-status={r.status}>
      <div className="flex items-center gap-1">
        <InlineText value={r.name} onSave={(name) => run(ops.updateRelease(r.key, { name }))} className="min-w-0 truncate text-[12.5px] font-semibold" testId={`release-name-${r.name}`} />
        <span className="flex-1" />
        <button title="上へ" disabled={!prev} onClick={() => swap(prev)} className="hidden rounded p-0.5 text-faint enabled:hover:bg-hover group-hover:block disabled:opacity-30"><ArrowUp size={11} /></button>
        <button title="下へ" disabled={!after} onClick={() => swap(after)} className="hidden rounded p-0.5 text-faint enabled:hover:bg-hover group-hover:block disabled:opacity-30"><ArrowDown size={11} /></button>
        {r.stories === 0 && (
          <button title="リリースを消す（ストーリーがないときだけ）" onClick={() => run(ops.deleteRelease(r.key))} className="hidden rounded p-0.5 text-faint hover:bg-hover group-hover:block"><Trash2 size={11} /></button>
        )}
      </div>
      {next && <span className="w-fit rounded bg-accent px-1.5 text-[10.5px] font-medium text-white" data-testid="release-next">次に出す</span>}
      <InlineText value={r.goal} onSave={(goal) => run(ops.updateRelease(r.key, { goal }))} multiline placeholder="ねらい（これで何ができるか）"
        className="text-[11.5px] leading-snug text-muted" testId={`release-goal-${r.name}`} />
      <div className="flex items-center gap-1.5 text-[11px]">
        <select value={r.status} onChange={(e) => run(ops.updateRelease(r.key, { status: e.target.value as ReleaseSlice["status"] }))} data-testid={`release-status-${r.name}`}
          className={`rounded px-1 py-0.5 text-[11px] outline-none ${RELEASE_STATUS[r.status].cls}`}>
          {(Object.keys(RELEASE_STATUS) as ReleaseSlice["status"][]).map((k) => <option key={k} value={k}>{RELEASE_STATUS[k].label}</option>)}
        </select>
        <span className="text-faint" data-testid={`release-progress-${r.name}`} title="完了したストーリー / ストーリー">{r.done}/{r.stories} 完了</span>
      </div>
      {r.metric && <div className="text-[10.5px] text-faint" title="うまくいったかの目安">目安: {r.metric}</div>}
    </div>
  );
}

function ReleaseRow({ release, slice, next, prev, after, projectKey, run, columns, stories, unsorted, selected, onOpen, onAdd }: {
  release: string;
  slice?: ReleaseSlice;
  next: boolean;
  prev?: ReleaseSlice;
  after?: ReleaseSlice;
  projectKey: string;
  run: (p: Promise<unknown>) => void;
  columns: Column[];
  stories: Story[];
  unsorted: Story[] | null; // null: structured, no 未整理 column
  selected?: string;
  onOpen: (key: string) => void;
  onAdd: (stepKey: string, title: string) => void;
}) {
  // The line above 次に出す runs across the whole map, as on a paper story map.
  const line = next ? "border-t-[3px] border-solid border-accent/50" : "border-t-2 border-dashed border-line";
  return (
    <>
      {slice ? <SliceHeader slice={slice} next={next} prev={prev} after={after} projectKey={projectKey} run={run} /> : (
        <div className="border-t-2 border-dashed border-line pt-2 text-[12px] font-medium text-muted" data-testid={`release-${release || "unplanned"}`}>
          {release || <span className="text-faint">未定</span>}
        </div>
      )}
      {unsorted && (
        <div className={`flex min-h-[64px] flex-col gap-1.5 rounded-b-md ${line} bg-[#d4a0170a] pt-2`} data-testid={`cell-unsorted-${release || "unplanned"}`}>
          {unsorted.filter((s) => s.release === release).map((s) => <StoryCard key={s.key} story={s} selected={selected === s.key} onOpen={onOpen} />)}
        </div>
      )}
      {columns.map(({ activity, step }) => (
        <div key={(step?.key ?? activity.key) + release} className={`flex min-h-[64px] flex-col gap-1.5 pt-2 ${line}`} data-testid={step ? `cell-${step.key}-${release || "unplanned"}` : undefined}>
          {step && stories.filter((s) => s.step_keys.includes(step.key) && s.release === release).map((s) => <StoryCard key={s.key} story={s} selected={selected === s.key} onOpen={onOpen} />)}
          {step && <AddInline label="ストーリー" testId={`add-story-${step.key}-${release || "unplanned"}`} onAdd={(t) => onAdd(step.key, t)} subtle />}
        </div>
      ))}
      <div className={line} />
    </>
  );
}

export function StoryCard({ story: s, selected, onOpen }: { story: Story; selected?: boolean; onOpen: (key: string) => void }) {
  const m = STATUS_META[s.status];
  return (
    <button onClick={() => onOpen(s.key)} data-testid={`story-${s.key}`} data-status={s.status} title={m.hint}
      className={`w-[200px] rounded-md border px-2 py-1.5 text-left text-[12.5px] shadow-sm ${m.card} ${selected ? "ring-2 ring-accent" : "hover:brightness-110"}`}>
      <div className="mb-0.5 flex items-center gap-1.5 text-[10.5px] text-faint">
        <span className={`h-1.5 w-1.5 rounded-full ${m.dot}`} /><span className="font-mono">{s.key}</span><span className="flex-1" />
        <span title="受け入れ条件">条件 {s.criteria_count}</span><span title="つながったイシュー">· イシュー {s.issues.length}</span>
      </div>
      <div className="line-clamp-3 text-fg">{s.title}</div>
      {s.body && <div className="mt-0.5 flex items-start gap-1 text-[11px] text-faint" data-testid="story-body-hint"><FileText size={10} className="mt-0.5 shrink-0" /><span className="line-clamp-1">{s.body}</span></div>}
    </button>
  );
}

// ---- small editors ----

function AddInline({ label, onAdd, testId, placeholder, className = "", subtle, compact, icon }: {
  label: string; onAdd: (v: string) => void; testId?: string; placeholder?: string; className?: string; subtle?: boolean; compact?: boolean; icon?: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [v, setV] = useState("");
  if (!editing) {
    return (
      <button data-testid={testId} onClick={(e) => (e.stopPropagation(), setEditing(true))} title={icon ? "手順を追加" : undefined}
        className={`flex items-center gap-1 rounded-md text-[12px] ${compact ? "shrink-0 p-0.5 text-white/80 hover:bg-white/20" : subtle ? "px-1.5 py-0.5 text-faint opacity-0 hover:bg-hover hover:text-fg focus:opacity-100 [div:hover>&]:opacity-100" : "border border-dashed border-line px-2 py-1.5 text-muted hover:bg-hover hover:text-fg"} ${className}`}>
        <Plus size={12} />{label && <span>{label}</span>}
      </button>
    );
  }
  const submit = () => {
    const t = v.trim();
    if (t) onAdd(t);
    setV("");
    setEditing(false);
  };
  return (
    <input autoFocus value={v} placeholder={placeholder ?? `${label || "手順"}の名前`} data-testid={testId ? `${testId}-input` : undefined}
      onChange={(e) => setV(e.target.value)} onBlur={submit} onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => { if (e.key === "Enter" && !e.nativeEvent.isComposing) submit(); if (e.key === "Escape") (setV(""), setEditing(false)); }}
      className={`w-full min-w-[120px] rounded-md border border-accent bg-bg px-2 py-1 text-[12.5px] text-fg outline-none ${className}`} />
  );
}

function InlineText({ value, onSave, className = "", inputClass = "", multiline, placeholder, testId }: {
  value: string; onSave: (v: string) => void; className?: string; inputClass?: string; multiline?: boolean; placeholder?: string; testId?: string;
}) {
  const [editing, setEditing] = useState(false);
  const [v, setV] = useState(value);
  if (!editing) {
    return (
      <span data-testid={testId} onClick={(e) => (e.stopPropagation(), setV(value), setEditing(true))} title="クリックで編集"
        className={`cursor-text whitespace-pre-wrap ${value ? "" : "text-faint"} ${className}`}>{value || placeholder || "（未入力）"}</span>
    );
  }
  const save = () => {
    setEditing(false);
    if (v !== value) onSave(v);
  };
  const common = {
    autoFocus: true, value: v, "data-testid": testId ? `${testId}-input` : undefined,
    onChange: (e: { target: { value: string } }) => setV(e.target.value), onBlur: save,
    onClick: (e: { stopPropagation: () => void }) => e.stopPropagation(),
    className: `w-full rounded border border-accent bg-bg px-1.5 py-0.5 text-[12.5px] text-fg outline-none ${inputClass}`,
  };
  return multiline
    ? <textarea {...common} rows={2} onKeyDown={(e) => e.key === "Escape" && (setV(value), setEditing(false))} />
    : <input {...common} onKeyDown={(e) => { if (e.key === "Enter" && !e.nativeEvent.isComposing) save(); if (e.key === "Escape") (setV(value), setEditing(false)); }} />;
}

// ---- S16 story detail ----

function StoryPeek({ projectKey, storyKey, map, quick, draftOf, onClose }: { projectKey: string; storyKey: string; map: StoryMap; quick: boolean; draftOf: string | null; onClose: () => void }) {
  const { data: st, error } = useStory(projectKey, storyKey);
  const ops = useStoryMapOps(projectKey);
  const navigate = useNavigate();
  const [err, run] = useErr();
  const [issueKey, setIssueKey] = useState("");
  const [more, setMoreFields] = useState(false);
  const moreFields = more || !quick; // structured: the story template is always shown
  if (error) return <div className="p-6 text-muted">{(error as Error).message}</div>;
  if (!st) return <div className="p-6 text-faint">読み込み中...</div>;
  const steps = map.activities.flatMap((a) => a.steps.map((s) => ({ ...s, label: `${a.name} › ${s.name}` })));
  const persona = map.personas.find((p) => p.key === st.persona_key);
  const criteria = st.criteria ?? [];
  const save = (body: Parameters<typeof ops.updateStory>[1]) => run(ops.updateStory(st.key, body));
  const move = (c: Criterion, dir: -1 | 1) => {
    const i = criteria.findIndex((x) => x.key === c.key);
    const o = criteria[i + dir];
    if (!o) return;
    run(Promise.all([ops.updateCriterion(c.key, { position: o.position }), ops.updateCriterion(o.key, { position: c.position })]));
  };
  return (
    <div className="flex h-full flex-col" data-testid="story-peek" data-story-key={st.key}>
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-line px-4 text-[12.5px] text-muted">
        <span className="font-mono text-fg">{st.key}</span>
        <span className={`flex items-center gap-1 rounded px-1.5 text-[11.5px] ${STATUS_META[st.status].card}`} data-testid="story-status">
          <span className={`h-1.5 w-1.5 rounded-full ${STATUS_META[st.status].dot}`} />{STATUS_META[st.status].label}
        </span>
        <span className="flex-1" />
        <button title="ストーリーをアーカイブ" onClick={() => window.confirm(`${st.key} をアーカイブしますか？（キーは使い回しません）`) && run(ops.archiveStory(st.key).then(onClose))}
          className="rounded-md p-1.5 hover:bg-hover hover:text-fg"><Trash2 size={14} /></button>
        <button title="閉じる" onClick={onClose} className="rounded-md p-1.5 hover:bg-hover hover:text-fg"><X size={16} /></button>
      </div>
      {err && <div className="border-b border-line bg-[#e5484d14] px-4 py-1.5 text-[12px] text-danger">{err}</div>}
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4 text-[13px]">
        <InlineText value={st.title} onSave={(title) => save({ title })} className="block text-[17px] font-semibold" testId="story-title" />

        {(quick || st.body) && <StoryBody value={st.body} onSave={(body) => save({ body })} />}

        <div className="mt-3 grid grid-cols-[84px_1fr] items-start gap-x-3 gap-y-2">
          {(!quick || map.personas.length > 0 || st.persona_key) && (
            <>
              <Label>ペルソナ</Label>
              <select value={st.persona_key ?? ""} onChange={(e) => save({ persona_key: e.target.value || null })} data-testid="story-persona"
                className="w-fit rounded-md border border-line bg-bg px-1.5 py-0.5 text-[12.5px] outline-none">
                <option value="">（なし）</option>
                {map.personas.map((p) => <option key={p.key} value={p.key}>{p.name}</option>)}
              </select>
            </>
          )}
          {(moreFields || st.i_want) && (
            <>
              <Label>{persona ? `${persona.name}として` : "したいこと"}</Label>
              <InlineText value={st.i_want} onSave={(i_want) => save({ i_want })} multiline placeholder="〇〇したい" testId="story-i-want" />
            </>
          )}
          {(moreFields || st.so_that) && (
            <>
              <Label>そうすれば</Label>
              <InlineText value={st.so_that} onSave={(so_that) => save({ so_that })} multiline placeholder="〇〇できる" testId="story-so-that" />
            </>
          )}
          <Label>リリース</Label>
          <InlineText value={st.release} onSave={(release) => save({ release })} placeholder="未定（例: R1）" testId="story-release" />
          <Label>手順</Label>
          <div className="flex flex-wrap items-center gap-1" data-testid="story-steps">
            {st.step_keys.map((k) => (
              <span key={k} className="flex items-center gap-1 rounded border border-[#e6d88a] bg-[#fff6c9] px-1.5 py-0.5 text-[12px] text-[#3b3520]">
                {steps.find((s) => s.key === k)?.label ?? k}
                <button title="外す" onClick={() => save({ step_keys: st.step_keys.filter((x) => x !== k) })} className="text-[#8a8060] hover:text-[#3b3520]"><X size={11} /></button>
              </span>
            ))}
            <select value="" onChange={(e) => e.target.value && save({ step_keys: [...st.step_keys, e.target.value] })} data-testid="story-add-step"
              className="rounded-md border border-dashed border-line bg-bg px-1 py-0.5 text-[12px] text-muted outline-none">
              <option value="">＋ 手順につなぐ</option>
              {steps.filter((s) => !st.step_keys.includes(s.key)).map((s) => <option key={s.key} value={s.key}>{s.label}</option>)}
            </select>
            {st.step_keys.length === 0 && (quick
              ? <span className="text-[11.5px] text-faint">未整理（手順を選ぶとマップの列に移る）</span>
              : <span className="text-[11.5px] text-[#e0823d]" data-testid="story-no-step">手順が未設定（抜け）</span>)}
          </div>
        </div>
        {!(moreFields || (st.i_want && st.so_that)) && (
          <button onClick={() => setMoreFields(true)} data-testid="story-more-fields" className="mt-2 flex items-center gap-1 text-[12px] text-faint hover:text-fg">
            <Plus size={12} />項目を足す（したいこと・そうすれば）
          </button>
        )}

        <Section title="受け入れ条件" count={criteria.length} hint={quick ? "1 行で書ける。前提・操作は「詳しく」から" : "前提・操作・結果の 3 つで書く。イシューの「受入基準」に写せる"}>
          <ol className="space-y-2" data-testid="criteria">
            {criteria.map((c, i) => (
              <li key={c.key} className="group rounded-md border border-line px-3 py-2" data-testid={`criterion-${c.key}`}>
                <div className="mb-1 flex items-center gap-2 text-[11px] text-faint">
                  <span className="font-mono">{c.key}</span><span className="flex-1" />
                  <button title="上へ" disabled={i === 0} onClick={() => move(c, -1)} className="rounded p-0.5 enabled:hover:bg-hover disabled:opacity-30"><ArrowUp size={12} /></button>
                  <button title="下へ" disabled={i === criteria.length - 1} onClick={() => move(c, 1)} className="rounded p-0.5 enabled:hover:bg-hover disabled:opacity-30"><ArrowDown size={12} /></button>
                  <button title="アーカイブ" onClick={() => run(ops.archiveCriterion(c.key))} className="rounded p-0.5 hover:bg-hover"><Trash2 size={12} /></button>
                </div>
                <CriterionLines c={c} full={!quick} onSave={(body) => run(ops.updateCriterion(c.key, body))} />
              </li>
            ))}
          </ol>
          <NewCriterion full={!quick} onAdd={(body) => run(ops.createCriterion(st.key, body))} />
        </Section>

        {draftOf ? (
          <Section title="開発タスク（イシュー）" count={0}>
            <p className="text-[12px] text-faint" data-testid="draft-no-issues">下書きではイシューをつなぎません。{draftOf} に取り込んだあと、{draftOf} のマップでつなぎます（取り込みでは今あるつながりはそのまま残ります）。</p>
          </Section>
        ) : <Section title="開発タスク（イシュー）" count={st.issues.length} hint="このストーリーを満たすためのイシュー。同じプロジェクトのものだけ">
          <div className="rounded-md border border-line" data-testid="story-issues">
            {st.issues.map((is) => (
              <div key={is.key} className="group flex items-center gap-2 border-b border-line px-3 py-1.5 last:border-b-0">
                <StateIcon state={{ category: is.state_category as never, color: "", name: is.state_name }} />
                <button onClick={() => navigate({ to: "/p/$key/issues", params: { key: projectKey }, search: { peek: is.key } })} className="flex min-w-0 flex-1 gap-2 text-left hover:underline">
                  <span className="shrink-0 font-mono text-[12px] text-faint">{is.key}</span><span className="truncate">{is.title}</span>
                </button>
                <span className="shrink-0 text-[11.5px] text-faint">{is.state_name}</span>
                <button title="つながりを外す" onClick={() => run(ops.unlinkIssue(st.key, is.key))} className="hidden rounded p-0.5 text-faint hover:bg-hover hover:text-fg group-hover:block"><X size={12} /></button>
              </div>
            ))}
            {st.issues.length === 0 && <div className="px-3 py-2 text-[12px] text-faint">つながったイシューはありません</div>}
          </div>
          <form className="mt-2 flex gap-2" onSubmit={(e) => {
            e.preventDefault();
            const k = issueKey.trim().toUpperCase();
            if (!k) return;
            run(ops.linkIssue(st.key, k.includes("-") ? k : `${projectKey}-${k}`).then(() => setIssueKey("")));
          }}>
            <input value={issueKey} onChange={(e) => setIssueKey(e.target.value)} placeholder={`イシューキー（例: ${projectKey}-12）`} data-testid="link-issue-input"
              className="w-56 rounded-md border border-line bg-bg px-2 py-1 font-mono text-[12.5px] outline-none focus:border-accent" />
            <button className="rounded-md border border-line px-2.5 py-1 text-[12px] hover:bg-hover">つなぐ</button>
          </form>
        </Section>}

        <Section title="変更履歴" count={st.history?.length ?? 0}>
          <ul className="space-y-1 text-[12px] text-muted" data-testid="story-history">
            {(st.history ?? []).map((h, i) => (
              <li key={i} className="flex gap-2">
                <span className="w-16 shrink-0 text-faint">{relativeTime(h.created_at)}</span>
                <span className="shrink-0">{h.actor.display_name}</span>
                <span className="shrink-0 font-mono text-faint">{h.entity_key.split("-").slice(1).join("-")}</span>
                <span className="truncate">{describeChange(h.action, h.detail)}</span>
              </li>
            ))}
          </ul>
        </Section>
      </div>
    </div>
  );
}

const ACTION_LABEL: Record<string, string> = { created: "作成", updated: "変更", archived: "アーカイブ", linked: "イシューをつないだ", unlinked: "イシューを外した" };
const FIELD_LABEL: Record<string, string> = { title: "タイトル", body: "本文", i_want: "したいこと", so_that: "そうすれば", release: "リリース", step_keys: "手順", persona_key: "ペルソナ", position: "並び", given: "Given", when: "When", then: "Then" };

function describeChange(action: string, detail: Record<string, unknown>): string {
  const a = ACTION_LABEL[action] ?? action;
  if (action === "linked" || action === "unlinked") return `${a}: ${String(detail.issue_key ?? "")}`;
  if (action === "updated") return `${a}: ${Object.keys(detail).map((k) => FIELD_LABEL[k] ?? k).join("・")}`;
  return a;
}

function Label({ children }: { children: ReactNode }) {
  return <span className="pt-0.5 text-[12px] text-faint">{children}</span>;
}

function Section({ title, count, hint, children }: { title: string; count: number; hint?: string; children: ReactNode }) {
  return (
    <section className="mt-6">
      <h3 className="mb-2 flex items-baseline gap-2 text-[13px] font-medium"><span className="shrink-0 whitespace-nowrap">{title}</span><span className="text-[12px] font-normal text-faint">{count}</span>
        {hint && <span className="truncate text-[11.5px] font-normal text-faint">{hint}</span>}</h3>
      {children}
    </section>
  );
}

function GWT({ label, value, onSave }: { label: string; value: string; onSave: (v: string) => void }) {
  return (
    <div className="flex gap-2 text-[12.5px]">
      <span className="w-11 shrink-0 text-[11px] leading-5 text-accent">{label}</span>
      <InlineText value={value} onSave={onSave} className="min-w-0 flex-1" placeholder="—" />
    </div>
  );
}

/** D3: an existing criterion is one line until it has a precondition or an action. */
function CriterionLines({ c, full, onSave }: { c: Criterion; full: boolean; onSave: (body: { given?: string; when?: string; then?: string }) => void }) {
  const [detail, setDetail] = useState(false);
  if (full || detail || c.given || c.when) {
    return (
      <>
        <GWT label="前提" value={c.given} onSave={(given) => onSave({ given })} />
        <GWT label="操作" value={c.when} onSave={(when) => onSave({ when })} />
        <GWT label="結果" value={c.then} onSave={(then) => onSave({ then })} />
      </>
    );
  }
  return (
    <div className="flex items-start gap-2 text-[12.5px]">
      <span className="mt-0.5 shrink-0 text-faint">☐</span>
      <InlineText value={c.then} onSave={(then) => onSave({ then })} className="min-w-0 flex-1" />
      <button onClick={() => setDetail(true)} className="shrink-0 text-[11px] text-faint hover:text-fg">詳しく</button>
    </div>
  );
}

/** D3: one line is enough; 詳しく opens the precondition and the action. */
function NewCriterion({ full, onAdd }: { full: boolean; onAdd: (body: { given: string; when: string; then: string }) => void }) {
  const [v, setV] = useState({ given: "", when: "", then: "" });
  const [open, setDetail] = useState(false);
  const detail = open || full; // structured: always the three fields
  const field = (k: keyof typeof v, label: string, ph: string) => (
    <label className="flex items-center gap-2">
      <span className="w-11 shrink-0 text-[11px] text-accent">{label}</span>
      <input value={v[k]} onChange={(e) => setV({ ...v, [k]: e.target.value })} placeholder={ph} data-testid={`new-criterion-${k}`}
        className="min-w-0 flex-1 rounded border border-line bg-bg px-1.5 py-0.5 text-[12.5px] outline-none focus:border-accent" />
    </label>
  );
  return (
    <form className="mt-2 space-y-1 rounded-md border border-dashed border-line px-3 py-2" onSubmit={(e) => {
      e.preventDefault();
      if (!v.then.trim()) return;
      onAdd(v);
      setV({ given: "", when: "", then: "" });
    }}>
      {detail && field("given", "前提", "例: マイタスクを開いた")}
      {detail && field("when", "操作", "例: 何も操作しない")}
      <div className="flex items-center gap-2">
        {detail ? <span className="w-11 shrink-0 text-[11px] text-accent">結果</span> : <span className="shrink-0 text-faint">☐</span>}
        <input value={v.then} onChange={(e) => setV({ ...v, then: e.target.value })} placeholder="こうなっていればいい（例: 完了したイシューは表示されない）" data-testid="new-criterion-then"
          className="min-w-0 flex-1 rounded border border-line bg-bg px-1.5 py-0.5 text-[12.5px] outline-none focus:border-accent" />
        {!detail && <button type="button" onClick={() => setDetail(true)} data-testid="new-criterion-detail" className="shrink-0 text-[11.5px] text-faint hover:text-fg">詳しく</button>}
        <button data-testid="add-criterion" disabled={!v.then.trim()} className="shrink-0 rounded-md bg-accent px-2.5 py-0.5 text-[12px] text-white disabled:opacity-40">追加</button>
      </div>
    </form>
  );
}

/** D2: free Markdown under the title. */
function StoryBody({ value, onSave }: { value: string; onSave: (v: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [v, setV] = useState(value);
  useEffect(() => setV(value), [value]);
  if (editing) {
    const done = () => { setEditing(false); if (v !== value) onSave(v); };
    return (
      <div className="mt-2">
        <textarea autoFocus value={v} onChange={(e) => setV(e.target.value)} rows={Math.max(4, v.split("\n").length + 1)} data-testid="story-body-input"
          onBlur={done} onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) done(); if (e.key === "Escape") (setV(value), setEditing(false)); }}
          className="w-full rounded-md border border-accent bg-bg p-2.5 font-mono text-[12.5px] leading-relaxed outline-none" placeholder="自由に書く（Markdown）。背景・思いつき・メモなど" />
      </div>
    );
  }
  return (
    <div onClick={() => setEditing(true)} data-testid="story-body" className="markdown mt-2 min-h-[36px] cursor-text rounded-md px-0.5 text-[13px] leading-relaxed hover:bg-hover/50">
      {value ? <Markdown>{value}</Markdown> : <span className="text-faint">本文を書く（自由に。Markdown）</span>}
    </div>
  );
}

// ---- personas and jobs panel ----

function PersonaPanel({ map, projectKey, onClose }: { map: StoryMap; projectKey: string; onClose: () => void }) {
  const ops = useStoryMapOps(projectKey);
  const [err, run] = useErr();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<"human" | "agent">("human");
  return (
    <aside className="flex w-[330px] shrink-0 flex-col border-r border-line bg-sidebar/50" data-testid="persona-panel">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-line px-4 text-[13px] font-medium">
        <Users size={14} className="text-muted" />ペルソナとジョブ<span className="flex-1" />
        <button title="閉じる" onClick={onClose} className="rounded p-1 text-muted hover:bg-hover"><X size={14} /></button>
      </div>
      {err && <div className="px-4 py-1 text-[12px] text-danger">{err}</div>}
      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-3 text-[12.5px]">
        <section>
          <h3 className="mb-1.5 text-[12px] font-medium text-muted">ペルソナ（誰が使うか）</h3>
          <div className="space-y-1.5">
            {map.personas.map((p) => (
              <div key={p.key} className="group rounded-md border border-line bg-bg px-2.5 py-1.5" data-testid={`persona-${p.key}`}>
                <div className="flex items-center gap-1.5">
                  {p.kind === "agent" ? <Bot size={13} className="text-muted" /> : <User size={13} className="text-muted" />}
                  <InlineText value={p.name} onSave={(n) => run(ops.updatePersona(p.key, { name: n }))} className="flex-1 font-medium" />
                  <span className="font-mono text-[10.5px] text-faint">{p.key.split("-")[1]}</span>
                  <button title="アーカイブ" onClick={() => window.confirm(`ペルソナ「${p.name}」をアーカイブしますか？`) && run(ops.archivePersona(p.key))}
                    className="hidden rounded p-0.5 text-faint hover:bg-hover group-hover:block"><Trash2 size={11} /></button>
                </div>
                <InlineText value={p.description} onSave={(d) => run(ops.updatePersona(p.key, { description: d }))} multiline placeholder="説明" className="mt-0.5 block text-[12px] text-muted" />
              </div>
            ))}
          </div>
          <form className="mt-2 flex gap-1.5" onSubmit={(e) => {
            e.preventDefault();
            if (!name.trim()) return;
            run(ops.createPersona({ name, kind }).then(() => setName("")));
          }}>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="新しいペルソナ" data-testid="new-persona-name"
              className="min-w-0 flex-1 rounded-md border border-line bg-bg px-2 py-1 outline-none focus:border-accent" />
            <select value={kind} onChange={(e) => setKind(e.target.value as "human" | "agent")} className="rounded-md border border-line bg-bg px-1 text-[12px] outline-none">
              <option value="human">人</option><option value="agent">AI</option>
            </select>
            <button data-testid="add-persona" className="rounded-md border border-line px-2 hover:bg-hover"><Plus size={13} /></button>
          </form>
        </section>
        <section>
          <h3 className="mb-1.5 text-[12px] font-medium text-muted">ジョブ（どんな状況で何をしたいか）</h3>
          <div className="space-y-1.5">
            {map.jobs.map((j) => <JobCard key={j.key} job={j} map={map} projectKey={projectKey} run={run} />)}
          </div>
          <NewJob map={map} onAdd={(body) => run(ops.createJob(body))} />
        </section>
      </div>
    </aside>
  );
}

function JobCard({ job: j, map, projectKey, run }: { job: Job; map: StoryMap; projectKey: string; run: (p: Promise<unknown>) => void }) {
  const ops = useStoryMapOps(projectKey);
  const persona = map.personas.find((p) => p.key === j.persona_key);
  const toggle = (a: string) => run(ops.updateJob(j.key, { activity_keys: j.activity_keys.includes(a) ? j.activity_keys.filter((x) => x !== a) : [...j.activity_keys, a] }));
  return (
    <div className="group rounded-md border border-line bg-bg px-2.5 py-1.5" data-testid={`job-${j.key}`}>
      <div className="mb-0.5 flex items-center gap-1.5 text-[10.5px] text-faint">
        <span className="font-mono">{j.key.split("-")[1]}</span>{persona && <span>{persona.name}</span>}<span className="flex-1" />
        <button title="アーカイブ" onClick={() => run(ops.archiveJob(j.key))} className="hidden rounded p-0.5 hover:bg-hover group-hover:block"><Trash2 size={11} /></button>
      </div>
      <div className="space-y-0.5">
        <JobLine label="状況" value={j.situation} onSave={(situation) => run(ops.updateJob(j.key, { situation }))} />
        <JobLine label="したい" value={j.motivation} onSave={(motivation) => run(ops.updateJob(j.key, { motivation }))} />
        <JobLine label="そうすれば" value={j.outcome} onSave={(outcome) => run(ops.updateJob(j.key, { outcome }))} />
      </div>
      <div className="mt-1 flex flex-wrap gap-1">
        {map.activities.map((a) => (
          <button key={a.key} onClick={() => toggle(a.key)} title="この活動につなぐ / 外す"
            className={`rounded px-1.5 py-0.5 text-[11px] ${j.activity_keys.includes(a.key) ? "bg-accent text-white" : "border border-dashed border-line text-faint hover:text-fg"}`}>{a.name}</button>
        ))}
      </div>
    </div>
  );
}

function JobLine({ label, value, onSave }: { label: string; value: string; onSave: (v: string) => void }) {
  return (
    <div className="flex gap-1.5">
      <span className="w-14 shrink-0 text-[11px] leading-5 text-faint">{label}</span>
      <InlineText value={value} onSave={onSave} className="min-w-0 flex-1" placeholder="—" />
    </div>
  );
}

function NewJob({ map, onAdd }: { map: StoryMap; onAdd: (body: { persona_key: string | null; situation: string; motivation: string; outcome: string }) => void }) {
  const [v, setV] = useState({ persona_key: "", situation: "", motivation: "", outcome: "" });
  const input = (k: "situation" | "motivation" | "outcome", ph: string) => (
    <input value={v[k]} onChange={(e) => setV({ ...v, [k]: e.target.value })} placeholder={ph} data-testid={`new-job-${k}`}
      className="w-full rounded border border-line bg-bg px-1.5 py-0.5 outline-none focus:border-accent" />
  );
  return (
    <form className="mt-2 space-y-1 rounded-md border border-dashed border-line px-2.5 py-2" onSubmit={(e) => {
      e.preventDefault();
      if (!v.motivation.trim()) return;
      onAdd({ ...v, persona_key: v.persona_key || null });
      setV({ persona_key: v.persona_key, situation: "", motivation: "", outcome: "" });
    }}>
      <select value={v.persona_key} onChange={(e) => setV({ ...v, persona_key: e.target.value })} className="w-full rounded border border-line bg-bg px-1 py-0.5 text-[12px] outline-none">
        <option value="">ペルソナ（なし）</option>
        {map.personas.map((p) => <option key={p.key} value={p.key}>{p.name}</option>)}
      </select>
      {input("situation", "状況（〇〇なとき）")}
      {input("motivation", "したいこと（必須）")}
      {input("outcome", "そうすれば（〇〇できる）")}
      <div className="flex justify-end"><button data-testid="add-job" disabled={!v.motivation.trim()} className="rounded-md bg-accent px-2.5 py-0.5 text-[12px] text-white disabled:opacity-40">ジョブを追加</button></div>
    </form>
  );
}
