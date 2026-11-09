import { AlertTriangle, ArrowRight, Flag, GitBranch, GitCommit, Scale } from "lucide-react";
import type { Checkpoint } from "../api/client";
import { relativeTime } from "../lib/meta";
import { Avatar } from "./icons";

const CONFIDENCE: Record<Checkpoint["confidence"], { label: string; cls: string }> = {
  high: { label: "確信度: 高", cls: "bg-[#3fae6a22] text-[#3fae6a]" },
  medium: { label: "確信度: 中", cls: "bg-[#d9a21b22] text-[#c2900f]" },
  low: { label: "確信度: 低 — 要確認", cls: "bg-[#e5484d22] text-[#e5484d]" },
};

/** An agent checkpoint, emphasizing the decision and its rationale (判断基準). */
export function CheckpointCard({ cp, issueLabel, onOpenIssue }: { cp: Checkpoint; issueLabel?: string; onOpenIssue?: (key: string) => void }) {
  const low = cp.confidence === "low";
  const art = cp.artifacts as { branch?: string; commits?: string[]; pr?: string; files?: string[] };
  return (
    <div className={`rounded-lg border bg-elev p-3.5 ${low ? "border-[#e5484d66]" : "border-line"}`}>
      <div className="mb-2 flex flex-wrap items-center gap-2 text-[12px]">
        <Avatar user={cp.author} size={16} />
        <span className="font-medium">{cp.author.display_name}</span>
        <span className="rounded bg-accent-soft px-1.5 font-medium text-accent">チェックポイント #{cp.seq}</span>
        {issueLabel && (
          <button onClick={() => onOpenIssue?.(cp.issue_key)} className="truncate font-mono text-[11.5px] text-muted hover:text-fg">{issueLabel}</button>
        )}
        <span className={`flex items-center gap-1 rounded px-1.5 font-medium ${CONFIDENCE[cp.confidence].cls}`}>
          {low && <AlertTriangle size={11} />}{CONFIDENCE[cp.confidence].label}
        </span>
        <span className="text-faint">{relativeTime(cp.created_at)}</span>
        {(cp.source === "mcp" || cp.source === "jira") && <span className="rounded bg-hover px-1 text-[10.5px] text-faint">{cp.source === "mcp" ? "MCP" : "Jira API"}</span>}
        {cp.progress !== null && (
          <span className="ml-auto flex items-center gap-1.5 text-muted">
            <span className="h-1.5 w-16 overflow-hidden rounded-full bg-hover"><span className="block h-full bg-accent" style={{ width: `${cp.progress}%` }} /></span>
            {cp.progress}%
          </span>
        )}
      </div>
      <div className="mb-2 text-[13.5px] font-medium">{cp.summary}</div>
      <dl className="space-y-1.5 text-[12.5px] leading-relaxed">
        {cp.decision && (
          <Row icon={<Flag size={12} />} label="判断">{cp.decision}</Row>
        )}
        <div className="flex gap-2 rounded-md bg-accent-soft px-2.5 py-1.5">
          <dt className="flex w-[72px] shrink-0 items-center gap-1 whitespace-nowrap font-medium text-accent"><Scale size={12} />判断基準</dt>
          <dd className="flex-1">{cp.rationale}</dd>
        </div>
        {cp.alternatives.length > 0 && (
          <Row icon={<span className="text-[11px]">⇄</span>} label="代替案">
            <ul className="list-disc pl-4">{cp.alternatives.map((a, i) => <li key={i}>{a}</li>)}</ul>
          </Row>
        )}
        {cp.next_steps && <Row icon={<ArrowRight size={12} />} label="次">{cp.next_steps}</Row>}
      </dl>
      {(art.branch || art.commits?.length || art.pr || art.files?.length) && (
        <div className="mt-2 flex flex-wrap gap-1.5 font-mono text-[11px] text-muted">
          {art.branch && <span className="flex items-center gap-1 rounded bg-hover px-1.5 py-0.5"><GitBranch size={11} />{art.branch}</span>}
          {art.commits?.map((c) => <span key={c} className="flex items-center gap-1 rounded bg-hover px-1.5 py-0.5"><GitCommit size={11} />{c.slice(0, 7)}</span>)}
          {art.pr && <a href={art.pr} target="_blank" rel="noreferrer" className="rounded bg-hover px-1.5 py-0.5 text-accent hover:underline">PR</a>}
          {art.files?.map((f) => <span key={f} className="rounded bg-hover px-1.5 py-0.5">{f}</span>)}
        </div>
      )}
    </div>
  );
}

function Row({ icon, label, children }: { icon: React.ReactNode; label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2 px-2.5">
      <dt className="flex w-[72px] shrink-0 items-center gap-1 whitespace-nowrap text-faint">{icon}{label}</dt>
      <dd className="flex-1 text-fg/90">{children}</dd>
    </div>
  );
}
