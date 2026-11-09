"""Generate docs/slides/src/req-draft/ (deck.json + slides/*.html): drafts of a story map, imported in one go.

Source of the content: docs/08-devcycle.md §5.9 and docs/checkpoints/extra-17.md. Screenshots live in
docs/slides/img/req-draft/. Run, then `python3 scripts/build_slides.py`.
"""
import json
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE.parent / "docs" / "slides" / "src" / "req-draft"

_src = (HERE / "gen_devcycle_deck.py").read_text()
_helpers = _src[_src.index("ACC = "):_src.index("# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]
TEAL, ORANGE, GREEN, AMBER, GREY, RED = "#2F7D8C", "#B85A1E", "#2E7D4F", "#9A6B00", "#9AA0B0", "#C0392B"


def shot(name, w=1180, caption=""):
    return (f'<figure style="margin:0;display:flex;flex-direction:column;gap:10px"><img src="img/req-draft/{name}" alt="{caption}" '
            f'style="width:{w}px;border:1px solid {LINE};border-radius:12px;box-shadow:0 8px 24px rgba(0,0,0,.12)">'
            + (f'<figcaption style="font-size:20px;color:#7A8091">{caption}</figcaption>' if caption else "") + "</figure>")


def notes(items):
    return "".join(
        f'<div style="display:flex;gap:14px;padding:12px 0;border-bottom:1px solid {LINE}"><span style="flex:none;width:34px;height:34px;border-radius:50%;'
        f'background:{ACC};color:{BG};font-size:20px;font-weight:700;display:flex;align-items:center;justify-content:center">{i}</span>'
        f'<div><div style="font-size:25px;font-weight:700">{t}</div><div style="font-size:21px;line-height:1.5;color:{SUB}">{d}</div></div></div>'
        for i, (t, d) in enumerate(items, 1))


def shot_slide(sid, kicker, title, img, side):
    frame(sid, kicker, title, f"""<div style="display:flex;gap:36px;align-items:flex-start">
{shot(img, 1180)}
<div style="flex:1">{notes(side)}</div>
</div>""", gap=26)


def step(n, title, text, color):
    return (f'<div style="flex:1;display:flex;flex-direction:column;gap:10px;background:{CARD};border:1px solid {LINE};border-radius:16px;padding:24px 26px;border-top:8px solid {color}">'
            f'<div style="font-family:{MONO};font-size:22px;color:{color}">{n}</div><h3 style="font-size:30px;font-weight:700">{title}</h3>'
            f'<div style="font-size:22px;line-height:1.55;color:{SUB}">{text}</div></div>')


# 1 cover -------------------------------------------------------------------
slides.append(("cover", f"""<section id="cover" data-transition="fade" style="background:{INK};color:{BG};font-family:{SANS};padding:128px;display:flex;flex-direction:column;justify-content:space-between">
<div style="position:absolute;left:0;top:0;width:24px;height:1080px;background:{ACC}"></div>
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">REQUIREMENTS · STORY MAP DRAFTS</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">下書きで試して<br>まとめて取り込む</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">ストーリーマップは、試行錯誤してから決まる。<br>下書きのプロジェクトで自由に組み替え、予定を見てから一度に入れる</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">ブランチ feat/req-drafts · 本体 task-app</p>
<p style="font-size:28px;color:#B9BED0">2026-11-03 · 確認待ち</p>
</div>
</section>"""))

# 2 why ---------------------------------------------------------------------
frame("why", "01 · なぜ", "ブレストの途中を、本番のマップに混ぜない",
      f"""<div style="display:flex;gap:28px">
{card("いまの困りごと", f"マップを組み替えながら考えると、{b('途中の形がそのまま本番に出る')}。AI も途中を読む。やめた案を消し忘れる", color=ORANGE, extra="border-top:8px solid " + ORANGE)}
{card("やりたいこと", f"一時的な{b('試しの場所')}で決めて、決まったら{b('一気に入れる')}", extra="border-top:8px solid " + ACC)}
{card("選んだ案", f"4 案（下書きのプロジェクト / マップの中の下書き / テキストで読み込む / 「仮」の印）から {b('下書きのプロジェクト')}。今の画面と道具がそのまま使える", color=TEAL, extra="border-top:8px solid " + TEAL)}
</div>
{table(["案", "よい点", "選ばなかった理由"], [
    [b("1 下書きのプロジェクト ✓"), "新しいのは取り込みだけ。何度でも壊して作り直せる", "—"],
    [b("2 マップの中の下書き"), "いちばん自然（本番と並べて見られる）", "すべての表に下書きの状態が要り、重い"],
    [b("3 テキストで読み込む"), "AI との会話・git に向く", "カードを動かしながら考えられない"],
    [b("4 「仮」の印"), "いちばん小さい", "本番と混ざる。消し忘れる"],
], ["26%", "40%", "34%"], size=22)}""", gap=28)

# 3 flow --------------------------------------------------------------------
frame("flow", "02 · 流れ", "作る → 試す → 予定を見る → 取り込む",
      f"""<div style="display:flex;gap:22px">
{step("1 作る", "下書き", f"マップのヘッダー「下書き」から。{b('いまのマップの複製')}（既定）か空。キーは TAPPD1、番号はそのまま（TAPP-ST3 → TAPPD1-ST3）", ACC)}
{step("2 試す", "自由に", f"足す・消す・名前を変える・置き直す。{b('元のプロジェクトは変わらない')}。AI も MCP で下書きを直せる", TEAL)}
{step("3 見る", "取り込みの予定", f"{b('増える・変わる・消える・戻す')}。両方で変わったものは衝突。元にしかないものは残る", AMBER)}
{step("4 入れる", "一度に", f"{b('人だけ')}が、1 つのトランザクションで。下書きはそのまま使え、次は違いだけが入る", GREEN)}
</div>
<div style="font-size:24px;line-height:1.6;color:{SUB}">下書きは{b('プロジェクト一覧・サイドバーには出ない')}。元のマップの「下書き」メニューから開く。いらなくなったら「下書きを捨てる」（アーカイブ）。</div>""", gap=32)

# 4-8 screens ---------------------------------------------------------------
shot_slide("menu", "03 · 下書きを作る", "元のマップのヘッダーに「下書き」", "d1-menu.png", [
    ("いまのマップから", "複製して始める（既定）。イシューとのつながりは複製しない"),
    ("空から", "何もないところから。同じ名前のリリースは元のものとみなす"),
    ("一覧", "作った下書きがここに並ぶ（新しい順）"),
])
shot_slide("draft", "04 · 下書きで試す", "帯が出ている間は、元は変わらない", "d2-draft.png", [
    ("帯", "「APP に取り込む前の試し場所」。下書きを捨てる・APP に取り込む"),
    ("いつもの画面", "書き方の既定は まず書く。ひとこと入力・未整理・リリースの見出しもそのまま"),
    ("この例", "ストーリーを 2 つ足し、R2 を「つぎ」に、1 つ消し、タイトルを直した"),
])
shot_slide("plan", "05 · 取り込みの予定", "何が起きるかを、取り込む前に全部見る", "d3-plan.png", [
    ("増える", "キーは取り込むときに付く（予定では「新規（APPD1-ST8）」）"),
    ("変わる", "項目ごとに 前 → 後。R2 → つぎ は、ストーリーがみんな付いていく"),
    ("消える", "アーカイブする（キーは使い回さない）"),
])
shot_slide("conflict", "06 · 衝突", "元でも変わっていたら、上書きを明示する", "d5-conflict.png", [
    ("3 者比較", "最後にそろったときの中身（ハッシュ）と、いまの下書き・いまの元を比べる。触っていない要素の基準は取り込みで動かさない"),
    ("下書きだけ変えた", "変わる。元だけ変えた → 何もしない（元のまま）"),
    ("両方変えた", "印を付け、「上書きして取り込む」に印を付けたときだけ取り込める（API は 409、force）"),
])
shot_slide("done", "07 · 取り込んだあと", "下書きは続けて使える", "d4-done.png", [
    ("新しいキー", "APP-ST8・APP-ST9 のように、元の続きの番号が付く"),
    ("もう一度", "取り込むと、その後の違いだけが入る"),
    ("履歴", "元の変更履歴に from_draft: APPD1 付きで残る"),
])

# 9 data/API ----------------------------------------------------------------
frame("api", "08 · データと API", "プロジェクト 1 列と、対応表 1 つ",
      table(["どこ", "変更"], [
          [b("projects.draft_of"), "取り込み先のプロジェクト（マイグレーション 013）。GET /projects には下書きを出さない"],
          [b("req_draft_links"), "下書きの要素 ↔ 取り込み先の要素。最後にそろったときの中身のハッシュを両側に持つ"],
          [b("GET/POST /projects/{key}/drafts"), "下書きの一覧と作成（from: copy | empty、key、name）"],
          [b("GET /projects/{draft}/import-plan"), "items（create / update / archive / restore、changes、conflict）・target_only・counts・warnings"],
          [b("POST /projects/{draft}/import"), "人だけ（エージェントは 403）。衝突があれば 409、force: true で上書き"],
          [b("MCP"), "create_draft・get_import_plan。説明に「大きく考え直すときは下書きで」"],
          [b("テスト"), "API 229 件（+13 drafts.test.ts）、画面 48 件（+2 storymap-drafts.spec.ts）すべて成功"],
      ], ["30%", "70%"], size=23), gap=30)

# 10 MCP map tools (A-19) ---------------------------------------------------
frame("mcp", "09 · MCP でマップを作る", "ペルソナ・活動・手順も AI が入れられる",
      table(["道具", "すること"], [
          [b("create_persona / update_persona"), "ペルソナを足す・直す（名前・human / agent・説明・並び）"],
          [b("create_activity"), "活動を足す。手順をまとめて渡せる（steps: [{name}, …]）。persona_key で誰の活動か"],
          [b("update_activity"), "名前・説明・並び。persona_key: \"\" でペルソナを外す"],
          [b("create_step / update_step"), "手順を活動の最後に足す・名前と並びを直す"],
          [b("create_story"), "persona_key・i_want・so_that を足した（手順の下に、誰の何のための話かまで一度に）"],
          [b("消す道具は足さない"), "消すのは画面か、下書きで消してから取り込む（元ではアーカイブ）"],
          [b("テスト"), "API 232 件（+3 mcp-storymap.test.ts）すべて成功"],
      ], ["34%", "66%"], size=23), gap=30)

# 10 next -------------------------------------------------------------------
frame("next", "10 · 制限と確認したいこと", "全部まとめて入れる形から始めた",
      f"""<div style="display:flex;gap:28px">
<div style="flex:1">{notes([
    ("選んで取り込めない", "予定に出たものは全部入る（入れないものは下書きで戻す）"),
    ("取り消しはない", "変更履歴には残る"),
    ("下書き同士は比べない", "下書きは何個でも作れる"),
    ("途中で見つけて直した", "リリース名の入れ替え（R1 ⇔ R2）、あいだの取り込みで衝突が消える誤り、ヘッダーのメニューが切れる表示"),
])}</div>
<div style="flex:1;display:flex;flex-direction:column;gap:18px">
{card("1. 決めたこと", "人だけが取り込む・消したものはアーカイブ・衝突は上書きを明示、でよいか", extra="padding:20px 28px")}
{card("2. 選んで取り込む", "項目ごとに選ぶ形が要るか", color=TEAL, extra="padding:20px 28px")}
{card("3. 本番へ", "push・PR・デプロイは指示をもらってから", color=GREY, extra="padding:20px 28px")}
</div></div>""", gap=28)

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "下書きで試して、まとめて取り込む",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
