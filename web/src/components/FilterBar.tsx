import { Bot, ListFilter, Search, X } from "lucide-react";
import type { ReactNode } from "react";
import { useLabels, useStates, useUsers } from "../api/hooks";
import { toggle, type FilterState } from "../lib/filters";
import { ISSUE_TYPES, PRIORITY_META, TIER_LABEL, TIERS, TYPE_META } from "../lib/meta";
import { Avatar, PriorityIcon, StateIcon, TypeIcon } from "./icons";
import { OptionList, Popover } from "./Menu";
import { TierIcon } from "./pickers";

/** Filter controls. projectKey enables state and label filters. */
export function FilterBar({ value, onChange, projectKey, extra }: {
  value: FilterState;
  onChange: (f: FilterState) => void;
  projectKey?: string;
  extra?: ReactNode;
}) {
  const { data: users = [] } = useUsers();
  const { data: states = [] } = useStates(projectKey);
  const { data: labels = [] } = useLabels(projectKey);
  const set = (patch: Partial<FilterState>) => onChange({ ...value, ...patch });

  const chips: { key: string; label: string; clear: () => void }[] = [];
  if (value.types?.length) chips.push({ key: "types", label: `種別: ${value.types.map((t) => TYPE_META[t].label).join(", ")}`, clear: () => set({ types: undefined }) });
  if (value.states?.length) chips.push({ key: "states", label: `ステータス: ${value.states.map((id) => states.find((s) => s.id === id)?.name ?? id).join(", ")}`, clear: () => set({ states: undefined }) });
  if (value.assignees?.length) chips.push({
    key: "assignees",
    label: `担当: ${value.assignees.map((a) => (a === "me" ? "自分" : a === "none" ? "未割り当て" : users.find((u) => u.username === a)?.display_name ?? a)).join(", ")}`,
    clear: () => set({ assignees: undefined }),
  });
  if (value.assigneeKind) chips.push({ key: "kind", label: value.assigneeKind === "agent" ? "担当: エージェント" : "担当: 人間", clear: () => set({ assigneeKind: undefined }) });
  if (value.labels?.length) chips.push({ key: "labels", label: `ラベル: ${value.labels.join(", ")}`, clear: () => set({ labels: undefined }) });
  if (value.priorities?.length) chips.push({ key: "prio", label: `優先度: ${value.priorities.map((p) => PRIORITY_META[p].label).join(", ")}`, clear: () => set({ priorities: undefined }) });
  const focusOnly = value.tiers?.length === 1 && value.tiers[0] === "focus";
  if (value.tiers?.length && !focusOnly) chips.push({ key: "tiers", label: `ティア: ${value.tiers.map((t) => TIER_LABEL[t]).join(", ")}`, clear: () => set({ tiers: undefined }) });
  if (value.under) chips.push({ key: "under", label: `配下: ${value.under}`, clear: () => set({ under: undefined }) });

  return (
    <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-3 py-2">
      <div className="flex items-center gap-1 rounded-md border border-line bg-bg px-2 py-1 focus-within:border-accent">
        <Search size={13} className="text-faint" />
        <input id="filter-search" value={value.q ?? ""} placeholder="検索 ( / )" onChange={(e) => set({ q: e.target.value || undefined })}
          className="w-40 bg-transparent text-[13px] outline-none placeholder:text-faint" />
      </div>

      <Popover trigger={() => (
        <button className="flex items-center gap-1.5 rounded-md px-2 py-1 text-[13px] text-muted hover:bg-hover hover:text-fg">
          <ListFilter size={14} /> フィルタ
        </button>
      )}>
        {() => (
          <div className="flex gap-1">
            <FilterSection title="種別">
              <OptionList multi options={ISSUE_TYPES.map((t) => ({ value: t, label: TYPE_META[t].label, icon: <TypeIcon type={t} /> }))}
                selected={value.types ?? []} onSelect={(t) => set({ types: toggle(value.types, t) })} />
            </FilterSection>
            <FilterSection title="担当者">
              <OptionList multi
                options={[
                  { value: "me", label: "自分", icon: <Avatar user={null} size={16} /> },
                  { value: "none", label: "未割り当て", icon: <Avatar user={null} size={16} /> },
                  ...users.map((u) => ({ value: u.username, label: u.display_name, icon: <Avatar user={u} size={16} />, hint: u.kind === "agent" ? "agent" : undefined })),
                ]}
                selected={value.assignees ?? []} onSelect={(a) => set({ assignees: toggle(value.assignees, a) })} />
              <div className="mt-1 flex gap-1 border-t border-line pt-1">
                {(["agent", "human"] as const).map((k) => (
                  <button key={k} onClick={() => set({ assigneeKind: value.assigneeKind === k ? undefined : k })}
                    className={`flex flex-1 items-center justify-center gap-1 rounded-md px-2 py-1 text-[12px] ${value.assigneeKind === k ? "bg-accent text-white" : "hover:bg-hover"}`}>
                    {k === "agent" ? <><Bot size={12} />エージェント</> : "人間"}
                  </button>
                ))}
              </div>
            </FilterSection>
            {projectKey && (
              <FilterSection title="ステータス">
                <OptionList multi options={states.map((s) => ({ value: s.id, label: s.name, icon: <StateIcon state={s} />, hint: s.applies_to === "qa" ? "QA" : undefined }))}
                  selected={value.states ?? []} onSelect={(id) => set({ states: toggle(value.states, id) })} />
              </FilterSection>
            )}
            {!projectKey && (
              <FilterSection title="プロジェクトのティア">
                <OptionList multi options={TIERS.map((t) => ({ value: t, label: TIER_LABEL[t], icon: <TierIcon tier={t} /> }))}
                  selected={value.tiers ?? []} onSelect={(t) => set({ tiers: toggle(value.tiers, t) })} />
              </FilterSection>
            )}
            <FilterSection title="優先度 / ラベル">
              <OptionList multi options={PRIORITY_META.map((p) => ({ value: p.value, label: p.label, icon: <PriorityIcon priority={p.value} /> }))}
                selected={value.priorities ?? []} onSelect={(p) => set({ priorities: toggle(value.priorities, p) })} />
              {labels.length > 0 && (
                <div className="mt-1 border-t border-line pt-1">
                  <OptionList multi options={labels.map((l) => ({ value: l.name, label: l.name, icon: <span className="h-2 w-2 rounded-full" style={{ background: l.color }} /> }))}
                    selected={value.labels ?? []} onSelect={(n) => set({ labels: toggle(value.labels, n) })} />
                </div>
              )}
            </FilterSection>
          </div>
        )}
      </Popover>

      <label className="flex cursor-pointer items-center gap-1.5 rounded-md px-2 py-1 text-[13px] text-muted hover:bg-hover">
        <input type="checkbox" checked={!!value.hideClosed} onChange={(e) => set({ hideClosed: e.target.checked || undefined })} className="accent-[var(--accent)]" />
        完了を隠す
      </label>
      {!projectKey && (
        <label data-testid="focus-only" title="ティアが「注力」のプロジェクトだけを表示" className="flex cursor-pointer items-center gap-1.5 rounded-md px-2 py-1 text-[13px] text-muted hover:bg-hover">
          <input type="checkbox" checked={focusOnly} onChange={(e) => set({ tiers: e.target.checked ? ["focus"] : undefined })} className="accent-[var(--accent)]" />
          <TierIcon tier="focus" size={12} />注力のみ
        </label>
      )}
      {extra}

      {chips.map((c) => (
        <span key={c.key} className="flex items-center gap-1 rounded-md border border-line bg-elev py-0.5 pl-2 pr-1 text-[12px]">
          {c.label}
          <button onClick={c.clear} className="rounded p-0.5 text-faint hover:bg-hover hover:text-fg"><X size={12} /></button>
        </span>
      ))}
      {chips.length > 0 && (
        <button onClick={() => onChange({ q: value.q, hideClosed: value.hideClosed, tiers: focusOnly ? value.tiers : undefined })} className="text-[12px] text-faint hover:text-fg">クリア</button>
      )}
    </div>
  );
}

function FilterSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col">
      <div className="px-2 pb-1 pt-1 text-[11px] font-medium uppercase tracking-wide text-faint">{title}</div>
      {children}
    </div>
  );
}
