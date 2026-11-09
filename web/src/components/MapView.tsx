import {
  applyNodeChanges, Background, Controls, getNodesBounds, getViewportForBounds, Handle, MiniMap, Position, ReactFlow,
  ReactFlowProvider, useReactFlow, type Edge, type Node, type NodeChange, type NodeProps,
} from "@xyflow/react";
import { toPng } from "html-to-image";
import "@xyflow/react/dist/style.css";
import { ChevronLeft, ChevronRight, Download, FolderKanban, Maximize2, MessageCircleQuestion } from "lucide-react";
import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { Issue } from "../api/client";
import { useUpdateIssue } from "../api/hooks";
import { TYPE_META } from "../lib/meta";
import { Avatar, ProgressPie, StateIcon, TypeIcon } from "./icons";

// Mind map (MAP-01〜05): the project's hierarchy laid out as a horizontal
// tree. QA tickets are not nodes; they show as a badge on the issue they belong to.

const W = 250, H = 64, ROOT_W = 200, GAP_X = 64, GAP_Y = 14;

interface Tree {
  issue?: Issue; // undefined for the project root
  children: Tree[];
}

type IssueNodeData = {
  issue?: Issue;
  title: string;
  childCount: number;
  collapsed: boolean;
  dimmed: boolean;
  selected: boolean;
  dropTarget?: boolean;
};

const MapActions = createContext<{ toggle: (key: string) => void; open: (key: string) => void }>({ toggle: () => {}, open: () => {} });

function buildForest(issues: Issue[], rootKey?: string): Tree[] {
  const byParent = new Map<string, Issue[]>();
  const keys = new Set(issues.map((i) => i.key));
  for (const is of issues) {
    if (is.type === "qa") continue;
    const p = is.parent_key && keys.has(is.parent_key) ? is.parent_key : "";
    if (!byParent.has(p)) byParent.set(p, []);
    byParent.get(p)!.push(is);
  }
  for (const list of byParent.values()) list.sort((a, b) => a.sort_order - b.sort_order || a.number - b.number);
  const build = (is: Issue): Tree => ({ issue: is, children: (byParent.get(is.key) ?? []).map(build) });
  if (rootKey) {
    const root = issues.find((i) => i.key === rootKey);
    return root ? [build(root)] : [];
  }
  return (byParent.get("") ?? []).map(build);
}

function layout(root: Tree, collapsed: Set<string>, matched: Set<string> | null, selected: string | undefined, projectName: string) {
  const nodes: Node<IssueNodeData>[] = [];
  const edges: Edge[] = [];
  const kids = (t: Tree) => (t.issue && collapsed.has(t.issue.key) ? [] : t.children);
  const height = (t: Tree): number => {
    const c = kids(t);
    if (c.length === 0) return H;
    return Math.max(H, c.reduce((s, x) => s + height(x), 0) + GAP_Y * (c.length - 1));
  };
  const place = (t: Tree, x: number, top: number, parentId?: string) => {
    const h = height(t);
    const id = t.issue?.key ?? "__root__";
    const w = t.issue ? W : ROOT_W;
    nodes.push({
      id, type: t.issue ? "issue" : "root", position: { x, y: top + h / 2 - H / 2 }, width: w, height: H, draggable: !!t.issue,
      data: {
        issue: t.issue, title: t.issue?.title ?? projectName, childCount: t.children.length,
        collapsed: !!t.issue && collapsed.has(t.issue.key),
        dimmed: !!matched && !!t.issue && !matched.has(t.issue.key),
        selected: !!t.issue && t.issue.key === selected,
      },
    });
    if (parentId) {
      edges.push({ id: `${parentId}->${id}`, source: parentId, target: id, type: "default",
        style: { stroke: "var(--border-strong)", strokeWidth: 1.5, opacity: matched && t.issue && !matched.has(t.issue.key) ? 0.35 : 1 } });
    }
    let y = top;
    for (const c of kids(t)) {
      place(c, x + w + GAP_X, y, id);
      y += height(c) + GAP_Y;
    }
  };
  place(root, 0, 0);
  return { nodes, edges };
}

