import type { Filter, IssueType, ProjectTier } from "../api/client";

/** Filter state kept in the URL (?f=...). Every field is optional. */
export interface FilterState {
  types?: IssueType[];
  states?: number[];
  /** usernames, "me", or "none" for unassigned */
  assignees?: string[];
  assigneeKind?: "human" | "agent";
  labels?: string[];
  priorities?: number[];
  /** issue key; matches its descendants */
  under?: string;
  q?: string;
  /** project tiers (focus / normal / side); cross-project screens only */
  tiers?: ProjectTier[];
  /** hide completed and canceled */
  hideClosed?: boolean;
}

export type ViewMode = "list" | "board" | "tree" | "map" | "roadmap";
export type GroupBy = "state" | "assignee" | "type" | "priority" | "none";
export type SortBy = "updated" | "created" | "priority" | "manual" | "key" | "due_date";

export function isEmptyFilter(f: FilterState): boolean {
  return Object.values(f).every((v) => v === undefined || v === false || v === "" || (Array.isArray(v) && v.length === 0));
}

export function countFilters(f: FilterState): number {
  return Object.entries(f).filter(([, v]) => !(v === undefined || v === false || v === "" || (Array.isArray(v) && v.length === 0))).length;
}

/** Converts UI state to the API Filter. */
export function toApiFilter(f: FilterState): Filter | undefined {
  const and: Filter[] = [];
  if (f.types?.length) and.push({ field: "type", op: "in", value: f.types });
  if (f.states?.length) and.push({ field: "state", op: "in", value: f.states });
  if (f.assignees?.length) {
    const names = f.assignees.filter((a) => a !== "none");
    const parts: Filter[] = [];
    if (names.length) parts.push({ field: "assignee", op: "in", value: names });
    if (f.assignees.includes("none")) parts.push({ field: "assignee", op: "empty" });
    and.push(parts.length === 1 ? parts[0] : { or: parts });
  }
  if (f.assigneeKind) and.push({ field: "assignee_kind", op: "eq", value: f.assigneeKind });
  if (f.labels?.length) and.push({ field: "label", op: "in", value: f.labels });
  if (f.priorities?.length) and.push({ field: "priority", op: "in", value: f.priorities });
  if (f.under) and.push({ field: "ancestor", op: "eq", value: f.under });
  if (f.q?.trim()) and.push({ field: "text", op: "contains", value: f.q.trim() });
  if (f.tiers?.length) and.push({ field: "project_tier", op: "in", value: f.tiers });
  if (f.hideClosed) and.push({ field: "state_category", op: "not_in", value: ["completed", "canceled"] });
  if (and.length === 0) return undefined;
  return and.length === 1 ? and[0] : { and };
}

export function toggle<T>(list: T[] | undefined, v: T): T[] | undefined {
  const cur = list ?? [];
  const next = cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v];
  return next.length ? next : undefined;
}
