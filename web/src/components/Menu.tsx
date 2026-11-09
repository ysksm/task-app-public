import { Check } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

/**
 * A button that opens a floating panel; closes on outside click or Escape.
 * fixed: place the panel in the viewport, for triggers inside a scrolling bar
 * (overflow-x: auto also clips vertically, so an absolute panel would be cut off).
 */
export function Popover({ trigger, children, align = "left", className = "", fixed = false }: {
  trigger: (open: boolean) => ReactNode;
  children: (close: () => void) => ReactNode;
  align?: "left" | "right";
  className?: string;
  fixed?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [at, setAt] = useState<{ top: number; left?: number; right?: number } | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  const toggle = () => {
    if (fixed && !open && ref.current) {
      const r = ref.current.getBoundingClientRect();
      setAt(align === "right" ? { top: r.bottom + 4, right: window.innerWidth - r.right } : { top: r.bottom + 4, left: r.left });
    }
    setOpen((o) => !o);
  };
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && (e.stopPropagation(), setOpen(false));
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey, true);
    };
  }, [open]);
  return (
    <div ref={ref} className={`relative ${className}`}>
      <div onClick={(e) => (e.stopPropagation(), toggle())}>{trigger(open)}</div>
      {open && (
        <div onClick={(e) => e.stopPropagation()} style={fixed && at ? { position: "fixed", ...at } : undefined}
          className={`${fixed ? "" : `absolute mt-1 ${align === "right" ? "right-0" : "left-0"}`} z-40 min-w-[200px] rounded-lg border border-line-strong bg-elev p-1 shadow-xl shadow-black/30`}>
          {children(() => setOpen(false))}
        </div>
      )}
    </div>
  );
}

export interface Option<T> {
  value: T;
  label: string;
  icon?: ReactNode;
  hint?: string;
}

/** Searchable option list for use inside a Popover. */
export function OptionList<T>({ options, selected, onSelect, multi = false, placeholder = "検索..." }: {
  options: Option<T>[];
  selected: T[];
  onSelect: (v: T) => void;
  multi?: boolean;
  placeholder?: string;
}) {
  const [q, setQ] = useState("");
  const [active, setActive] = useState(0);
  const shown = options.filter((o) => o.label.toLowerCase().includes(q.toLowerCase()));
  return (
    <div className="w-[240px]">
      {options.length > 6 && (
        <input autoFocus value={q} placeholder={placeholder}
          onChange={(e) => (setQ(e.target.value), setActive(0))}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") setActive((a) => Math.min(a + 1, shown.length - 1));
            if (e.key === "ArrowUp") setActive((a) => Math.max(a - 1, 0));
            if (e.key === "Enter" && shown[active]) onSelect(shown[active].value);
          }}
          className="mb-1 w-full rounded-md border border-line bg-bg px-2 py-1.5 text-[13px] outline-none focus:border-accent" />
      )}
      <div className="max-h-[300px] overflow-y-auto">
        {shown.map((o, i) => {
          const isSel = selected.includes(o.value);
          return (
            <button key={String(o.value)} type="button" onMouseEnter={() => setActive(i)} onClick={() => onSelect(o.value)}
              className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] ${i === active ? "bg-hover" : ""}`}>
              {multi && (
                <span className={`flex h-3.5 w-3.5 items-center justify-center rounded-[3px] border ${isSel ? "border-accent bg-accent" : "border-line-strong"}`}>
                  {isSel && <Check size={10} className="text-white" />}
                </span>
              )}
              {o.icon}
              <span className="flex-1 truncate">{o.label}</span>
              {o.hint && <span className="text-[11px] text-faint">{o.hint}</span>}
              {!multi && isSel && <Check size={13} className="text-muted" />}
            </button>
          );
        })}
        {shown.length === 0 && <div className="px-2 py-2 text-faint">該当なし</div>}
      </div>
    </div>
  );
}
