"""Generate docs/slides/src/m1/ (deck.json + slides/*.html): what M1 changed in the UI and the DB.

Screenshots live in docs/slides/img/m1/. Run, then `python3 scripts/build_slides.py`.
"""
import json
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE.parent / "docs" / "slides" / "src" / "m1"

_src = (HERE / "gen_devcycle_deck.py").read_text()
_helpers = _src[_src.index("ACC = "):_src.index("# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]
TEAL, ORANGE, GREEN, AMBER = "#2F7D8C", "#B85A1E", "#2E7D4F", "#9A6B00"


def dark(text):
    return f'<div style="background:{INK};color:{BG};border-radius:16px;padding:24px 34px;font-size:26px;line-height:1.6">{text}</div>'


def shot(name, w=1180, caption=""):
    return (f'<figure style="margin:0;display:flex;flex-direction:column;gap:10px"><img src="img/m1/{name}" alt="{caption}" '
            f'style="width:{w}px;border:1px solid {LINE};border-radius:12px;box-shadow:0 8px 24px rgba(0,0,0,.12)">'
            + (f'<figcaption style="font-size:20px;color:#7A8091">{caption}</figcaption>' if caption else "") + "</figure>")


def notes(items):
    return "".join(
        f'<div style="display:flex;gap:14px;padding:12px 0;border-bottom:1px solid {LINE}"><span style="flex:none;width:34px;height:34px;border-radius:50%;'
        f'background:{ACC};color:{BG};font-size:20px;font-weight:700;display:flex;align-items:center;justify-content:center">{i}</span>'
        f'<div><div style="font-size:25px;font-weight:700">{t}</div><div style="font-size:21px;line-height:1.5;color:{SUB}">{d}</div></div></div>'
        for i, (t, d) in enumerate(items, 1))


# 1 cover -------------------------------------------------------------------
slides.append(("cover", f"""<section id="cover" data-transition="fade" style="background:{INK};color:{BG};font-family:{SANS};padding:128px;display:flex;flex-direction:column;justify-content:space-between">
<div style="position:absolute;left:0;top:0;width:24px;height:1080px;background:{ACC}"></div>
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">DEV CYCLE · M1 IMPLEMENTED</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">M1 実装<br>UI と DB の変更点</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">要求の層（ペルソナ・ジョブ・ストーリーマップ・受け入れ条件）を案 A で追加した。<br>新しいテーブル 11、新しい画面 2、変わった画面 2</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">設計 docs/08-devcycle.md · ER docs/04-er-diagram.md</p>
<p style="font-size:28px;color:#B9BED0">2026-11-02 · 確認待ち</p>
</div>
</section>"""))

# 2 summary -----------------------------------------------------------------
frame("summary", "01 · まとめ", "DB に要求の層を足し、画面はストーリーマップを中心に 4 か所",
      f"""<div style="display:flex;gap:28px">
{card("DB（マイグレーション 009）", f"{b('新しいテーブル 11')}（req_*）。既存のテーブルは{b('1 列も変えていない')}。<br>イシューとは中間テーブル req_story_issues だけでつながる", extra="border-top:8px solid " + ACC)}
{card("UI", f"{b('新規')}：S15 ストーリーマップ・S16 ストーリー詳細（ペルソナ・ジョブのパネルを含む）<br>{b('変更')}：S06 イシュー一覧のヘッダーに入口、S09 イシュー詳細に「満たすストーリー」と「条件から写す」", color=TEAL, extra="border-top:8px solid " + TEAL)}
{card("API・MCP", f"{b('25 の操作')}を追加（/projects/{{key}}/storymap ほか）。イシュー詳細に stories。<br>MCP は get_storymap を追加、get_issue に「なぜ」が入る", color=ORANGE, extra="border-top:8px solid " + ORANGE)}
</div>
{table(["テスト", "件数", "増えた分"], [
    [b("API テスト（言語非依存）"), "201 件すべて成功", "+12（storymap.test.ts 9、契約 3）"],
    [b("画面テスト（Playwright）"), "38 件すべて成功", "+4（storymap.spec.ts）"],
    [b("Go の単体テスト"), "すべて成功", "+2（キーの解釈、カードの色の判定）"],
], ["34%", "28%", "38%"], size=25)}""", gap=32)

# 3 ER diagram --------------------------------------------------------------
BOXES = {
    # name: (x, y, new?, lines)
    "projects": (40, 290, False, ["既存", "全 req_* の project_id"]),
    "req_counters": (40, 560, True, ["project_id, kind, next", "キーの連番（使い回さない）"]),
    "users": (40, 830, False, ["既存", "変更した人"]),
    "req_personas": (420, 290, True, ["TAPP-P1 · name, kind", "human | agent"]),
    "req_criteria": (420, 560, True, ["TAPP-AC42", "given / when / then"]),
    "req_changes": (420, 830, True, ["entity_key, action, detail", "変更履歴（版の代わり）"]),
    "req_jobs": (800, 290, True, ["TAPP-J3", "situation / motivation / outcome"]),
    "req_stories": (800, 560, True, ["TAPP-ST12 · title", "i_want, so_that, release"]),
    "req_story_issues": (800, 830, True, ["story_id, issue_id", "N:N"]),
    "req_job_activities": (1180, 290, True, ["job_id, activity_id", "N:N"]),
    "req_story_steps": (1180, 560, True, ["story_id, step_id", "N:N"]),
    "issues": (1180, 830, False, ["既存（列は変更なし）", "API の詳細に stories"]),
    "req_activities": (1560, 290, True, ["TAPP-A1 · name", "persona_id"]),
    "req_steps": (1560, 560, True, ["TAPP-A1-2 · name", "activity_id"]),
    "acceptance_criteria": (1560, 830, False, ["既存（変更なし）", "受け入れ条件から写すだけ"]),
}
W, H = 300, 150


def er_box(name, x, y, new, lines):
    border = f"3px solid {ACC}" if new else "2px dashed #9AA0B0"
    bg = "#FFFFFF" if new else "#ECEBE4"
    badge = (f'<span style="position:absolute;right:10px;top:10px;font-size:14px;font-weight:700;color:{BG};background:{ACC};border-radius:6px;padding:1px 6px">新規</span>'
             if new else "")
    body = "".join(f'<div style="font-size:18px;line-height:1.45;color:{SUB}">{l}</div>' for l in lines)
    return (f'<div style="position:absolute;left:{x}px;top:{y}px;width:{W}px;height:{H}px;box-sizing:border-box;background:{bg};border:{border};'
            f'border-radius:12px;padding:14px 16px">{badge}<div style="font-family:{MONO};font-size:20px;font-weight:600;color:{INK};margin-bottom:8px">{name}</div>{body}</div>')


def mid(name, side):
    x, y, _, _ = BOXES[name]
    return {"l": (x, y + H / 2), "r": (x + W, y + H / 2), "t": (x + W / 2, y), "b": (x + W / 2, y + H)}[side]


EDGES = [
    ("projects", "r", "req_personas", "l", "1:N", False), ("projects", "b", "req_counters", "t", "1:N", False),
    ("req_personas", "r", "req_jobs", "l", "0..1:N", False), ("req_jobs", "r", "req_job_activities", "l", "1:N", False),
    ("req_job_activities", "r", "req_activities", "l", "N:1", False), ("req_activities", "b", "req_steps", "t", "1:N", False),
    ("req_steps", "l", "req_story_steps", "r", "1:N", False), ("req_story_steps", "l", "req_stories", "r", "N:1", False),
    ("req_stories", "l", "req_criteria", "r", "1:N", False), ("req_stories", "b", "req_story_issues", "t", "1:N", False),
    ("req_story_issues", "r", "issues", "l", "N:1", False), ("issues", "r", "acceptance_criteria", "l", "1:N", True),
    ("users", "r", "req_changes", "l", "1:N", False),
]
svg = []
labels = []
for a, sa, c, sc, lbl, old in EDGES:
    (x1, y1), (x2, y2) = mid(a, sa), mid(c, sc)
    color = "#9AA0B0" if old else ACC
    dash = ' stroke-dasharray="8 6"' if old else ""
    svg.append(f'<path d="M{x1:.0f} {y1:.0f} L{x2:.0f} {y2:.0f}" stroke="{color}" stroke-width="3"{dash} marker-end="url(#{"g" if old else "a"})"/>')
    lx, ly = (x1 + x2) / 2, (y1 + y2) / 2
    horizontal = abs(y1 - y2) < 1
    labels.append(f'<span style="position:absolute;left:{lx - 40 if horizontal else lx + 10:.0f}px;top:{ly - 30 if horizontal else ly - 14:.0f}px;width:80px;'
                  f'text-align:{"center" if horizontal else "left"};font-family:{MONO};font-size:15px;color:{SUB}">{lbl}</span>')
# persona → stories / activities ("誰の"), drawn dotted
(px, py) = (BOXES["req_personas"][0] + W * 0.75, BOXES["req_personas"][1] + H)
(sx, sy) = (BOXES["req_stories"][0] + W * 0.25, BOXES["req_stories"][1])
svg.append(f'<path d="M{px:.0f} {py:.0f} L{sx:.0f} {sy:.0f}" stroke="{ACC}" stroke-width="2.5" stroke-dasharray="3 6" marker-end="url(#a)"/>')
labels.append(f'<span style="position:absolute;left:{(px + sx) / 2 + 8:.0f}px;top:{(py + sy) / 2 - 12:.0f}px;font-size:16px;color:{SUB}">誰の（NULL 可）</span>')
svg.append(f'<path d="M{BOXES["req_personas"][0] + W / 2:.0f} {BOXES["req_personas"][1]:.0f} L{BOXES["req_personas"][0] + W / 2:.0f} 266 L{BOXES["req_activities"][0] + W / 2:.0f} 266 '
           f'L{BOXES["req_activities"][0] + W / 2:.0f} {BOXES["req_activities"][1]:.0f}" fill="none" stroke="{ACC}" stroke-width="2.5" stroke-dasharray="3 6" marker-end="url(#a)"/>')
labels.append(f'<span style="position:absolute;left:1040px;top:240px;font-size:16px;color:{SUB}">誰の（NULL 可）</span>')

er = (f'<svg style="position:absolute;left:0;top:0" width="1920" height="1080" viewBox="0 0 1920 1080">'
      f'<defs><marker id="a" markerWidth="10" markerHeight="10" refX="8" refY="5" orient="auto"><path d="M0 0 L10 5 L0 10 z" fill="{ACC}"/></marker>'
      f'<marker id="g" markerWidth="10" markerHeight="10" refX="8" refY="5" orient="auto"><path d="M0 0 L10 5 L0 10 z" fill="#9AA0B0"/></marker></defs>'
      + "".join(svg) + "</svg>" + "".join(er_box(n, *v) for n, v in BOXES.items()) + "".join(labels))
slides.append(("er", f"""<section id="er" data-transition="fade" style="background:{BG};color:{INK};font-family:{SANS};padding:72px 112px">
<p style="font-family:{MONO};font-size:24px;color:{ACC};letter-spacing:2px">02 · DB の変更点（ER 図）</p>
<h2 style="font-size:48px;font-weight:700;line-height:1.3;margin-top:8px">新規 11 テーブル（実線）。既存（点線）は列を変えず、つなぐだけ</h2>
{er}
<p style="position:absolute;left:112px;bottom:30px;font-size:22px;color:#7A8091">__PAGE__</p>
</section>"""))

# 4 tables ------------------------------------------------------------------
frame("tables", "03 · DB: 新しいテーブル", "キーはプロジェクト ＋ 種類 ＋ 連番。消さずに archived_at",
      table(["テーブル", "層", "役割", "主な列"], [
          [b("req_personas"), "L1", "ペルソナ（誰が使うか）", "num(TAPP-P1), name, kind human|agent, description"],
          [b("req_jobs"), "L2", "ジョブ（状況・したいこと・そうすれば）", "num(J3), persona_id, situation, motivation（必須）, outcome"],
          [b("req_activities"), "L3", "ストーリーマップの背骨", "num(A1), persona_id, name"],
          [b("req_steps"), "L4", "活動の手順", "activity_id, num（活動の中で: A1-2）, name"],
          [b("req_stories"), "L5", "ストーリー。完了で閉じない", "num(ST12), title, persona_id, i_want, so_that, release"],
          [b("req_criteria"), "L6", "受け入れ条件", "story_id, num(AC42), given_text, when_text, then_text（必須）"],
          [b("req_job_activities"), "—", "ジョブ ⇄ 活動", "job_id, activity_id"],
          [b("req_story_steps"), "—", "ストーリー ⇄ 手順（マップ上の置き場所）", "story_id, step_id"],
          [b("req_story_issues"), "—", "ストーリー ⇄ イシュー（同じプロジェクト）", "story_id, issue_id"],
          [b("req_counters"), "—", "キーの連番", "project_id, kind, next"],
          [b("req_changes"), "—", "変更履歴（誰が・いつ・何を）", "entity_key, story_id, actor_id, action, detail(JSON)"],
      ], ["24%", "6%", "32%", "38%"], size=21)
      + f'<p style="font-size:22px;color:{SUB}">全テーブルに position・created_at・updated_at（中間テーブルと履歴以外）。エクスポート（JSON）にも親から順に入る</p>', gap=22)

# 5 rules -------------------------------------------------------------------
frame("rules", "04 · DB: 決めたこと", "要求は消さない・閉じない。色は保存せず、つながりから計算する",
      f"""<div style="display:flex;gap:44px">
<div style="flex:1">{notes([
    ("アーカイブだけ", "DELETE は archived_at を入れる。活動をアーカイブすると手順も。マップ・抜け・つながりの一覧から消える"),
    ("キーは使い回さない", "req_counters で採番。TAPP-ST2 をアーカイブしても次は TAPP-ST3。手順は活動ごとの連番"),
    ("イシューは同じプロジェクトだけ", "別プロジェクトのイシューをつなごうとすると 400。PUT は何度呼んでも同じ（履歴も 1 回）"),
])}</div>
<div style="flex:1">{notes([
    ("カードの色（status）", "条件なし = draft、条件あり・作業前 = ready、イシューが進行中 = in_progress、すべて完了/キャンセル（1 件以上完了）= done"),
    ("履歴が版の代わり", "作成・変更（変えた項目）・アーカイブ・つなぐ/外す。ストーリー詳細に新しい順で 50 件"),
    ("受入基準とは別物", "イシューの acceptance_criteria は今のまま。「写す」ボタンで【前提】【操作】【結果】の 1 行にして足す。同期はしない"),
])}</div>
</div>""", gap=28)

# 6 UI overview -------------------------------------------------------------
frame("ui", "05 · UI の変更点", "新しい画面 2 つと、既存の画面 2 か所",
      table(["画面", "区分", "変わったこと"], [
          [b("S15 ストーリーマップ"), '<span style="color:#2E7D4F;font-weight:700">新規</span>', "/p/:key/storymap。横に活動と手順、縦にリリース。カードの色・抜けの一覧・ペルソナで絞り込み。活動・手順・ストーリーはその場で追加"],
          [b("S16 ストーリー詳細"), '<span style="color:#2E7D4F;font-weight:700">新規</span>', "S15 の右のサイドピーク。ペルソナ・したいこと・リリース・手順・受け入れ条件（Given/When/Then、並べ替え）・イシューのつなぎ外し・変更履歴"],
          [b("ペルソナとジョブ"), '<span style="color:#2E7D4F;font-weight:700">新規</span>', "S15 の左のパネル（別の画面は作らない）。ジョブは活動のボタンでつなぐ"],
          [b("S06 イシュー一覧"), f'<span style="color:{AMBER};font-weight:700">変更</span>', "プロジェクトのヘッダーに「ストーリーマップ」への入口"],
          [b("S09 イシュー詳細"), f'<span style="color:{AMBER};font-weight:700">変更</span>', "「満たすストーリー」（このイシューが何のためにあるか）。ストーリー/フィーチャーの受入基準に「ストーリーの条件から写す」"],
      ], ["22%", "9%", "69%"], size=23)
      + f'<div style="display:flex;gap:16px;align-items:center"><span style="font-size:21px;color:{SUB};white-space:nowrap">S06 のヘッダー：</span>{shot("s06-header.png", 1420)}</div>', gap=26)


# 7-10 screenshots ----------------------------------------------------------
def shot_slide(sid, kicker, title, img, side):
    frame(sid, kicker, title, f"""<div style="display:flex;gap:36px;align-items:flex-start">
{shot(img, 1180)}
<div style="flex:1">{notes(side)}</div>
</div>""", gap=26)


shot_slide("s15", "06 · S15 ストーリーマップ（新規）", "手順の下にストーリー、リリースごとの行、色で進み具合", "s15-map.png", [
    ("色", "灰 = 条件なし、白 = 着手可能、黄 = 実装中、緑 = 完了。凡例はヘッダー"),
    ("抜け", "手順のないストーリー、ストーリーのない手順など 5 種類を一覧"),
    ("その場で追加", "＋ 活動・＋ 手順・セルの ＋ ストーリー・＋ リリース"),
])
shot_slide("s16", "07 · S16 ストーリー詳細（新規）", "ストーリーから条件・イシュー・履歴まで縦につながる", "s16-story.png", [
    ("受け入れ条件", "Given / When / Then を 1 件ずつ。Then は必須。↑↓ で並べ替え"),
    ("開発タスク", "イシューキー（番号だけでも可）でつなぐ。クリックでイシューを開く"),
    ("変更履歴", "誰が・いつ・何を変えたか"),
])
shot_slide("panel", "08 · ペルソナとジョブ（新規）", "マップの左に開くパネルで編集する", "panel.png", [
    ("ペルソナ", "人 / AI。名前と説明はクリックで編集"),
    ("ジョブ", "状況・したいこと・そうすれば。活動のボタンでつなぐ"),
    ("絞り込み", "ヘッダーのペルソナを選ぶと、その人の活動とストーリーだけ"),
])
shot_slide("s09", "09 · S09 イシュー詳細（変更）", "イシューが「何のためにあるか」を表示する", "s09-issue.png", [
    ("満たすストーリー", "ストーリーのしたいこと・そうすれば・受け入れ条件。キーでマップへ"),
    ("条件から写す", "受入基準に【前提】【操作】【結果】として足す（同期しない）"),
    ("AI にも同じ", "MCP の get_issue に stories が入る"),
])

# 11 API --------------------------------------------------------------------
frame("api", "10 · API と MCP の変更点", "25 の操作を追加。既存の API は stories が増えただけ",
      table(["パス（/api/v1/projects/{key}/…）", "操作"], [
          [b("storymap"), "GET：マップ全体と抜け（gaps）"],
          ["personas · jobs", "GET 一覧・POST 作成、{キー} に PATCH・DELETE（アーカイブ）"],
          ["activities、activities/{キー}/steps、steps/{キー}", "POST・PATCH・DELETE"],
          ["stories、stories/{キー}", "GET 一覧・POST、GET（条件・履歴つき）・PATCH・DELETE"],
          ["stories/{キー}/criteria、criteria/{キー}", "POST・PATCH・DELETE"],
          ["stories/{キー}/issues/{イシューキー}", "PUT つなぐ・DELETE 外す"],
          ["/api/v1/issues/{キー}（既存）", "stories（キー・タイトル・したいこと・条件）を追加。一覧には入れない"],
      ], ["48%", "52%"], size=23)
      + f"""<div style="display:flex;gap:28px">
{card("キーの書き方", "TAPP-ST12 でも ST12 でも受け付ける。他プロジェクトのキーは 404")}
{card("MCP", "get_storymap（読み取り）を追加。get_issue の説明と結果に「なぜ」（ストーリー・条件）", color=TEAL)}
</div>""", gap=24)

# 12 next -------------------------------------------------------------------
qs = [("1. 画面と DB の形", "この ER 図・画面でよいか。直したいところがあれば、本番に入れる前に"),
      ("2. 本番への反映", "PR を出してマージ後にデプロイしてよいか。デプロイ後、08 のペルソナ・ジョブ・マップを本番の TAPP に登録する"),
      ("3. 最初のリリース R1", "マイタスクの整理 T1〜T5 を TAPP のストーリーとして置き、M1 の仕組みで回してよいか")]
frame("next", "11 · 確認したいこと", "まだ PR は出していない（ブランチ docs/devcycle-proposal）",
      f"""<div style="display:flex;flex-direction:column;gap:20px;flex:1">{''.join(card(t, d) for t, d in qs)}</div>""", gap=30)

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "M1 実装: UI と DB の変更点",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
