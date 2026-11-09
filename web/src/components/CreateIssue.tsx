import { X } from "lucide-react";
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import type { IssueType } from "../api/client";
import { useCreateIssue, useIssue, useProjects, useStates, useUsers } from "../api/hooks";
import { allowedChildTypes, PRIORITY_META, TYPE_META } from "../lib/meta";
import { Avatar, PriorityIcon, StateIcon, TypeIcon } from "./icons";
import { OptionList, Popover } from "./Menu";

interface Defaults {
  projectKey?: string;
  parentKey?: string;
  type?: IssueType;
  assignee?: string;
}

const Ctx = createContext<(d?: Defaults) => void>(() => {});
export const useCreateIssueModal = () => useContext(Ctx);

/** Provides the "create issue" modal; `C` opens it anywhere. */
export function CreateIssueProvider({ children, currentProject, onCreated }: {
  children: ReactNode;
  currentProject?: string;
  onCreated: (key: string) => void;
}) {
  const [defaults, setDefaults] = useState<Defaults | null>(null);
  const open = useCallback((d?: Defaults) => setDefaults({ projectKey: currentProject, ...d }), [currentProject]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName)) return;
      if (e.key === "c" && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        open();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);
  return (
    <Ctx.Provider value={open}>
      {children}
      {defaults && <CreateIssueModal defaults={defaults} onClose={() => setDefaults(null)} onCreated={onCreated} />}
    </Ctx.Provider>
  );
}

