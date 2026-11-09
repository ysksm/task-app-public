import { useNavigate } from "@tanstack/react-router";
import {
  Bookmark, Bot, ClipboardCheck, CornerDownLeft, FolderKanban, Inbox, Kanban, Keyboard, Map as MapIcon, MessageCircleQuestion,
  Plus, Search, Settings, TrendingUp,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useIssues, useProjects, useViews } from "../api/hooks";
import { useCreateIssueModal } from "./CreateIssue";
import { StateIcon, TypeIcon } from "./icons";

// Command palette (M02, ISS-07) and global keyboard shortcuts.

interface Command {
  id: string;
  group: "操作" | "移動" | "プロジェクト" | "ビュー" | "イシュー";
  label: string;
  hint?: string;
  icon: ReactNode;
  keywords?: string;
  run: () => void;
}

const SHORTCUTS: [string, string][] = [
  ["⌘K / Ctrl+K", "コマンドパレット"],
  ["C", "新規イシュー"],
  ["/", "一覧のフィルタ（検索）へ"],
  ["G → M", "マイタスク"],
  ["G → B", "全体ボード"],
  ["G → Q", "QA インボックス"],
  ["G → A", "受入検査"],
  ["G → E", "エージェント活動"],
  ["G → P", "プロジェクト一覧"],
  ["G → S", "設定"],
  ["Esc", "詳細パネル・ダイアログを閉じる"],
  ["?", "このショートカット一覧"],
];

const typing = (e: KeyboardEvent) => {
  const t = e.target as HTMLElement;
  return t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName);
};

const OPEN_EVENT = "taskapp:open-palette";

/** Opens the palette from anywhere (e.g. the sidebar button). */
export const openCommandPalette = () => window.dispatchEvent(new Event(OPEN_EVENT));

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [help, setHelp] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    let g = 0; // time of the last "g" press
    const go: Record<string, string> = { m: "/my", b: "/board", q: "/inbox/qa", a: "/inbox/acceptance", e: "/agents", p: "/projects", s: "/settings" };
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
        return;
      }
      if (typing(e) || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === "?") {
        e.preventDefault();
        setHelp((h) => !h);
        return;
      }
      if (e.key === "g") {
        g = Date.now();
        return;
      }
      if (Date.now() - g < 1200 && go[e.key]) {
        e.preventDefault();
        g = 0;
        navigate({ to: go[e.key] });
      }
    };
    const onOpen = () => setOpen(true);
    window.addEventListener("keydown", onKey);
    window.addEventListener(OPEN_EVENT, onOpen);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener(OPEN_EVENT, onOpen);
    };
  }, [navigate]);

  return (
    <>
      {open && createPortal(<Palette onClose={() => setOpen(false)} onHelp={() => { setOpen(false); setHelp(true); }} />, document.body)}
      {help && createPortal(<ShortcutHelp onClose={() => setHelp(false)} />, document.body)}
    </>
  );
}