function IssueNode({ data }: NodeProps<Node<IssueNodeData>>) {
  const { toggle, open } = useContext(MapActions);
  const is = data.issue!;
  const color = TYPE_META[is.type].color;
  const agent = is.assignee?.kind === "agent";
  return (
    <div onClick={() => open(is.key)} onDoubleClick={(e) => { e.stopPropagation(); if (data.childCount) toggle(is.key); }}
      style={{ width: W, height: H, opacity: data.dimmed ? 0.32 : 1, borderLeftColor: color }}
      className={`group relative flex cursor-pointer flex-col justify-center rounded-lg border border-l-4 bg-elev px-2.5 py-1.5 text-left shadow-sm transition-opacity
        ${data.dropTarget ? "border-[#3fae6a] ring-2 ring-[#3fae6a]/50" : data.selected ? "border-accent ring-2 ring-accent/40" : "border-line hover:border-[var(--border-strong)]"}`}>
      <Handle type="target" position={Position.Left} className="!h-1 !w-1 !min-w-0 !border-0 !bg-transparent" />
      <div className="flex min-w-0 items-center gap-1.5 whitespace-nowrap text-[11px] text-faint">
        <span className="shrink-0"><TypeIcon type={is.type} size={12} /></span>
        <span className="shrink-0 whitespace-nowrap font-mono">{is.key}</span>
        <span className="shrink-0"><StateIcon state={is.state} size={11} /></span>
        <span className="min-w-0 truncate">{is.state.name}</span>
        <span className="flex-1" />
        {is.open_qa_count > 0 && (
          <span title={`未回答の QA ${is.open_qa_count} 件`} className={`flex items-center gap-0.5 rounded px-1 ${is.blocking_qa_count ? "bg-[#e5484d22] text-[#e5484d]" : "bg-[#e0823d22] text-[#c2700f]"}`}>
            <MessageCircleQuestion size={10} />{is.open_qa_count}
          </span>
        )}
        <ProgressPie total={is.progress.total} done={is.progress.done} size={12} />
        {is.assignee && <span className={agent ? "rounded-full ring-1 ring-[#e0823d]" : ""}><Avatar user={is.assignee} size={15} /></span>}
      </div>
      <div className="mt-0.5 line-clamp-2 text-[12.5px] font-medium leading-snug text-fg">{data.title}</div>
      {data.childCount > 0 && (
        <button title={data.collapsed ? "展開" : "折りたたむ"} onClick={(e) => { e.stopPropagation(); toggle(is.key); }}
          className="absolute -right-3 top-1/2 flex h-5 min-w-5 -translate-y-1/2 items-center justify-center rounded-full border border-line bg-elev px-1 text-[10px] text-muted shadow-sm hover:text-fg">
          {data.collapsed ? <>{data.childCount}<ChevronRight size={10} /></> : <ChevronLeft size={10} />}
        </button>
      )}
      <Handle type="source" position={Position.Right} className="!h-1 !w-1 !min-w-0 !border-0 !bg-transparent" />
    </div>
  );
}

function RootNode({ data }: NodeProps<Node<IssueNodeData>>) {
  return (
    <div style={{ width: ROOT_W, height: H }} className={`flex items-center gap-2 rounded-xl border-2 bg-accent-soft px-3 text-[13.5px] font-semibold ${data.dropTarget ? "border-[#3fae6a] ring-2 ring-[#3fae6a]/50" : "border-accent"}`}>
      <FolderKanban size={16} className="shrink-0 text-accent" />
      <span className="line-clamp-2">{data.title}</span>
      <Handle type="source" position={Position.Right} className="!h-1 !w-1 !min-w-0 !border-0 !bg-transparent" />
    </div>
  );
}

const nodeTypes = { issue: IssueNode, root: RootNode };

const LARGE = 300;

/** Keys to collapse so that only `depth` levels below the root are shown. */
function collapsedBelow(issues: Issue[], depth: number): Set<string> {
  const byKey = new Map(issues.map((i) => [i.key, i]));
  const level = (is: Issue): number => {
    let d = 0;
    for (let p = is.parent_key; p && byKey.has(p); p = byKey.get(p)!.parent_key) d++;
    return d;
  };
  const out = new Set<string>();
  for (const is of issues) if (level(is) >= depth - 1) out.add(is.key);
  return out;
}

