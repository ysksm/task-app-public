import { useNavigate } from "@tanstack/react-router";
import { Bookmark, BookmarkPlus, MoreHorizontal, Pencil, Save, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import type { Filter, SavedView } from "../api/client";
import { useCreateView, useDeleteView, useUpdateView, useViews } from "../api/hooks";
import { Popover } from "../components/Menu";
import type { FilterState, GroupBy, SortBy, ViewMode } from "../lib/filters";
import { IssuesScreen, type IssuesSearch } from "./pages";

// Saved views (VIEW-05, S10). A view stores the API filter (usable by any
// client) and, in `display`, the UI state to restore: view mode, grouping,
// sort, the filter bar state and the mind map root.

interface Display {
  view?: ViewMode;
  group?: GroupBy;
  sort?: SortBy;
  f?: FilterState;
  root?: string;
  /** a screen's fixed condition (e.g. My Tasks: assignee = me), kept with the view */
  base?: Filter;
}

function toDisplay(s: IssuesSearch): Display {
  const d: Display = { view: s.view, group: s.group, sort: s.sort, f: s.f, root: s.root };
  return JSON.parse(JSON.stringify(d)); // drop undefined
}

const same = (a: Display, b: Display) => JSON.stringify(toDisplay(a)) === JSON.stringify(toDisplay(b)) && JSON.stringify(a.base) === JSON.stringify(b.base);

/** Save / update controls shown in the filter bar (after the other controls). */
export function ViewControls({ projectKey, search, filter, baseFilter, savedView }: {
  projectKey?: string; search: IssuesSearch; filter?: Filter; baseFilter?: Filter; savedView?: SavedView;
}) {
  const navigate = useNavigate();
  const create = useCreateView();
  const update = useUpdateView();
  const remove = useDeleteView();
  const [name, setName] = useState("");
  const [renaming, setRenaming] = useState<string | null>(null);
  const display: Display = { ...toDisplay(search), ...(baseFilter ? { base: baseFilter } : {}) };
  const dirty = savedView && !same(display, (savedView.display ?? {}) as Display);
  const btn = "flex items-center gap-1 rounded-md px-2 py-1 text-[12px] text-muted hover:bg-hover hover:text-fg";

  const saveNew = (close: () => void) => {
    if (!name.trim()) return;
    create.mutate({ name: name.trim(), project_key: projectKey ?? null, filter: filter ?? {}, display: display as Record<string, unknown> }, {
      onSuccess: (v) => { close(); setName(""); navigate({ to: "/views/$id", params: { id: v.id }, search: toDisplay(search) as IssuesSearch }); },
    });
  };

  return (
    <div className="flex items-center gap-1">
      {savedView && dirty && (
        <button className={`${btn} !text-accent`} disabled={update.isPending}
          onClick={() => update.mutate({ id: savedView.id, filter: filter ?? {}, display: display as Record<string, unknown> })}>
          <Save size={13} />ビューを更新
        </button>
      )}
      <Popover trigger={() => <button className={btn}><BookmarkPlus size={13} />{savedView ? "別名で保存" : "ビューを保存"}</button>}>
        {(close) => (
          <form className="w-[260px] p-2" onSubmit={(e) => { e.preventDefault(); saveNew(close); }}>
            <div className="mb-1.5 text-[12px] text-muted">現在のフィルタと表示を保存します{projectKey ? `（${projectKey}）` : "（全プロジェクト）"}</div>
            <input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="ビューの名前"
              className="w-full rounded-md border border-line bg-bg px-2 py-1 text-[13px] outline-none focus:border-accent" />
            <div className="mt-2 flex justify-end">
              <button disabled={!name.trim() || create.isPending} className="rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white disabled:opacity-40">保存</button>
            </div>
          </form>
        )}
      </Popover>
      {savedView && (
        <Popover trigger={() => <button className={btn} title="ビューの操作"><MoreHorizontal size={14} /></button>}>
          {(close) => renaming !== null ? (
            <form className="w-[240px] p-2" onSubmit={(e) => {
              e.preventDefault();
              if (renaming.trim()) update.mutate({ id: savedView.id, name: renaming.trim() }, { onSuccess: () => { setRenaming(null); close(); } });
            }}>
              <input autoFocus value={renaming} onChange={(e) => setRenaming(e.target.value)}
                className="w-full rounded-md border border-line bg-bg px-2 py-1 text-[13px] outline-none focus:border-accent" />
            </form>
          ) : (
            <div className="w-[180px] text-[13px]">
              <button onClick={() => setRenaming(savedView.name)} className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 hover:bg-hover"><Pencil size={13} />名前を変更</button>
              <button onClick={() => remove.mutate(savedView.id, { onSuccess: () => { close(); navigate({ to: savedView.project_key ? "/p/$key/issues" : "/my", params: { key: savedView.project_key ?? "" } }); } })}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-danger hover:bg-hover"><Trash2 size={13} />ビューを削除</button>
            </div>
          )}
        </Popover>
      )}
    </div>
  );
}

/** S10: a saved view, shown with the same list / board / tree / map screens. */
export function SavedViewPage({ id, search, setSearch }: { id: string; search: IssuesSearch; setSearch: (s: IssuesSearch, replace?: boolean) => void }) {
  const { data: views, isLoading } = useViews();
  const view = views?.find((v) => v.id === id);
  const display = (view?.display ?? {}) as Display;
  const empty = !search.view && !search.f && !search.group && !search.sort && !search.root;

  // Opening a view restores its stored display state into the URL.
  useEffect(() => {
    if (view && empty) setSearch({ ...toDisplay(display as IssuesSearch), peek: search.peek }, true);
  }, [view?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  if (isLoading) return <div className="p-6 text-faint">読み込み中...</div>;
  if (!view) return <div className="p-10 text-center text-faint">ビューが見つかりません</div>;
  // Views created through the API without UI state still filter with their stored filter.
  const baseFilter = display.base ?? (display.f || Object.keys(view.filter).length === 0 ? undefined : (view.filter as Filter));
  return (
    <IssuesScreen key={view.id} projectKey={view.project_key ?? undefined} search={search} setSearch={setSearch} baseFilter={baseFilter}
      savedView={view} showProject={!view.project_key}
      views={view.project_key ? ["list", "board", "tree", "map", "roadmap"] : ["list", "board", "tree"]}
      icon={<Bookmark size={15} className="text-muted" />}
      title={<>{view.name} {view.project_key && <span className="ml-1 font-mono text-[12px] text-faint">{view.project_key}</span>}</>} />
  );
}
