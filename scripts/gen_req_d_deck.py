"""Generate docs/slides/src/req-d/ (deck.json + slides/*.html): option D "M1 をゆるく使う" as implemented.

Source of the content: docs/09-requirements-loose.md §3 and docs/checkpoints/extra-14.md. Screenshots live in
docs/slides/img/req-d/ (after) and docs/slides/img/m1/ (before). Run, then `python3 scripts/build_slides.py`.
"""
import json
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE.parent / "docs" / "slides" / "src" / "req-d"

_src = (HERE / "gen_devcycle_deck.py").read_text()
_helpers = _src[_src.index("ACC = "):_src.index("# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]
TEAL, ORANGE, GREEN, AMBER, GREY = "#2F7D8C", "#B85A1E", "#2E7D4F", "#9A6B00", "#9AA0B0"


def shot(name, w=1180, caption="", folder="req-d"):
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
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">REQUIREMENTS · OPTION D IMPLEMENTED</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">案 D 実装<br>M1 をゆるく使う</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">テーブルは M1 のまま、列を 1 つ足しただけ。<br>書くときの決まりと画面をゆるめ、整理は後から AI と</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">ブランチ exp/req-d · task-app-next/d-loose</p>
<p style="font-size:28px;color:#B9BED0">2026-11-02 · 確認待ち</p>
</div>
</section>"""))

# 2 summary -----------------------------------------------------------------
frame("summary", "01 · まとめ", "DB は 1 列、画面は 3 か所、MCP に 2 つの道具",
      f"""<div style="display:flex;gap:28px">
{card("DB（マイグレーション 010）", f"{b('req_stories.body')} を 1 列足しただけ。ほかの 10 テーブルは M1 のまま", extra="border-top:8px solid " + ACC)}
{card("UI", f"S15 に{b('ひとこと入力')}と{b('未整理')}の列。S16 に{b('本文')}と{b('1 行の条件')}。ペルソナ・活動は{b('使うときだけ')}", color=TEAL, extra="border-top:8px solid " + TEAL)}
{card("MCP", f"{b('create_story')}・{b('update_story')}。AI は get_storymap で読み、ask_question で聞き、はいだけ直す", color=ORANGE, extra="border-top:8px solid " + ORANGE)}
</div>
{table(["テスト", "件数", "増えた分"], [
    [b("API テスト（言語非依存）"), "206 件すべて成功", "+5（storymap-loose.test.ts）"],
    [b("画面テスト（Playwright）"), "41 件すべて成功", "+3（storymap-loose.spec.ts）。M1 の 4 件は手順を合わせて直した"],
    [b("Go の単体テスト"), "すべて成功", "—"],
], ["34%", "28%", "38%"], size=25)}""", gap=32)

# 3 D1-D5 -------------------------------------------------------------------
frame("changes", "02 · 何を変えたか", "固さ K3・K4・K6 を、書く人から見えなくした",
      table(["#", "M1（前）", "案 D（後）", "固さ"], [
          [b("D1"), "ストーリーはセルの ＋ から。先に活動と手順が要る", f"マップの上の{b('ひとこと入力')}。Enter で{b('未整理')}の列へ", "K3・K6"],
          [b("D2"), "したいこと・そうすれば の欄が並ぶ", f"{b('本文')}（Markdown）。欄は入っているとき・「項目を足す」で", "K6"],
          [b("D3"), "前提・操作・結果の 3 欄を書く", f"{b('1 行')}（☐ 結果）。前提・操作は「詳しく」", "K4"],
          [b("D4"), "ペルソナの絞り込み・抜けの警告が常に出る", f"{b('使うときだけ')}。活動 0 件ならリリース × 未整理。抜けは数だけ", "K3・K5"],
          [b("D5"), "AI はマップを読めるだけ", f"AI が{b('聞いてから直す')}（create_story・update_story）", "K6"],
      ], ["7%", "36%", "45%", "12%"], size=24), gap=30)

# 4 D1 before/after ---------------------------------------------------------
before_after("d1", "03 · D1 ひとこと入力と未整理の列", "思いついたら 1 行。置き場所は後で決める", "s15-map.png", "d1-map.png",
             f"M1 でマップの下にあった「手順につながっていないストーリー」を、{b('最初の列「未整理」')}に移した。リリースのない行は「未定」。カードには本文の 1 行目が出る")

# 5 D2 before/after ---------------------------------------------------------
before_after("d2", "04 · D2 本文", "タイトルと本文だけで書ける。欄は必要になったら", "s16-story.png", "d2-story.png",
             f"タイトルの下に{b('本文')}（クリックで編集、Markdown で表示）。「したいこと」「そうすれば」は空なら隠れ、「項目を足す」で出る。手順がなければ「未整理」と表示")

# 6 D3 ---------------------------------------------------------------------
shot_slide("d3", "05 · D3 受け入れ条件は 1 行", "「こうなっていればいい」を 1 行。詳しく書きたいときだけ 3 欄", "d3-criteria.png", [
    ("1 行", "☐ の入力に結果だけを書いて追加。DB は M1 のまま（前提・操作は空）"),
    ("詳しく", "押すと前提・操作の欄が出る（写真は開いたところ）"),
    ("イシューへ写す", "1 行の条件は 1 行のまま写る。3 欄のものは【前提】【操作】【結果】"),
])

# 7 D4 ---------------------------------------------------------------------
shot_slide("d4", "06 · D4 使うときだけ", "活動もペルソナもないプロジェクトは、リリース × 未整理だけ", "d4-empty.png", [
    ("ペルソナ", "0 件なら絞り込みを出さない。S16 のペルソナ欄も出さない"),
    ("活動と手順", "0 件ならマップは未整理の列だけ。「＋ 活動」と短い説明"),
    ("抜け", "警告の色をやめて数だけ。開けば一覧"),
])

# 8 D5 ---------------------------------------------------------------------
shot_slide("d5", "07 · D5 AI が整える", "AI は聞いてから直す。オーナーは「はい / いいえ」で答えるだけ", "d5-qa.png", [
    ("読む", "get_storymap で未整理と抜けを読む"),
    ("聞く", "置き場所・リリース・条件の分け方を ask_question（yes/no）で。質問は整理用のタスクに付ける"),
    ("直す", "「はい」のものだけ update_story。履歴には AI の名前が残る"),
])

# 9 S09 --------------------------------------------------------------------
shot_slide("s09", "08 · S09 イシュー詳細", "イシューの「なぜ」に本文も出る", "s09-issue.png", [
    ("満たすストーリー", "タイトル・本文・したいこと・条件"),
    ("条件から写す", "1 行の条件はそのまま 1 行で"),
    ("AI にも同じ", "MCP の get_issue の stories に body"),
])

# 10 DB / API / MCP ----------------------------------------------------------
frame("api", "09 · DB・API・MCP の変更点", "テーブルは M1 のまま。足したのは本文の列と、AI の道具 2 つ",
      table(["どこ", "変更"], [
          [b("req_stories"), "body TEXT NOT NULL DEFAULT ''（マイグレーション 010）"],
          [b("API"), "Story・StoryRef・StoryInput・StoryPatch に body。変更履歴にも残る。ほかの操作は M1 のまま"],
          [b("MCP create_story"), "project・title だけで作れる。body・release・step_keys・criteria（then だけでよい）は任意"],
          [b("MCP update_story"), "title・body・persona_key・i_want・so_that・release・step_keys（[] で未整理に戻す）・add_criteria・update_criteria"],
          [b("MCP の説明"), "「get_storymap で読み、ask_question（yes_no）で聞き、承認されたものだけ update_story」。サーバーは AI を呼ばない"],
          [b("demo データ"), "ST6・ST7 を未整理で追加（本文あり、ST6 は 1 行の条件つき）"],
      ], ["24%", "76%"], size=24), gap=30)

# 11 next ------------------------------------------------------------------
frame("next", "10 · 制限と確認したいこと", "置き場所はまだドラッグできない",
      f"""<div style="display:flex;gap:28px">
<div style="flex:1">{notes([
    ("置き場所", "S16 の「手順につなぐ」とリリースの欄で決める。ドラッグはまだ"),
    ("抜けの数", "未整理が多いと大きくなる（3 件で 8）。未整理を抜けから外すかは未定"),
    ("M1 の制限", "ペルソナで絞ると、活動と別のペルソナのストーリーが消える（そのまま）"),
])}</div>
<div style="flex:1;display:flex;flex-direction:column;gap:18px">
{card("1. 形", "未整理の列・本文・1 行の条件・使うときだけ、でよいか", color=TEAL, extra="padding:20px 28px")}
{card("2. D5 を試す", "AI が聞いて、はいだけ直す流れを本番の TAPP で試してよいか", color=TEAL, extra="padding:20px 28px")}
</div></div>""", gap=28)

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "案 D 実装: M1 をゆるく使う",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
