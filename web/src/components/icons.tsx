import { Bot, User as UserIcon } from "lucide-react";
import type { IssueType, State, UserRef } from "../api/client";
import { PRIORITY_META, TYPE_META } from "../lib/meta";

export function TypeIcon({ type, size = 14 }: { type: IssueType; size?: number }) {
  const m = TYPE_META[type];
  if (type === "qa") {
    return (
      <span title={m.label} className="inline-flex shrink-0 items-center justify-center rounded-full font-bold text-white"
        style={{ width: size, height: size, background: m.color, fontSize: size * 0.7 }}>?</span>
    );
  }
  const radius = type === "epic" ? 3 : type === "story" ? size / 2 : type === "feature" ? 2 : 1;
  return (
    <span title={m.label} className="inline-flex shrink-0 items-center justify-center font-bold text-white"
      style={{ width: size, height: size, background: m.color, borderRadius: radius, fontSize: size * 0.62 }}>
      {m.short}
    </span>
  );
}

/** Linear-style circular state icon. */
export function StateIcon({ state, size = 14 }: { state: Pick<State, "category" | "color" | "name">; size?: number }) {
  const r = size / 2 - 1.25;
  const c = size / 2;
  const color = state.color || "#9aa0b0";
  const common = { width: size, height: size, viewBox: `0 0 ${size} ${size}` };
  let inner = null;
  switch (state.category) {
    case "backlog":
      return (
        <svg {...common} aria-label={state.name}>
          <circle cx={c} cy={c} r={r} fill="none" stroke={color} strokeWidth={1.5} strokeDasharray="2 2" />
        </svg>
      );
    case "started":
      inner = <path d={`M${c} ${c} L${c} ${c - r + 2} A${r - 2} ${r - 2} 0 0 1 ${c} ${c + r - 2} Z`} fill={color} />;
      break;
    case "completed":
      return (
        <svg {...common} aria-label={state.name}>
          <circle cx={c} cy={c} r={r + 0.75} fill={color} />
          <path d={`M${c - r / 2} ${c} l${r / 3} ${r / 3} l${r / 1.6} -${r / 1.6}`} stroke="white" strokeWidth={1.5} fill="none" strokeLinecap="round" />
        </svg>
      );
    case "canceled":
      return (
        <svg {...common} aria-label={state.name}>
          <circle cx={c} cy={c} r={r + 0.75} fill={color} />
          <path d={`M${c - r / 2.4} ${c - r / 2.4} l${r / 1.2} ${r / 1.2} M${c + r / 2.4} ${c - r / 2.4} l-${r / 1.2} ${r / 1.2}`} stroke="white" strokeWidth={1.5} strokeLinecap="round" />
        </svg>
      );
  }
  return (
    <svg {...common} aria-label={state.name}>
      <circle cx={c} cy={c} r={r} fill="none" stroke={color} strokeWidth={1.5} />
      {inner}
    </svg>
  );
}

export function PriorityIcon({ priority, size = 14 }: { priority: number; size?: number }) {
  const label = PRIORITY_META[priority]?.label ?? "";
  if (priority === 1) {
    return (
      <span title={label} className="inline-flex shrink-0 items-center justify-center rounded-[3px] bg-[#e5484d] font-bold text-white"
        style={{ width: size, height: size, fontSize: size * 0.75 }}>!</span>
    );
  }
  const bars = priority === 0 ? 0 : 5 - priority; // high=3, medium=2, low=1
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-label={label}>
      {[0, 1, 2].map((i) => (
        <rect key={i} x={1.5 + i * 5} y={10 - i * 4} width={3} height={4 + i * 4} rx={1}
          fill={priority === 0 ? "none" : i < bars ? "var(--text-muted)" : "var(--border-strong)"}
          stroke={priority === 0 ? "var(--text-faint)" : "none"} strokeDasharray={priority === 0 ? "1.5 1.5" : undefined} />
      ))}
    </svg>
  );
}

export function Avatar({ user, size = 18 }: { user: UserRef | null | undefined; size?: number }) {
  if (!user) {
    return (
      <span title="未割り当て" className="inline-flex shrink-0 items-center justify-center rounded-full border border-dashed border-line-strong text-faint"
        style={{ width: size, height: size }}>
        <UserIcon size={size * 0.6} />
      </span>
    );
  }
  const isAgent = user.kind === "agent";
  return (
    <span title={`${user.display_name}${isAgent ? "（エージェント）" : ""}`}
      className={`inline-flex shrink-0 items-center justify-center font-semibold text-white ${isAgent ? "rounded-[5px]" : "rounded-full"}`}
      style={{ width: size, height: size, background: user.avatar_color || "#6b7285", fontSize: size * 0.5 }}>
      {isAgent ? <Bot size={size * 0.68} /> : user.display_name.slice(0, 1).toUpperCase()}
    </span>
  );
}

export function ProgressPie({ total, done, size = 14 }: { total: number; done: number; size?: number }) {
  if (total === 0) return null;
  const ratio = done / total;
  const r = size / 2 - 1;
  const c = size / 2;
  const angle = ratio * 2 * Math.PI;
  const x = c + r * Math.sin(angle);
  const y = c - r * Math.cos(angle);
  return (
    <span className="inline-flex items-center gap-1 text-[11px] text-muted" title={`子孫 ${done}/${total} 完了`}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        <circle cx={c} cy={c} r={r} fill="none" stroke="var(--border-strong)" strokeWidth={1.5} />
        {ratio >= 1 ? (
          <circle cx={c} cy={c} r={r} fill="#3fae6a" />
        ) : ratio > 0 ? (
          <path d={`M${c} ${c} L${c} ${c - r} A${r} ${r} 0 ${ratio > 0.5 ? 1 : 0} 1 ${x} ${y} Z`} fill="#3fae6a" />
        ) : null}
      </svg>
      {done}/{total}
    </span>
  );
}
