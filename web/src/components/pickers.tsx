import { Circle, Sprout, Star } from "lucide-react";
import type { ReactNode } from "react";
import type { Issue, ProjectTier } from "../api/client";
import { useStates, useUpdateIssue, useUpdateProject, useUsers } from "../api/hooks";
import { PRIORITY_META, TIER_LABEL, TIERS } from "../lib/meta";
import { Avatar, PriorityIcon, StateIcon } from "./icons";
import { OptionList, Popover } from "./Menu";

const triggerCls = "flex items-center gap-1.5 rounded-md px-1 py-0.5 hover:bg-hover";

export function StatePicker({ issue, children }: { issue: Issue; children?: ReactNode }) {
  const { data: states = [] } = useStates(issue.project_key);
  const update = useUpdateIssue();
  const applies = issue.type === "qa" ? "qa" : "all";
  const options = states.filter((s) => s.applies_to === applies)
    .map((s) => ({ value: s.id, label: s.name, icon: <StateIcon state={s} /> }));
  return (
    <Popover trigger={() => children ?? <button className={triggerCls} title={issue.state.name}><StateIcon state={issue.state} /></button>}>
      {(close) => (
        <OptionList options={options} selected={[issue.state.id]}
          onSelect={(id) => (update.mutate({ key: issue.key, state_id: id }), close())} />
      )}
    </Popover>
  );
}

export function AssigneePicker({ issue, children }: { issue: Issue; children?: ReactNode }) {
  const { data: users = [] } = useUsers();
  const update = useUpdateIssue();
  const options = [
    { value: "", label: "未割り当て", icon: <Avatar user={null} size={16} /> },
    ...users.map((u) => ({ value: u.username, label: u.display_name, icon: <Avatar user={u} size={16} />, hint: u.kind === "agent" ? "agent" : undefined })),
  ];
  return (
    <Popover align="right" trigger={() => children ?? <button className={triggerCls}><Avatar user={issue.assignee} /></button>}>
      {(close) => (
        <OptionList options={options} selected={[issue.assignee?.username ?? ""]}
          onSelect={(v) => (update.mutate({ key: issue.key, assignee: v || null }), close())} />
      )}
    </Popover>
  );
}

export function PriorityPicker({ issue, children }: { issue: Issue; children?: ReactNode }) {
  const update = useUpdateIssue();
  const options = PRIORITY_META.map((p) => ({ value: p.value, label: p.label, icon: <PriorityIcon priority={p.value} /> }));
  return (
    <Popover trigger={() => children ?? <button className={triggerCls}><PriorityIcon priority={issue.priority} /></button>}>
      {(close) => (
        <OptionList options={options} selected={[issue.priority]}
          onSelect={(v) => (update.mutate({ key: issue.key, priority: v }), close())} />
      )}
    </Popover>
  );
}

/** ★ 注力 / ○ 通常 / 🌱 その他 */
export function TierIcon({ tier, size = 13 }: { tier: ProjectTier; size?: number }) {
  if (tier === "focus") return <Star size={size} className="shrink-0 fill-[#e0a82e] text-[#e0a82e]" />;
  if (tier === "side") return <Sprout size={size} className="shrink-0 text-[#6aa87a]" />;
  return <Circle size={size} className="shrink-0 text-faint" />;
}

/** Changes a project's tier. The trigger shows the current tier; `label` adds its name. */
export function TierPicker({ projectKey, tier, label, align }: { projectKey: string; tier: ProjectTier; label?: boolean; align?: "left" | "right" }) {
  const update = useUpdateProject(projectKey);
  const options = TIERS.map((t) => ({ value: t, label: TIER_LABEL[t], icon: <TierIcon tier={t} /> }));
  return (
    <Popover align={align} trigger={() => (
      <button data-testid={`tier-${projectKey}`} title={`ティア: ${TIER_LABEL[tier]}（クリックで変更）`}
        className="flex shrink-0 items-center gap-1 rounded-md px-1 py-0.5 text-[12px] text-muted hover:bg-hover hover:text-fg">
        <TierIcon tier={tier} />{label && TIER_LABEL[tier]}
      </button>
    )}>
      {(close) => (
        <OptionList options={options} selected={[tier]} onSelect={(t) => (update.mutate({ tier: t }), close())} />
      )}
    </Popover>
  );
}
