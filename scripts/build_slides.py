"""Build standalone HTML slide decks from their sources.

Usage:
  python3 scripts/build_slides.py                 # build every deck in docs/slides/src/<deck>/
  python3 scripts/build_slides.py <srcdir> <out>  # build one deck

A deck source is <srcdir>/deck.json ({"title", "order", "faces"}) plus one
<srcdir>/slides/<id>.html per slide, each holding one 1920x1080 <section>.
"""
import html as htmllib
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SLIDES_DIR = ROOT / "docs" / "slides"

ICONS = {
    "Activity": "📈", "Book": "📘", "Chart": "📊", "Chat": "💬", "Check": "✔", "CheckCircle": "✅",
    "Clock": "🕒", "Cloud": "☁", "Code": "⌨", "Database": "🗄", "Globe": "🌐", "GraduationCap": "🎓",
    "Home": "🏠", "Key": "🔑", "Lightbulb": "💡", "Lightning": "⚡", "Link": "🔗", "Lock": "🔒",
    "PaperPlane": "✈", "Play": "▶", "Search": "🔍", "Settings": "⚙", "Star": "★", "ThumbsUp": "👍",
    "Tool": "🛠", "Trust": "🤝", "Users": "👥", "Verified": "✔", "Warning": "⚠", "Wrench": "🔧",
}

CSS = """
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html, body { height: 100%; background: #0E1016; overflow: hidden; }
  #stage { position: absolute; left: 50%; top: 50%; width: 1920px; height: 1080px; transform-origin: center; }
  #stage > section { position: absolute; inset: 0; width: 1920px; height: 1080px; overflow: hidden; }
  #stage > section:not(.active) { display: none !important; }
  aside { display: none; }
  ul, ol { padding-left: 1.2em; }
  table { border-collapse: collapse; width: 100%; }
  th, td { text-align: left; padding: 0.35em 0.6em; border-bottom: 1px solid rgba(127,127,127,.3); vertical-align: top; }
  x-icon, x-shape { display: inline-block; flex: none; }
  x-shape[kind="arrow-right"] { clip-path: polygon(0 30%, 60% 30%, 60% 0, 100% 50%, 60% 100%, 60% 70%, 0 70%); }
  x-shape[kind="arrow-left"]  { clip-path: polygon(100% 30%, 40% 30%, 40% 0, 0 50%, 40% 100%, 40% 70%, 100% 70%); }
  x-shape[kind="arrow-down"]  { clip-path: polygon(30% 0, 70% 0, 70% 60%, 100% 60%, 50% 100%, 0 60%, 30% 60%); }
  x-shape[kind="arrow-up"]    { clip-path: polygon(30% 100%, 70% 100%, 70% 40%, 100% 40%, 50% 0, 0 40%, 30% 40%); }
  x-shape[kind="ellipse"]     { border-radius: 50%; }
  x-shape[kind="diamond"]     { clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%); }
  x-shape[kind="rounded"]     { border-radius: 16px; }
  x-shape[kind="line"]        { height: 0 !important; border-top: 3px solid currentColor; }
  #nav { position: fixed; right: 16px; bottom: 12px; color: #8A90A2; font: 14px sans-serif; }
  #nav a { color: #A9A7FF; }
  @media print {
    @page { size: 1920px 1080px; margin: 0; }
    html, body { overflow: visible; height: auto; background: none; }
    #stage { position: static; transform: none !important; width: auto; height: auto; }
    #stage > section, #stage > section:not(.active) { position: relative; display: flex !important; page-break-after: always; }
    #nav { display: none; }
  }
"""

SCRIPT = """
  const slides = [...document.querySelectorAll('#stage > section')];
  const icons = __ICONS__;
  document.querySelectorAll('x-icon').forEach(el => {
    const h = el.style.height || '40px';
    el.textContent = icons[el.getAttribute('name')] || '•';
    el.style.fontSize = (parseInt(h) || 40) * 0.8 + 'px'; el.style.lineHeight = h; el.style.textAlign = 'center';
  });
  let i = Math.max(0, Math.min(slides.length - 1, (parseInt(location.hash.slice(1)) || 1) - 1));
  function show(n) {
    i = Math.max(0, Math.min(slides.length - 1, n));
    slides.forEach((s, k) => s.classList.toggle('active', k === i));
    history.replaceState(null, '', '#' + (i + 1));
  }
  function fit() {
    const k = Math.min(innerWidth / 1920, innerHeight / 1080);
    document.getElementById('stage').style.transform = `translate(-50%, -50%) scale(${k})`;
  }
  addEventListener('resize', fit);
  addEventListener('hashchange', () => show((parseInt(location.hash.slice(1)) || 1) - 1));
  addEventListener('keydown', e => {
    if (['ArrowRight', 'PageDown', ' '].includes(e.key)) show(i + 1);
    else if (['ArrowLeft', 'PageUp'].includes(e.key)) show(i - 1);
    else if (e.key === 'Home') show(0);
    else if (e.key === 'End') show(slides.length - 1);
    else if (e.key === 'p') print();
  });
  addEventListener('click', e => {
    if (e.target.closest('a')) return;
    show(i + (e.clientX > innerWidth / 2 ? 1 : -1));
  });
  fit(); show(i);
"""


