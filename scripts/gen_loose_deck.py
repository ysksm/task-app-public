"""Generate docs/slides/src/loose/ (deck.json + slides/*.html): looser alternatives to the M1 requirements layer.

Source of the content: docs/09-requirements-loose.md. Nothing here is implemented, so the screens are
HTML mock-ups, not screenshots. Run, then `python3 scripts/build_slides.py`.
"""
import json
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE.parent / "docs" / "slides" / "src" / "loose"

_src = (HERE / "gen_devcycle_deck.py").read_text()
_helpers = _src[_src.index("ACC = "):_src.index("# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]
TEAL, ORANGE, GREEN, AMBER, GREY = "#2F7D8C", "#B85A1E", "#2E7D4F", "#9A6B00", "#9AA0B0"

# The app's dark UI, for the mock-ups.
UI_BG, UI_PANEL, UI_LINE, UI_FG, UI_MUTED = "#16171C", "#1E2027", "#2C2F38", "#E6E7EB", "#8B8F9C"
DOT = {"draft": "#8B8F9C", "ready": "#4E8DF5", "in_progress": "#E0A23D", "done": "#3FB27F"}


def dark(text):
    return f'<div style="background:{INK};color:{BG};border-radius:16px;padding:24px 34px;font-size:26px;line-height:1.6">{text}</div>'


def chip(text, color=ACC):
    return (f'<span style="display:inline-block;font-size:18px;padding:2px 10px;margin:2px 6px 2px 0;border-radius:999px;'
            f'background:{color}22;color:{color};border:1px solid {color}66">{text}</span>')


def tbl(name, cols, kind="new", w=330):
    border = {"new": f"3px solid {ACC}", "old": f"2px dashed {GREY}", "col": f"3px solid {ORANGE}"}[kind]
    bg = "#FFFFFF" if kind != "old" else "#ECEBE4"
    badge = {"new": ("新規", ACC), "col": ("列を追加", ORANGE), "old": ("", "")}[kind]
    tag = (f'<span style="float:right;font-size:14px;font-weight:700;color:{BG};background:{badge[1]};border-radius:6px;padding:1px 6px">{badge[0]}</span>'
           if badge[0] else "")
    lines = "".join(f'<div style="font-size:17px;line-height:1.45;color:{SUB}">{c}</div>' for c in cols)
    return (f'<div style="width:{w}px;box-sizing:border-box;background:{bg};border:{border};border-radius:12px;padding:12px 16px">{tag}'
            f'<div style="font-family:{MONO};font-size:19px;font-weight:600;margin-bottom:6px">{name}</div>{lines}</div>')


def arrow(label=""):
    return (f'<div style="display:flex;flex-direction:column;align-items:center;justify-content:center;color:{SUB};font-family:{MONO};font-size:15px">'
            f'{label}<span style="font-size:30px;color:{ACC}">→</span></div>')


def ui(body, w=1000, h=None, title=""):
    hh = f"height:{h}px;" if h else ""
    head = (f'<div style="height:40px;border-bottom:1px solid {UI_LINE};display:flex;align-items:center;gap:14px;padding:0 16px;font-size:16px;color:{UI_MUTED}">'
            f'<span style="color:{UI_FG};font-weight:600">{title}</span></div>') if title else ""
    return (f'<div style="width:{w}px;{hh}box-sizing:border-box;background:{UI_BG};color:{UI_FG};border:1px solid {UI_LINE};border-radius:12px;'
            f'overflow:hidden;box-shadow:0 10px 30px rgba(0,0,0,.25);font-size:16px">{head}<div style="padding:16px">{body}</div></div>')


def scard(key, title, status="draft", extra=""):
    return (f'<div style="background:{UI_PANEL};border:1px solid {UI_LINE};border-left:4px solid {DOT[status]};border-radius:8px;padding:8px 10px;margin-bottom:8px">'
            f'<div style="font-family:{MONO};font-size:13px;color:{UI_MUTED}">{key}</div><div style="font-size:16px;line-height:1.35">{title}</div>{extra}</div>')


# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]
TEAL, ORANGE, GREEN, AMBER, GREY = "#2F7D8C", "#B85A1E", "#2E7D4F", "#9A6B00", "#9AA0B0"

# The app's dark UI, for the mock-ups.
UI_BG, UI_PANEL, UI_LINE, UI_FG, UI_MUTED = "#16171C", "#1E2027", "#2C2F38", "#E6E7EB", "#8B8F9C"
DOT = {"draft": "#8B8F9C", "ready": "#4E8DF5", "in_progress": "#E0A23D", "done": "#3FB27F"}


def dark(text):
    return f'<div style="background:{INK};color:{BG};border-radius:16px;padding:24px 34px;font-size:26px;line-height:1.6">{text}</div>'


def chip(text, color=ACC):
    return (f'<span style="display:inline-block;font-size:18px;padding:2px 10px;margin:2px 6px 2px 0;border-radius:999px;'
            f'background:{color}22;color:{color};border:1px solid {color}66">{text}</span>')


def tbl(name, cols, kind="new", w=330):
    border = {"new": f"3px solid {ACC}", "old": f"2px dashed {GREY}", "col": f"3px solid {ORANGE}"}[kind]
    bg = "#FFFFFF" if kind != "old" else "#ECEBE4"
    badge = {"new": ("新規", ACC), "col": ("列を追加", ORANGE), "old": ("", "")}[kind]
    tag = (f'<span style="float:right;font-size:14px;font-weight:700;color:{BG};background:{badge[1]};border-radius:6px;padding:1px 6px">{badge[0]}</span>'
           if badge[0] else "")
    lines = "".join(f'<div style="font-size:17px;line-height:1.45;color:{SUB}">{c}</div>' for c in cols)
    return (f'<div style="width:{w}px;box-sizing:border-box;background:{bg};border:{border};border-radius:12px;padding:12px 16px">{tag}'
            f'<div style="font-family:{MONO};font-size:19px;font-weight:600;margin-bottom:6px">{name}</div>{lines}</div>')


def arrow(label=""):
    return (f'<div style="display:flex;flex-direction:column;align-items:center;justify-content:center;color:{SUB};font-family:{MONO};font-size:15px">'
            f'{label}<span style="font-size:30px;color:{ACC}">→</span></div>')


def ui(body, w=1000, h=None, title=""):
    hh = f"height:{h}px;" if h else ""
    head = (f'<div style="height:40px;border-bottom:1px solid {UI_LINE};display:flex;align-items:center;gap:14px;padding:0 16px;font-size:16px;color:{UI_MUTED}">'
            f'<span style="color:{UI_FG};font-weight:600">{title}</span></div>') if title else ""
    return (f'<div style="width:{w}px;{hh}box-sizing:border-box;background:{UI_BG};color:{UI_FG};border:1px solid {UI_LINE};border-radius:12px;'
            f'overflow:hidden;box-shadow:0 10px 30px rgba(0,0,0,.25);font-size:16px">{head}<div style="padding:16px">{body}</div></div>')


def scard(key, title, status="draft", extra=""):
    return (f'<div style="background:{UI_PANEL};border:1px solid {UI_LINE};border-left:4px solid {DOT[status]};border-radius:8px;padding:8px 10px;margin-bottom:8px">'
            f'<div style="font-family:{MONO};font-size:13px;color:{UI_MUTED}">{key}</div><div style="font-size:16px;line-height:1.35">{title}</div>{extra}</div>')


def mock_note(pending=False):
    return f'<span style="font-size:13px;color:{AMBER if pending else UI_MUTED}">{"（案）" if pending else ""}</span>'


# 1 cover -------------------------------------------------------------------
slides.append(("cover", f"""<section id="cover" data-transition="fade" style="background:{INK};color:{BG};font-family:{SANS};padding:128px;display:flex;flex-direction:column;justify-content:space-between">
<div style="position:absolute;left:0;top:0;width:24px;height:1080px;background:{ACC}"></div>
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">DEV CYCLE · LOOSER OPTIONS</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">要求の層<br>構造をゆるくする案</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">M1（案 A）は固すぎる、という感想を受けて。<br>M1 の画面は進めたまま、ゆるい案を三つ比べる</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">docs/09-requirements-loose.md · 提案だけ（未実装）</p>
<p style="font-size:28px;color:#B9BED0">2026-11-02 · 確認待ち</p>
</div>
</section>"""))

# 2 where we are ------------------------------------------------------------
frame("where", "01 · いまの位置", "M1 は PR #15 で進め、その上で「どこまでゆるめるか」を決める",
      f"""<div style="display:flex;gap:28px">
{card("M1（案 A）", f"PR #15 に出した。{b('テーブル 11・API 25 操作')}。ペルソナ → ジョブ → 活動 → 手順 → ストーリー → 条件 → イシュー")}
{card("感想", f"よくできているが、そのぶん{b('ガチガチすぎる')}。画面はこのまま進め、ゆるい案も見たい", color=ORANGE)}
{card("この資料", f"固いところを {b('K1〜K6')} に分け、それを外す案 {b('B・C・D')} を比べる。どれも実装はまだ", color=TEAL)}
</div>
{dark(f"利用者はオーナーと AI だけ。<b style='color:{ACC_L}'>重い分類は、共有する相手がいるときほどには役に立たない</b>")}""", gap=36)

# 3 rigid -------------------------------------------------------------------
frame("rigid", "02 · 何が固いのか", "固さは DB よりも「書くときの決まり」から来ている",
      table(["#", "固いところ", "使うときに起きること"], [
          [b("K1"), "キーが 6 系列（P・J・A・A-n・ST・AC）", "何を何と呼ぶかを覚えないと書けない"],
          [b("K2"), "テーブル 11、うち N:N が 4", "API 25 操作。AI も覚える道具が多い"],
          [b("K3"), "書く順番が決まった鎖", "マップに載せるには、先に活動と手順を作る"],
          [b("K4"), "受け入れ条件は前提・操作・結果の 3 欄", "「こうなっていればいい」の 1 行が書きにくい"],
          [b("K5"), "マップの軸が固定（手順 × リリース）", "「朝」「AI」など別の切り口で並べられない"],
          [b("K6"), "書く前に分類を決める", "思いついたときに、すぐ書き留められない"],
      ], ["7%", "43%", "50%"], size=26), gap=32)

# 4 invariant ---------------------------------------------------------------
frame("keep", "03 · どの案でも守ること", "AI が、イシューの「なぜ」を読めること",
      f"""<div style="display:flex;gap:28px">
{card("なぜ要るか", f"08 の棚卸しで見つけた抜け {b('B2-2')}：AI は担当のイシューを読めても、{b('何のための作業か')}がわからない。要求の層を足す一番の理由")}
{card("読む道具", f"MCP の {b('get_issue')}。どの案でも、ここに「なぜ」（要求のタイトル・本文・条件）が入る", color=TEAL)}
</div>
{table(["案", "get_issue で読める「なぜ」"], [
    [b("A（M1）"), "stories：ストーリーのタイトル・したいこと・受け入れ条件"],
    [b("B 要求メモ"), "つながったメモのタイトル・本文・チェック行"],
    [b("C イシューで書く"), "親の「要求」イシューの説明と受入基準（今の get_issue のまま）"],
    [b("D M1 をゆるく使う"), "A と同じ＋ストーリーの本文"],
], ["26%", "74%"], size=25)}""", gap=32)

# 5 B ------------------------------------------------------------------------
note_mock = ui(f"""<div style="font-family:{MONO};font-size:13px;color:{UI_MUTED}">TAPP-R12</div>
<div style="font-size:22px;font-weight:600;margin:4px 0 10px">朝、今日やるものを数件に絞りたい</div>
<div>{chip("@オーナー", "#4E8DF5")}{chip("#朝", "#E0A23D")}{chip("R1", "#3FB27F")}</div>
<div style="margin-top:12px;line-height:1.7;color:{UI_FG};font-size:16px">マイタスクを開いたら、未完了で注力のプロジェクトのものだけが見えてほしい。<br>回答待ちの QA は別の場所でいい。</div>
<div style="margin-top:10px;line-height:1.8;font-size:16px">☐ 完了したものは最初から隠れている<br>☐ 注力のプロジェクトだけに絞れる<br>☑ 回答待ちの QA はサイドバーのバッジで見る</div>
<div style="margin-top:12px;border-top:1px solid {UI_LINE};padding-top:10px;font-size:14px;color:{UI_MUTED}">つながったイシュー：TAPP-41 · TAPP-42</div>""", w=760, title="要求メモ")
board_mock = ui(f"""<div style="font-size:13px;color:{UI_MUTED};margin-bottom:8px">横：<span style="color:{UI_FG}">#の付くタグ</span>　縦：<span style="color:{UI_FG}">R の付くタグ</span>（切り替えられる）</div>
<div style="display:grid;grid-template-columns:50px 1fr 1fr 1fr;gap:8px;font-size:14px">
<div></div><div style="color:{UI_MUTED}">#朝</div><div style="color:{UI_MUTED}">#AIに任せる</div><div style="color:{UI_MUTED}">タグなし</div>
<div style="color:{UI_MUTED}">R1</div><div>{scard("R12", "今日の数件に絞る", "in_progress")}</div><div>{scard("R14", "任せたものを追う", "ready")}</div><div></div>
<div style="color:{UI_MUTED}">R2</div><div></div><div>{scard("R15", "AI の質問をまとめて返す")}</div><div>{scard("R16", "週末にふりかえる")}</div>
</div>""", w=720, title="要求メモ · タグで並べる")
frame("b", "04 · 案 B「要求メモ」", "いちばんゆるい：要求は 1 種類のメモだけ。分類はタグ",
      f"""<div style="display:flex;gap:36px;align-items:flex-start">{note_mock}{board_mock}</div>
<div style="display:flex;gap:24px">
{card("書き方", f"本文は自由な Markdown。{b('☐ の行が受け入れ条件')}。@ は誰の、# は場面、R はリリース。どれも任意")}
{card("マップ", f"保存しない。{b('タグで並べた見え方')}。色は M1 と同じく、つながったイシューから計算", color=TEAL)}
</div>""", gap=28)

# 6 B ER --------------------------------------------------------------------
frame("b-er", "05 · 案 B の DB", "テーブルは 3 つ。ペルソナ・活動・リリースはタグの文字列になる",
      f"""<div style="display:flex;gap:18px;align-items:center">
{tbl("projects", ["既存"], "old", 220)}{arrow("1:N")}
{tbl("req_notes", ["TAPP-R12 · title", "body（Markdown）", "parent_id（NULL 可）"])}{arrow("1:N")}
{tbl("req_note_tags", ["note_id, tag", "'@オーナー' '#朝' 'R1'"], w=300)}
</div>
<div style="display:flex;gap:18px;align-items:center;margin-left:458px">
<span style="font-size:30px;color:{ACC}">↓</span><span style="font-family:{MONO};font-size:15px;color:{SUB}">1:N</span>
</div>
<div style="display:flex;gap:18px;align-items:center;margin-left:410px">
{tbl("req_note_issues", ["note_id, issue_id", "N:N"], w=300)}{arrow("N:1")}{tbl("issues", ["既存（変更なし）"], "old", 260)}
</div>
{table(["", "案 B"], [
    [b("なくなる固さ"), "K1〜K6 すべて"],
    [b("PR #15"), f"{b('置き換え')}。009 の 11 テーブルと API・画面を作り直す（本番データの移行はない）"],
    [b("弱いところ"), "タグの表記ゆれ。「手順はあるのにストーリーがない」などの抜けを機械で見つけにくい"],
], ["22%", "78%"], size=24)}""", gap=22)

# 7 C -----------------------------------------------------------------------
tree = ui(f"""<div style="line-height:2;font-size:16px">
<div><span style="color:#B07CF0">◆ 要求</span> TAPP-40 朝、今日やるものを数件に絞りたい <span style="color:{UI_MUTED};font-size:13px">完了なし · 子 1/3 完了</span></div>
<div style="padding-left:28px">├ <span style="color:{DOT['done']}">●</span> TAPP-41 完了を既定で隠す</div>
<div style="padding-left:28px">├ <span style="color:{DOT['in_progress']}">●</span> TAPP-42 注力のみを既定に</div>
<div style="padding-left:28px">└ <span style="color:{DOT['draft']}">●</span> TAPP-43 回答待ち QA タブを外す</div>
<div style="margin-top:6px;color:{UI_MUTED};font-size:14px">受入基準（今のまま）：☐ 完了は隠れている　☐ 注力だけに絞れる</div></div>""", w=860, title="イシュー · 種別「要求」")
frame("c", "06 · 案 C「イシューで書く」", "新しいものをほとんど足さない：要求はイシューの種別の一つ",
      f"""<div style="display:flex;gap:32px;align-items:flex-start">{tree}
<div style="display:flex;flex-direction:column;gap:14px;flex:1">
{tbl("issues.type", ["+ 'requirement'", "完了の状態を持たない", "状態は子から計算して表示"], "col", 420)}
{tbl("acceptance_criteria", ["既存のまま", "要求の受け入れ条件として使う"], "old", 420)}
</div></div>
{table(["", "案 C"], [
    [b("08 で見送った理由"), "「完了で閉じると要求も閉じる」→ 要求の種別には完了の状態を持たせないことで避ける"],
    [b("なくなる固さ"), "K1・K2・K4・K6（K3・K5 は親子とラベルの使い方しだい）"],
    [b("PR #15"), f"{b('置き換え')}（009 は使わない）。AI は今の get_issue で親を読めば足りる"],
    [b("弱いところ"), "一覧・マイタスク・全体ボードに「要求」が混ざり、どこでも隠す設定が要る。受入基準の意味が二つ重なる"],
], ["22%", "78%"], size=23)}""", gap=26)

# 8 D -----------------------------------------------------------------------
frame("d", "07 · 案 D「M1 をゆるく使う」（おすすめ）", "テーブルは M1 のまま。決まりと画面をゆるめ、分類は後から AI と付ける",
      table(["#", "変えること", "なくなる固さ"], [
          [b("D1"), f"マップの上に{b('ひとこと入力')}。何も選ばずに入り、「未整理」の列に置かれる", "K3・K6"],
          [b("D2"), f"ストーリーに{b('本文')}（自由な Markdown）。「したいこと」「そうすれば」は入っているときだけ出す", "K6"],
          [b("D3"), f"受け入れ条件は{b('1 行')}（結果だけ）。前提・操作は「詳しく」で開く。DB はもともと空でよい", "K4"],
          [b("D4"), f"ペルソナ・ジョブ・活動・手順は{b('使うときだけ')}。0 件ならマップは「リリース × 未整理」だけ。抜けは警告の色をやめて数だけ", "K3・K5"],
          [b("D5"), f"{b('AI が整える')}：「ST7 を A1-2 に置いてよいか」を QA（yes/no）で聞く。オーナーは答えるだけ", "K6"],
      ], ["7%", "75%", "18%"], size=25)
      + f"""<div style="display:flex;gap:24px;align-items:center">
{tbl("req_stories", ["+ body（本文）", "マイグレーション 010"], "col", 380)}
{tbl("ほかの 10 テーブル", ["M1 のまま"], "old", 300)}
{card("MCP", f"{b('update_story')}（本文・置き場所・条件）を足し、AI が整理できるようにする", color=TEAL)}
</div>""", gap=26)

# 9 D mock ------------------------------------------------------------------
quick = (f'<div style="display:flex;gap:10px;align-items:center;background:{UI_PANEL};border:1px solid #4E8DF5;border-radius:8px;padding:10px 14px;margin-bottom:14px">'
         f'<span style="color:#4E8DF5">＋</span><span style="color:{UI_FG}">朝の画面で、AI に任せたものだけを見たい</span>'
         f'<span style="margin-left:auto;font-size:13px;color:{UI_MUTED}">Enter で「未整理」へ</span></div>')
crit = (f'<div style="margin-top:6px;font-size:13px;color:{UI_MUTED}">☐ 完了は最初から隠れている</div>')
grid = f"""<div style="display:grid;grid-template-columns:60px 230px 230px 230px;gap:10px;font-size:14px">
<div></div><div style="color:#E0A23D;font-weight:600">未整理 · 2</div><div style="color:{UI_MUTED}">A1 朝：今日を決める</div><div style="color:{UI_MUTED}">A3 AI に任せる</div>
<div style="color:{UI_MUTED}">R1</div>
<div style="background:#E0A23D14;border:1px dashed #E0A23D66;border-radius:8px;padding:8px">{scard("ST8", "朝の画面で、AI に任せたものだけを見たい")}{scard("ST7", "週末に滞りを見たい")}</div>
<div>{scard("ST1", "今日の数件に絞る", "in_progress", crit)}</div><div>{scard("ST3", "任せたものを追う", "ready")}</div>
<div style="color:{UI_MUTED}">R2</div><div></div><div></div><div>{scard("ST5", "AI の質問をまとめて返す")}</div></div>"""
frame("d-map", "08 · 案 D の画面（ストーリーマップ）", "思いついたら 1 行。置き場所は後で決める",
      f"""<div style="display:flex;gap:32px;align-items:flex-start">
{ui(quick + grid, w=1060, title="ストーリーマップ · TAPP　　<span style='color:#8B8F9C;font-weight:400'>抜け 3</span>")}
<div style="display:flex;flex-direction:column;gap:18px;flex:1">
{card("D1 ひとこと入力", "ペルソナも手順もリリースも選ばない。入ったものは「未整理」の列へ", extra="padding:20px 24px")}
{card("D3 条件は 1 行", "カードの上で ☐ の 1 行。前提・操作は詳細の「詳しく」から", color=TEAL, extra="padding:20px 24px")}
{card("D4 抜けは数だけ", "警告の色をやめる。開けば一覧は出る", color=ORANGE, extra="padding:20px 24px")}
</div></div>""", gap=28)

# 10 D AI -------------------------------------------------------------------
qa = ui(f"""<div style="font-size:13px;color:{UI_MUTED}">QA インボックス · claude から</div>
<div style="font-size:20px;font-weight:600;margin:6px 0 12px">未整理のストーリー 2 件の置き場所</div>
<div style="background:{UI_PANEL};border:1px solid {UI_LINE};border-radius:8px;padding:12px;margin-bottom:10px">
<div>ST8「朝の画面で、AI に任せたものだけを見たい」を <b>A1-1 自分の未完了だけを見る</b>・R1 に置いてよいか</div>
<div style="margin-top:8px;display:flex;gap:8px"><span style="background:#3FB27F;color:#0b1a12;border-radius:6px;padding:3px 14px">はい</span><span style="border:1px solid {UI_LINE};border-radius:6px;padding:3px 14px">いいえ</span></div></div>
<div style="background:{UI_PANEL};border:1px solid {UI_LINE};border-radius:8px;padding:12px">
<div>ST7 の条件「滞りがわかる」を、次の 2 行に分けてよいか<br><span style="color:{UI_MUTED}">☐ 3 日動いていないものが上に出る　☐ プロジェクトごとの件数が出る</span></div>
<div style="margin-top:8px;display:flex;gap:8px"><span style="background:#3FB27F;color:#0b1a12;border-radius:6px;padding:3px 14px">はい</span><span style="border:1px solid {UI_LINE};border-radius:6px;padding:3px 14px">いいえ</span></div></div>""", w=900, title="QA")
frame("d-ai", "09 · 案 D5 AI が整える", "分類は AI が提案し、オーナーは答えるだけ",
      f"""<div style="display:flex;gap:36px;align-items:flex-start">{qa}
<div style="display:flex;flex-direction:column;gap:18px;flex:1">
{card("流れ", f"1. AI が {b('get_storymap')} で未整理と抜けを読む<br>2. 置き場所や条件の分け方を {b('ask_question')}（yes/no）で聞く<br>3. はい → AI が {b('update_story')} で直す")}
{card("今あるものを使う", "QA インボックスとバッジは今のまま。新しい画面は要らない", color=TEAL)}
</div></div>""", gap=30)

# 11 compare ----------------------------------------------------------------
frame("compare", "10 · くらべる", "D は M1 を活かしたまま、使う人から見える固さを外す",
      table(["", "A（M1・今）", "B 要求メモ", "C イシューで書く", "D M1 をゆるく使う"], [
          [b("新しいテーブル"), "11", "3", "0", "0（列 1）"],
          [b("書き始めるまで"), "活動と手順を先に作る", "タイトル 1 行", "イシューを起票", "ひとこと 1 行"],
          [b("受け入れ条件"), "前提・操作・結果", "本文のチェック行", "今の受入基準", "1 行（詳しくも書ける）"],
          [b("並べ方"), "手順 × リリース", "タグで自由に", "ボードのグループ", "未整理 → 手順 × リリース"],
          [b("抜けを見つける"), "自動", "むずかしい", "親のない作業だけ", "自動（整理した分）"],
          [b("PR #15"), "—", "置き換え", "置き換え", "そのまま使う"],
          [b("作業の量"), "済み", "大", "中", "小"],
      ], ["18%", "19%", "20%", "20%", "23%"], size=24), gap=30)

# 12 next -------------------------------------------------------------------
frame("next", "11 · おすすめと確認したいこと", "おすすめは D。使ってみて足りなければ B に寄せる",
      f"""<div style="display:flex;gap:24px">
{card("D をすすめる理由", f"固さは{b('書くときの決まり')}から来ているので、DB を変えずにゆるめられる。分類を AI に任せれば、オーナーは書くことと答えることだけ。D の本文とタイトルは、後で B のメモにそのまま移せる")}
</div>
<div style="display:flex;flex-direction:column;gap:16px">
{card("1. どの案で進めるか", "B・C・D のどれか（おすすめ D。PR #15 のマージ後に小さな変更として入れる）", color=TEAL, extra="padding:20px 28px")}
{card("2. D の中身", "D1〜D5 のうち、要らないもの・足したいものはあるか", color=TEAL, extra="padding:20px 28px")}
</div>""", gap=24)

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "要求の層: 構造をゆるくする案",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