export interface MapViewProps {
  /** Every issue of the project (the map always shows the whole hierarchy). */
  issues: Issue[];
  /** Keys matching the current filter; null when no filter is active. */
  matched: Set<string> | null;
  projectName: string;
  rootKey?: string;
  selectedKey?: string;
  onOpen: (key: string) => void;
}

export function MapView(props: MapViewProps) {
  return (
    <ReactFlowProvider>
      <MapCanvas {...props} />
    </ReactFlowProvider>
  );
}

function MapCanvas({ issues, matched, projectName, rootKey, selectedKey, onOpen }: MapViewProps) {
  // Large projects open collapsed below the stories so the whole map fits.
  const [collapsed, setCollapsed] = useState<Set<string>>(() => (issues.length > LARGE ? collapsedBelow(issues, 2) : new Set()));
  const flow = useReactFlow();
  const { nodes, edges } = useMemo(() => {
    const forest = buildForest(issues, rootKey);
    const root: Tree = rootKey && forest[0] ? forest[0] : { children: forest };
    return layout(root, collapsed, matched, selectedKey, projectName);
  }, [issues, rootKey, collapsed, matched, selectedKey, projectName]);

  // Nodes are state so they can be dragged; every layout change resets them.
  const [flowNodes, setFlowNodes] = useState(nodes);
  useEffect(() => setFlowNodes(nodes), [nodes]);
  const [dropKey, setDropKey] = useState<string | null>(null);
  const [message, setMessage] = useState<string>();
  const update = useUpdateIssue();
  const byKey = useMemo(() => new Map(issues.map((i) => [i.key, i])), [issues]);
  const isDescendant = (key: string, of: string) => {
    for (let p = byKey.get(key)?.parent_key; p; p = byKey.get(p)?.parent_key) if (p === of) return true;
    return false;
  };
  const targetOf = (node: Node) => {
    const hits = flow.getIntersectingNodes(node).filter((n) => n.id !== node.id && !isDescendant(n.id, node.id));
    return hits.length ? hits[hits.length - 1].id : null;
  };
  const onDrag = (_: unknown, node: Node) => setDropKey(targetOf(node));
  const onDragStop = (_: unknown, node: Node) => {
    const target = targetOf(node);
    setDropKey(null);
    setFlowNodes(nodes); // snap back; the new layout arrives with the refetch
    const is = byKey.get(node.id);
    if (!target || !is) return;
    const parent = target === "__root__" ? null : target;
    if ((is.parent_key ?? null) === parent) return;
    setMessage(undefined);
    update.mutate({ key: is.key, parent_key: parent }, {
      onSuccess: () => setMessage(`${is.key} を ${parent ?? "プロジェクト直下"} に移動しました`),
      onError: (e) => setMessage(`移動できません: ${(e as Error).message}`),
    });
  };
  const shownNodes = useMemo(() => flowNodes.map((n) => (n.id === dropKey ? { ...n, data: { ...n.data, dropTarget: true } } : n)), [flowNodes, dropKey]);

  // Off-screen nodes are not rendered (onlyRenderVisibleElements), so
  // rendering is switched to "everything" while the image is taken.
  const [exporting, setExporting] = useState(false);
  const exportPng = async () => {
    setExporting(true);
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 50))));
    try {
      await savePng();
    } finally {
      setExporting(false);
    }
  };
  const savePng = async () => {
    const bounds = getNodesBounds(nodes);
    const width = Math.min(6000, bounds.width + 80), height = Math.min(6000, bounds.height + 80);
    const vp = getViewportForBounds(bounds, width, height, 0.05, 2, 0.02);
    const el = document.querySelector<HTMLElement>(".react-flow__viewport");
    if (!el) return;
    const bg = getComputedStyle(document.body).backgroundColor;
    const url = await toPng(el, { backgroundColor: bg, width, height,
      style: { width: `${width}px`, height: `${height}px`, transform: `translate(${vp.x}px, ${vp.y}px) scale(${vp.zoom})` } });
    const a = document.createElement("a");
    a.href = url;
    a.download = `${projectName || "map"}${rootKey ? "-" + rootKey : ""}.png`;
    a.click();
  };

  const toggle = (key: string) => {
    const next = new Set(collapsed);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    setCollapsed(next);
  };
  const fit = () => flow.fitView({ padding: 0.15, duration: 300 });
  useEffect(() => { const t = setTimeout(fit, 50); return () => clearTimeout(t); }, [rootKey, issues.length > 0]); // eslint-disable-line react-hooks/exhaustive-deps

  const collapseTo = (depth: number) => {
    setCollapsed(collapsedBelow(issues, depth));
    setTimeout(fit, 50);
  };

  return (
    <MapActions.Provider value={{ toggle, open: onOpen }}>
      <div className="relative h-full w-full">
        <ReactFlow nodes={shownNodes} edges={edges} nodeTypes={nodeTypes} minZoom={0.05} maxZoom={1.75}
          onNodesChange={(c: NodeChange<Node<IssueNodeData>>[]) => setFlowNodes((ns) => applyNodeChanges(c, ns))}
          onNodeDrag={onDrag} onNodeDragStop={onDragStop} nodeDragThreshold={6}
          nodesConnectable={false} elementsSelectable={false} onlyRenderVisibleElements={!exporting} proOptions={{ hideAttribution: true }} colorMode="system"
          fitView fitViewOptions={{ padding: 0.15 }}>
          <Background gap={24} size={1} color="var(--border)" />
          <Controls showInteractive={false} position="bottom-left" />
          <MiniMap pannable zoomable position="bottom-right" nodeStrokeWidth={2} style={{ width: 150, height: 96 }}
            nodeColor={(n) => (n.data as IssueNodeData).issue ? TYPE_META[(n.data as IssueNodeData).issue!.type].color : "var(--accent)"}
            className="!bg-elev" maskColor="rgba(120,120,140,0.12)" />
        </ReactFlow>
        <div className="absolute left-3 top-3 flex gap-1 rounded-lg border border-line bg-elev p-1 text-[12px] shadow-sm">
          <button onClick={fit} className="flex items-center gap-1 rounded px-2 py-0.5 text-muted hover:bg-hover hover:text-fg"><Maximize2 size={12} />全体表示</button>
          <span className="w-px bg-line" />
          <span className="px-1 py-0.5 text-faint">表示する深さ</span>
          {[1, 2, 3].map((d) => (
            <button key={d} onClick={() => collapseTo(d)} className="rounded px-1.5 py-0.5 text-muted hover:bg-hover hover:text-fg">{d}</button>
          ))}
          <button onClick={() => { setCollapsed(new Set()); setTimeout(fit, 50); }} className="rounded px-1.5 py-0.5 text-muted hover:bg-hover hover:text-fg">すべて</button>
          <span className="w-px bg-line" />
          <button onClick={exportPng} title="表示中の階層を PNG で保存" className="flex items-center gap-1 rounded px-2 py-0.5 text-muted hover:bg-hover hover:text-fg"><Download size={12} />PNG</button>
        </div>
        <div className="pointer-events-none absolute right-3 top-3 max-w-[360px] text-right text-[11.5px] text-faint">ノードをドラッグして別のノードに重ねると、親を付け替えられます</div>
        {message && (
          <div className={`absolute bottom-4 left-1/2 -translate-x-1/2 rounded-lg border px-3 py-1.5 text-[12.5px] shadow-lg ${message.startsWith("移動できません") ? "border-[#e5484d66] bg-elev text-danger" : "border-line bg-elev"}`}>
            {message}
            <button onClick={() => setMessage(undefined)} className="ml-2 text-faint hover:text-fg">×</button>
          </div>
        )}
        {nodes.length <= 1 && <div className="pointer-events-none absolute inset-0 flex items-center justify-center text-faint">表示するイシューがありません</div>}
      </div>
    </MapActions.Provider>
  );
}