def build(deck_dir: pathlib.Path, out: pathlib.Path) -> int:
    deck = json.loads((deck_dir / "deck.json").read_text())
    sections = [(deck_dir / "slides" / f"{sid}.html").read_text().strip() for sid in deck["order"]]
    links = "\n".join(f'<link rel="stylesheet" href="{f["href"]}">' for f in deck["faces"].values())
    script = SCRIPT.replace("__ICONS__", json.dumps(ICONS, ensure_ascii=False))
    page = f"""<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{htmllib.escape(deck["title"])}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
{links}
<style>{CSS}</style>
</head>
<body>
<div id="stage">
{chr(10).join(sections)}
</div>
<div id="nav"><a href="index.html">一覧</a> · ← → / Space で移動 · P で印刷(PDF)</div>
<script>{script}</script>
</body>
</html>
"""
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(page)
    print(f"{out.relative_to(ROOT) if out.is_relative_to(ROOT) else out}: {len(sections)} slides")
    return len(sections)


def build_index(decks: list[tuple[str, str, int]]) -> None:
    rows = "\n".join(
        f'<li><a href="{name}.html"><span class="t">{htmllib.escape(title)}</span>'
        f'<span class="n">{count} 枚</span></a></li>'
        for name, title, count in decks
    )
    (SLIDES_DIR / "index.html").write_text(f"""<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>task-app スライド一覧</title>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;700&display=swap">
<style>
  :root {{ --bg: #F5F4EF; --ink: #1B1F2A; --sub: #4A5160; --card: #FCFBF8; --line: #DDDBD3; --accent: #5B58D6; }}
  @media (prefers-color-scheme: dark) {{
    :root {{ --bg: #1B1F2A; --ink: #F5F4EF; --sub: #B9BED0; --card: #2A3040; --line: #3A4152; --accent: #A9A7FF; }}
  }}
  body {{ margin: 0; background: var(--bg); color: var(--ink); font-family: 'Noto Sans JP', sans-serif; }}
  main {{ max-width: 720px; margin: 0 auto; padding: 48px 16px; }}
  h1 {{ font-size: 28px; margin: 0 0 8px; }}
  p {{ color: var(--sub); margin: 0 0 32px; }}
  ul {{ list-style: none; padding: 0; margin: 0; display: grid; gap: 12px; }}
  a {{ display: flex; justify-content: space-between; gap: 16px; padding: 20px 24px; background: var(--card);
       border: 1px solid var(--line); border-radius: 12px; color: var(--ink); text-decoration: none; }}
  a:hover {{ border-color: var(--accent); }}
  .n {{ color: var(--sub); white-space: nowrap; }}
</style>
</head>
<body>
<main>
<h1>task-app スライド</h1>
<p>AIエージェントと人間のタスク管理アプリの設計資料。← → / Space で移動、P で印刷・PDF 出力。</p>
<ul>
{rows}
</ul>
</main>
</body>
</html>
""")
    print(f"docs/slides/index.html: {len(decks)} decks")


ORDER = ["requirements", "architecture", "screens", "er", "plan"]


def main() -> None:
    if len(sys.argv) == 3:
        build(pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]))
        return
    if len(sys.argv) != 1:
        sys.exit(__doc__)
    src = SLIDES_DIR / "src"
    names = sorted((d.name for d in src.iterdir() if (d / "deck.json").exists()),
                   key=lambda n: (ORDER.index(n) if n in ORDER else len(ORDER), n))
    decks = []
    for name in names:
        count = build(src / name, SLIDES_DIR / f"{name}.html")
        title = json.loads((src / name / "deck.json").read_text())["title"]
        decks.append((name, title, count))
    build_index(decks)


if __name__ == "__main__":
    main()
