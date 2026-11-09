"""Generate docs/slides/src/req-d/ (deck.json + slides/*.html): option E "A と D をプロジェクトで選ぶ" as implemented.

Source of the content: docs/09-requirements-loose.md §6 and docs/checkpoints/extra-15.md. Screenshots live in
docs/slides/img/req-e/. Run, then `python3 scripts/build_slides.py`.
"""
import json
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE.parent / "docs" / "slides" / "src" / "req-e"

_src = (HERE / "gen_devcycle_deck.py").read_text()
_helpers = _src[_src.index("ACC = "):_src.index("# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]
TEAL, ORANGE, GREEN, AMBER, GREY = "#2F7D8C", "#B85A1E", "#2E7D4F", "#9A6B00", "#9AA0B0"


def shot(name, w=1180, caption="", folder="req-e"):
    return (f'<figure style="margin:0;display:flex;flex-direction:column;gap:10px"><img src="img/{folder}/{name}" alt="{caption}" '
            f'style="width:{w}px;border:1px solid {LINE};border-radius:12px;box-shadow:0 8px 24px rgba(0,0,0,.12)">'
            + (f'<figcaption style="font-size:20px;color:#7A8091">{caption}</figcaption>' if caption else "") + "</figure>")


def notes(items):
    return "".join(
        f'<div style="display:flex;gap:14px;padding:12px 0;border-bottom:1px solid {LINE}"><span style="flex:none;width:34px;height:34px;border-radius:50%;'
        f'background:{ACC};color:{BG};font-size:20px;font-weight:700;display:flex;align-items:center;justify-content:center">{i}</span>'
        f'<div><div style="font-size:25px;font-weight:700">{t}</div><div style="font-size:21px;line-height:1.5;color:{SUB}">{d}</div></div></div>'
        for i, (t, d) in enumerate(items, 1))


def tag(text, color):
    return f'<span style="display:inline-block;font-size:18px;font-weight:700;color:{BG};background:{color};border-radius:6px;padding:2px 10px">{text}</span>'


def before_after(sid, kicker, title, before, after, note):
    frame(sid, kicker, title, f"""<div style="display:flex;gap:32px;align-items:flex-start">
<div style="display:flex;flex-direction:column;gap:10px">{tag("前（M1）", GREY)}{shot(before, 820, folder="m1")}</div>
<div style="display:flex;flex-direction:column;gap:10px">{tag("後（案 D）", ACC)}{shot(after, 820)}</div>
</div>
<div style="font-size:23px;line-height:1.6;color:{SUB}">{note}</div>""", gap=24)


def shot_slide(sid, kicker, title, img, side):
    frame(sid, kicker, title, f"""<div style="display:flex;gap:36px;align-items:flex-start">
{shot(img, 1180)}
<div style="flex:1">{notes(side)}</div>
</div>""", gap=26)


# 1 cover -------------------------------------------------------------------
slides.append(("cover", f"""<section id="cover" data-transition="fade" style="background:{INK};color:{BG};font-family:{SANS};padding:128px;display:flex;flex-direction:column;justify-content:space-between">
<div style="position:absolute;left:0;top:0;width:24px;height:1080px;background:{ACC}"></div>
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">REQUIREMENTS · OPTION E IMPLEMENTED</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">案 E 実装<br>A と D をプロジェクトで選ぶ</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">データは 1 つ、画面は 2 つ。<br>プロジェクトの設定「要求の書き方」で、まず書く（quick）と整理して書く（structured）を切り替える</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">ブランチ exp/req-e · task-app-next/e-both</p>
<p style="font-size:28px;color:#B9BED0">2026-11-02 · 確認待ち</p>
</div>
</section>"""))

# 2 why it works --------------------------------------------------------------
frame("why", "01 · なぜ両立できるか", "A と D の違いは、テーブルではなく画面と「抜け」の見せ方だけ",
      f"""<div style="display:flex;gap:28px">
{card("同じデータ", f"案 D は M1 の 11 テーブルに {b('req_stories.body')} を 1 列足しただけ。ストーリー・条件・置き場所の持ち方は同じ", extra="border-top:8px solid " + ACC)}
{card("違うのは画面", f"書き始め（ひとこと／マスの ＋）・条件（1 行／3 欄）・ペルソナ（使うときだけ／いつも）・抜け（数／警告）", color=TEAL, extra="border-top:8px solid " + TEAL)}
{card("だから設定 1 つ", f"プロジェクトに {b('requirements_mode')} を持たせ、S15・S16 をそれで切り替える。切り替えてもデータは変わらない", color=ORANGE, extra="border-top:8px solid " + ORANGE)}
</div>
{table(["テスト", "件数", "増えた分"], [
    [b("API テスト（言語非依存）"), "210 件すべて成功", "+4（requirements-mode.test.ts）"],
    [b("画面テスト（Playwright）"), "43 件すべて成功", "+2（storymap-mode.spec.ts）"],
    [b("Go の単体テスト"), "すべて成功", "—"],
], ["34%", "28%", "38%"], size=25)}""", gap=32)

# 3 setting -------------------------------------------------------------------
shot_slide("setting", "02 · 設定", "プロジェクト設定の「要求の書き方」", "e1-settings.png", [
    ("まず書く（quick）", "案 D の画面。新しいプロジェクトの既定"),
    ("整理して書く（structured）", "案 A の画面。前からあるプロジェクトはこちらのまま"),
    ("どこから", "ストーリーマップのヘッダーのモード表示を押すと、ここへ来る"),
])

# 4 compare -------------------------------------------------------------------
frame("compare", "03 · 何が変わるか", "まず書く と 整理して書く",
      table(["", "まず書く（quick）", "整理して書く（structured）"], [
          [b("書き始め"), "マップの上のひとこと入力 → 未整理の列", "手順 × リリースのマスの ＋"],
          [b("手順のないもの"), "最初の列「未整理」", "マップの上に「⚠ 未整理 N件」（抜け）"],
          [b("本文"), "いつも出る", "書いてあるときだけ（消えない）"],
          [b("したいこと・そうすれば"), "「項目を足す」で", "いつも出る"],
          [b("受け入れ条件"), "1 行。前提・操作は「詳しく」", "いつも 3 欄"],
          [b("ペルソナ"), "使うときだけ", "いつも出る"],
          [b("抜け"), "数だけ", "警告の色"],
          [b("AI（get_storymap）"), "mode: quick。1 行でよい", "mode: structured。手順の下に、3 欄で"],
      ], ["22%", "39%", "39%"], size=23), gap=28)

# 5 quick -----------------------------------------------------------------------
shot_slide("quick", "04 · まず書く", "案 D の画面そのまま", "e2-quick.png", [
    ("ひとこと入力", "Enter で未整理の列へ"),
    ("本文・1 行の条件", "S16 はタイトルと本文から"),
    ("抜け", "色を付けず数だけ"),
])

# 6 structured ------------------------------------------------------------------
shot_slide("structured", "05 · 整理して書く", "同じデータを案 A の画面で", "e3-structured.png", [
    ("未整理 N件", "手順のないストーリーはマップの上に、直すものとして出る"),
    ("抜け", "警告の色（橙）"),
    ("ペルソナ", "絞り込みがいつも出る"),
])

# 7 structured story -------------------------------------------------------------
shot_slide("story", "06 · 整理して書く の S16", "まず書く で書いたストーリーを開くと", "e4-structured-story.png", [
    ("本文", "書いてあるので出る。モードを変えても消えない"),
    ("手順が未設定（抜け）", "置き場所を決めるよう促す"),
    ("条件は 3 欄", "1 行で書いた条件は、前提・操作が「—」"),
])

# 8 api -----------------------------------------------------------------------
frame("api", "07 · DB・API・MCP の変更点", "列 1 つ、項目 2 つ",
      table(["どこ", "変更"], [
          [b("projects"), "requirements_mode TEXT（structured | quick）。マイグレーション 011。列の既定は structured（前からあるプロジェクト）"],
          [b("POST /projects"), "requirements_mode を指定できる。指定がなければ quick"],
          [b("PATCH /projects/{key}"), "requirements_mode を変えられる。ほかの値は 400"],
          [b("GET …/storymap・MCP get_storymap"), "mode を返す"],
          [b("MCP の説明"), "structured のプロジェクトでは、手順の下に置き、前提・操作・結果を埋め、抜けを直すものとして扱う"],
          [b("決まり"), "ストーリーと条件の API はモードで変えない（structured でもタイトルだけを受け付け、抜けに出す）"],
      ], ["30%", "70%"], size=24), gap=30)

# 10-12 release slices (§5.8) --------------------------------------------------
frame("rl-why", "08 · リリースのまとまり", "地図の横線に、ねらいと状態を持たせる",
      f"""<div style="display:flex;gap:28px">
{card("まとまりを持つ", f"{b('req_releases')}（マイグレーション 012）: 名前・{b('ねらい')}・状態（計画中 / 作っている / 出した）・目安・並び。キーは TAPP-RL1", extra="border-top:8px solid " + ACC)}
{card("ストーリーは名前で", f"ストーリーの release は今のまま文字列。{b('名前を変えるとストーリーも付いてくる')}。新しい名前を書けばまとまりができる", color=TEAL, extra="border-top:8px solid " + TEAL)}
{card("次に出す", f"いちばん上の出していないまとまりに{b('次に出す')}。その上に線を端まで引く（ウォーキングスケルトン）", color=ORANGE, extra="border-top:8px solid " + ORANGE)}
</div>
{table(["どこ", "変更"], [
    [b("REST"), "GET/POST /projects/{key}/releases、PATCH/DELETE /projects/{key}/releases/{rkey}（ストーリーがあれば 409）"],
    [b("storymap"), "releases（名前）が並び順に。release_slices にねらい・状態・n/m 完了"],
    [b("MCP"), "get_storymap に release_slices、update_release（名前で探し、なければ作る）"],
    [b("テスト"), "API 216 件（+6 releases.test.ts）、画面 46 件（+2 storymap-releases.spec.ts）すべて成功"],
], ["18%", "82%"], size=23)}""", gap=30)

shot_slide("rl-quick", "09 · まず書く の地図", "行の見出しに、ねらい・状態・進み具合", "e5-releases-quick.png", [
    ("R1 次に出す", "ねらい「ログインして、ORCA から自分のタスクを読める」。作っている・1/3 完了・目安"),
    ("線", "次に出す の上の線は地図の端まで"),
    ("見出しで直す", "名前・ねらいはクリックで、状態は選んで。上下に動かせる"),
])

shot_slide("rl-structured", "10 · 整理して書く の地図", "同じまとまりを、整理して書く でも", "e6-releases-structured.png", [
    ("同じデータ", "まとまりはモードに関係ない"),
    ("決めたこと", "知らない名前でまとまりができる・状態は人が決める・空のときだけ消せる"),
    ("まだ", "出したものを畳む表示、ドラッグ、S16 で選ぶ形"),
])

# 12 next ------------------------------------------------------------------
frame("next", "11 · 制限と確認したいこと", "モードはプロジェクト単位",
      f"""<div style="display:flex;gap:28px">
<div style="flex:1">{notes([
    ("人ごとではない", "同じプロジェクトを人によって違う画面で見る切り替えはない"),
    ("API はゆるいまま", "structured でもタイトルだけのストーリーを作れる（抜けに出る）"),
    ("3 欄の抜け", "前提・操作が空の条件は抜けの一覧に入れていない"),
])}</div>
<div style="flex:1;display:flex;flex-direction:column;gap:18px">
{card("1. 既定 → 回答済み", "既定は OK。作るときに明示的に選ぶ形にした", color=GREY, extra="padding:20px 28px")}
{card("2. API → 回答済み", "structured でもタイトルだけを受け付け、抜けとして見せる（問題があれば後で）", color=GREY, extra="padding:20px 28px")}
{card("3. リリースのまとまり", "決めたこと（知らない名前でできる・状態は人）でよいか。S16 で選ぶ形にするか", color=TEAL, extra="padding:20px 28px")}
</div></div>""", gap=28)

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "案 E 実装: A と D をプロジェクトで選ぶ",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
