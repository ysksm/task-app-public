import type { IssueType, ProjectTier, RequirementsMode, State } from "../api/client";

export const TYPE_META: Record<IssueType, { label: string; color: string; short: string }> = {
  epic: { label: "エピック", color: "#8b6cf0", short: "E" },
  story: { label: "ストーリー", color: "#4c8df0", short: "S" },
  feature: { label: "フィーチャー", color: "#3fae6a", short: "F" },
  task: { label: "タスク", color: "#9aa0b0", short: "T" },
  qa: { label: "QA", color: "#e0823d", short: "?" },
};
export const ISSUE_TYPES: IssueType[] = ["epic", "story", "feature", "task", "qa"];

export const PRIORITY_META: { value: number; label: string }[] = [
  { value: 0, label: "優先度なし" },
  { value: 1, label: "緊急" },
  { value: 2, label: "高" },
  { value: 3, label: "中" },
  { value: 4, label: "低" },
];

/** Project tiers, in display order. 「その他」= low priority, things started casually. */
export const TIERS: ProjectTier[] = ["focus", "normal", "side"];
export const TIER_LABEL: Record<ProjectTier, string> = { focus: "注力", normal: "通常", side: "その他" };
export const TIER_HINT: Record<ProjectTier, string> = {
  focus: "いま力を入れているプロジェクト",
  normal: "ときどき進めるプロジェクト",
  side: "優先度は低め・気軽に始めてみたもの",
};
/** Requirements modes (docs/09 §6): how the story map is written. */
export const REQ_MODES: RequirementsMode[] = ["quick", "structured"];
export const REQ_MODE_LABEL: Record<RequirementsMode, string> = { quick: "まず書く", structured: "整理して書く" };
export const REQ_MODE_HINT: Record<RequirementsMode, string> = {
  quick: "ひとことで未整理に置き、手順・リリース・条件の細かさはあとで決める",
  structured: "ストーリーは手順の下に置き、受け入れ条件は前提・操作・結果で書く。抜けは警告で示す",
};
export const tierRank = (t: ProjectTier | undefined) => TIERS.indexOf(t ?? "normal");

export const CATEGORY_LABEL: Record<State["category"], string> = {
  backlog: "Backlog",
  unstarted: "未着手",
  started: "進行中",
  completed: "完了",
  canceled: "キャンセル",
};

/** Allowed child types for a parent type (mirrors the server rules; the server is authoritative). */
export function allowedChildTypes(parent: IssueType | null, allowSkip = true): IssueType[] {
  switch (parent) {
    case null:
      return ["epic", "story", "feature", "task", "qa"];
    case "epic":
      return allowSkip ? ["story", "feature", "task", "qa"] : ["story", "qa"];
    case "story":
      return allowSkip ? ["feature", "task", "qa"] : ["feature", "qa"];
    case "feature":
    case "task":
      return ["task", "qa"];
    case "qa":
      return [];
  }
}

export function relativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return "たった今";
  if (diff < 3600) return `${Math.floor(diff / 60)}分前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}時間前`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}日前`;
  return new Date(iso).toLocaleDateString("ja-JP");
}