function CreateIssueModal({ defaults, onClose, onCreated }: { defaults: Defaults; onClose: () => void; onCreated: (key: string) => void }) {
  const { data: projects = [] } = useProjects();
  const { data: users = [] } = useUsers();
  const [projectKey, setProjectKey] = useState(defaults.projectKey ?? "");
  const [parentKey, setParentKey] = useState(defaults.parentKey ?? "");
  const { data: parent } = useIssue(parentKey || undefined);
  const project = projects.find((p) => p.key === projectKey) ?? projects[0];
  const { data: states = [] } = useStates(project?.key);
  const allowed = allowedChildTypes(parent?.type ?? null, project?.allow_skip_levels ?? true);
  const [type, setType] = useState<IssueType>(defaults.type ?? "task");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [stateId, setStateId] = useState<number | null>(null);
  const [priority, setPriority] = useState(0);
  const [assignee, setAssignee] = useState(defaults.assignee ?? "");
  const [more, setMore] = useState(false);
  const create = useCreateIssue();

  useEffect(() => {
    if (!projectKey && projects[0]) setProjectKey(projects[0].key);
  }, [projects, projectKey]);
  useEffect(() => {
    if (allowed.length && !allowed.includes(type)) setType(allowed.includes("task") ? "task" : allowed[0]);
  }, [allowed, type]);
  const applicable = states.filter((s) => s.applies_to === (type === "qa" ? "qa" : "all"));
  const state = applicable.find((s) => s.id === stateId) ?? applicable.find((s) => s.is_default) ?? applicable[0];

  const submit = () => {
    if (!title.trim() || !project) return;
    create.mutate(
      {
        project_key: project.key, type, title, description, priority,
        parent_key: parentKey || null, state_id: state?.id ?? null, assignee: assignee || null,
      },
      {
        onSuccess: (is) => {
          if (more) {
            setTitle("");
            setDescription("");
          } else {
            onClose();
            onCreated(is.key);
          }
        },
      },
    );
  };

  const chip = "flex items-center gap-1.5 rounded-md border border-line px-2 py-1 text-[12px] hover:bg-hover";
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 pt-[12vh]" onMouseDown={onClose}>
      <div onMouseDown={(e) => e.stopPropagation()} className="w-[640px] rounded-xl border border-line-strong bg-elev shadow-2xl"
        onKeyDown={(e) => { if (e.key === "Escape") onClose(); if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) submit(); }}>
        <div className="flex items-center gap-2 px-4 pt-3 text-[12px] text-muted">
          <Popover trigger={() => <button className={chip}>{project?.key ?? "—"}</button>}>
            {(close) => (
              <OptionList options={projects.map((p) => ({ value: p.key, label: `${p.key} ${p.name}` }))} selected={[project?.key ?? ""]}
                onSelect={(k) => (setProjectKey(k), setParentKey(""), close())} />
            )}
          </Popover>
          {parent && (
            <span className={chip}>
              親: <TypeIcon type={parent.type} size={12} /> <span className="font-mono">{parent.key}</span>
              <span className="max-w-[160px] truncate">{parent.title}</span>
              <button onClick={() => setParentKey("")} className="text-faint hover:text-fg"><X size={12} /></button>
            </span>
          )}
          {!parent && (
            <input value={parentKey} onChange={(e) => setParentKey(e.target.value.toUpperCase())} placeholder="親キー (任意)"
              className="w-28 rounded-md border border-line bg-transparent px-2 py-1 font-mono text-[12px] outline-none focus:border-accent" />
          )}
          <span className="flex-1" />
          <span>新規イシュー</span>
          <button onClick={onClose} className="rounded p-1 hover:bg-hover"><X size={14} /></button>
        </div>
        <div className="px-4 pt-3">
          <input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} placeholder="タイトル"
            className="w-full bg-transparent text-[18px] font-medium outline-none placeholder:text-faint" />
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="説明 (Markdown)" rows={4}
            className="mt-2 w-full resize-none bg-transparent text-[13px] outline-none placeholder:text-faint" />
        </div>
        <div className="flex flex-wrap items-center gap-1.5 px-4 pb-3">
          <Popover trigger={() => <button className={chip}><TypeIcon type={type} size={13} />{TYPE_META[type].label}</button>}>
            {(close) => (
              <OptionList options={allowed.map((t) => ({ value: t, label: TYPE_META[t].label, icon: <TypeIcon type={t} /> }))} selected={[type]}
                onSelect={(t) => (setType(t), setStateId(null), close())} />
            )}
          </Popover>
          {state && (
            <Popover trigger={() => <button className={chip}><StateIcon state={state} />{state.name}</button>}>
              {(close) => (
                <OptionList options={applicable.map((s) => ({ value: s.id, label: s.name, icon: <StateIcon state={s} /> }))} selected={[state.id]}
                  onSelect={(id) => (setStateId(id), close())} />
              )}
            </Popover>
          )}
          <Popover trigger={() => <button className={chip}><PriorityIcon priority={priority} />{PRIORITY_META[priority].label}</button>}>
            {(close) => (
              <OptionList options={PRIORITY_META.map((p) => ({ value: p.value, label: p.label, icon: <PriorityIcon priority={p.value} /> }))} selected={[priority]}
                onSelect={(p) => (setPriority(p), close())} />
            )}
          </Popover>
          <Popover trigger={() => {
            const u = users.find((x) => x.username === assignee);
            return <button className={chip}><Avatar user={u ?? null} size={14} />{u?.display_name ?? "担当"}</button>;
          }}>
            {(close) => (
              <OptionList options={[{ value: "", label: "未割り当て" }, ...users.map((u) => ({ value: u.username, label: u.display_name, icon: <Avatar user={u} size={16} />, hint: u.kind === "agent" ? "agent" : undefined }))]}
                selected={[assignee]} onSelect={(v) => (setAssignee(v), close())} />
            )}
          </Popover>
        </div>
        <div className="flex items-center gap-3 border-t border-line px-4 py-2.5">
          {create.error && <span className="text-[12px] text-danger">{(create.error as Error).message}</span>}
          <span className="flex-1" />
          <label className="flex items-center gap-1.5 text-[12px] text-muted">
            <input type="checkbox" checked={more} onChange={(e) => setMore(e.target.checked)} className="accent-[var(--accent)]" /> 続けて作成
          </label>
          <button onClick={submit} disabled={!title.trim() || create.isPending} className="rounded-md bg-accent px-3 py-1.5 text-[12px] font-medium text-white disabled:opacity-40">
            作成 (⌘↵)
          </button>
        </div>
      </div>
    </div>
  );
}