function Palette({ onClose, onHelp }: { onClose: () => void; onHelp: () => void }) {
  const navigate = useNavigate();
  const openCreate = useCreateIssueModal();
  const { data: projects = [] } = useProjects();
  const { data: views = [] } = useViews();
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 150);
    return () => clearTimeout(t);
  }, [q]);
  const { data: found } = useIssues({
    filter: { field: "text", op: "contains", value: debounced }, limit: 20, enabled: debounced.length > 0,
  });

  const go = (to: string, params?: Record<string, string>, search?: Record<string, unknown>) => () => navigate({ to, params, search } as never);
  const commands = useMemo<Command[]>(() => {
    const list: Command[] = [
      { id: "new", group: "操作", label: "新規イシュー", hint: "C", icon: <Plus size={14} />, keywords: "create new 作成", run: () => openCreate() },
      { id: "help", group: "操作", label: "キーボードショートカット", hint: "?", icon: <Keyboard size={14} />, keywords: "shortcut help", run: onHelp },
      { id: "my", group: "移動", label: "マイタスク", hint: "G M", icon: <Inbox size={14} />, keywords: "my tasks", run: go("/my") },
      { id: "board", group: "移動", label: "全体ボード", hint: "G B", icon: <Kanban size={14} />, keywords: "board kanban all projects", run: go("/board") },
      { id: "qa", group: "移動", label: "QA インボックス", hint: "G Q", icon: <MessageCircleQuestion size={14} />, keywords: "question", run: go("/inbox/qa") },
      { id: "acc", group: "移動", label: "受入検査", hint: "G A", icon: <ClipboardCheck size={14} />, keywords: "acceptance", run: go("/inbox/acceptance") },
      { id: "agents", group: "移動", label: "エージェント活動", hint: "G E", icon: <Bot size={14} />, keywords: "agent checkpoint", run: go("/agents") },
      { id: "projects", group: "移動", label: "プロジェクト一覧", hint: "G P", icon: <FolderKanban size={14} />, keywords: "projects", run: go("/projects") },
      { id: "settings", group: "移動", label: "設定", hint: "G S", icon: <Settings size={14} />, keywords: "settings token mcp jira export", run: go("/settings") },
    ];
    for (const p of projects) {
      const kw = `${p.key} ${p.name}`;
      list.push(
        { id: `p-${p.key}`, group: "プロジェクト", label: `${p.name} — イシュー`, hint: p.key, icon: <FolderKanban size={14} />, keywords: kw, run: go("/p/$key/issues", { key: p.key }) },
        { id: `m-${p.key}`, group: "プロジェクト", label: `${p.name} — マインドマップ`, hint: p.key, icon: <MapIcon size={14} />, keywords: `${kw} map`, run: go("/p/$key/issues", { key: p.key }, { view: "map" }) },
        { id: `r-${p.key}`, group: "プロジェクト", label: `${p.name} — ロードマップ`, hint: p.key, icon: <TrendingUp size={14} />, keywords: `${kw} roadmap`, run: go("/p/$key/issues", { key: p.key }, { view: "roadmap" }) },
        { id: `s-${p.key}`, group: "プロジェクト", label: `${p.name} — プロジェクト設定`, hint: p.key, icon: <Settings size={14} />, keywords: `${kw} settings`, run: go("/p/$key/settings", { key: p.key }) },
      );
    }
    for (const v of views) {
      list.push({ id: `v-${v.id}`, group: "ビュー", label: v.name, hint: v.project_key ?? "", icon: <Bookmark size={14} />, keywords: "view", run: go("/views/$id", { id: v.id }) });
    }
    return list;
  }, [projects, views]); // eslint-disable-line react-hooks/exhaustive-deps

  const needle = q.trim().toLowerCase();
  const shown = useMemo(() => {
    const matched = commands.filter((c) => !needle || `${c.label} ${c.keywords ?? ""} ${c.hint ?? ""}`.toLowerCase().includes(needle));
    const issues: Command[] = needle
      ? (found?.issues ?? []).map((is) => ({
          id: `i-${is.key}`, group: "イシュー" as const, label: is.title, hint: is.key,
          icon: <span className="flex items-center gap-1"><TypeIcon type={is.type} size={13} /><StateIcon state={is.state} size={12} /></span>,
          run: go("/p/$key/issues", { key: is.project_key }, { peek: is.key }),
        }))
      : [];
    // Issues first once the user types something that looks like a search.
    return needle ? [...issues, ...matched.slice(0, 12)] : matched;
  }, [commands, needle, found]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => setActive(0), [needle, found]);
  useEffect(() => {
    listRef.current?.querySelector(`[data-index="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const run = (c?: Command) => {
    if (!c) return;
    onClose();
    c.run();
  };

  let lastGroup = "";
  return (
    <div className="fixed inset-0 z-[60] flex items-start justify-center bg-black/40 pt-[14vh]" onMouseDown={onClose}>
      <div onMouseDown={(e) => e.stopPropagation()} className="w-[620px] overflow-hidden rounded-xl border border-line-strong bg-elev shadow-2xl">
        <div className="flex items-center gap-2 border-b border-line px-4">
          <Search size={15} className="text-faint" />
          <input autoFocus value={q} onChange={(e) => setQ(e.target.value)} placeholder="移動先・操作・イシュー（キーやタイトル）を検索"
            onKeyDown={(e) => {
              if (e.nativeEvent.isComposing) return;
              if (e.key === "Escape") onClose();
              if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => Math.min(a + 1, shown.length - 1)); }
              if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
              if (e.key === "Enter") { e.preventDefault(); run(shown[active]); }
            }}
            className="h-12 flex-1 bg-transparent text-[14px] outline-none" />
          <kbd className="rounded border border-line px-1.5 text-[11px] text-faint">esc</kbd>
        </div>
        <div ref={listRef} className="max-h-[420px] overflow-y-auto py-1">
          {shown.length === 0 && <div className="px-4 py-6 text-center text-[13px] text-faint">{debounced && !found ? "検索中..." : "見つかりません"}</div>}
          {shown.map((c, i) => {
            const header = c.group !== lastGroup ? c.group : null;
            lastGroup = c.group;
            return (
              <div key={c.id}>
                {header && <div className="px-4 pb-1 pt-2 text-[11px] font-medium text-faint">{header}</div>}
                <button data-index={i} onMouseMove={() => setActive(i)} onClick={() => run(c)}
                  className={`flex w-full items-center gap-2.5 px-4 py-1.5 text-left text-[13px] ${i === active ? "bg-active" : ""}`}>
                  <span className="flex w-5 shrink-0 justify-center text-muted">{c.icon}</span>
                  <span className="min-w-0 flex-1 truncate">{c.label}</span>
                  {c.hint && <span className="shrink-0 font-mono text-[11px] text-faint">{c.hint}</span>}
                  {i === active && <CornerDownLeft size={12} className="shrink-0 text-faint" />}
                </button>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function ShortcutHelp({ onClose }: { onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40" onMouseDown={onClose}>
      <div onMouseDown={(e) => e.stopPropagation()} className="w-[420px] rounded-xl border border-line-strong bg-elev p-5 shadow-2xl">
        <h2 className="mb-3 flex items-center gap-2 text-[14px] font-semibold"><Keyboard size={15} />キーボードショートカット</h2>
        <dl className="grid grid-cols-[130px_1fr] gap-y-1.5 text-[13px]">
          {SHORTCUTS.map(([k, v]) => (
            <div key={k} className="contents">
              <dt><kbd className="rounded border border-line bg-bg px-1.5 py-0.5 font-mono text-[11.5px]">{k}</kbd></dt>
              <dd className="text-muted">{v}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  );
}
