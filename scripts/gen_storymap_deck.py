"""Generate docs/slides/src/storymap/ (deck.json + slides/*.html).

Source of the content: docs/08-devcycle.md. Run, then `python3 scripts/build_slides.py`.
"""
import importlib.util
import json
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE.parent / "docs" / "slides" / "src" / "storymap"

# Reuse the look of the proposal deck (colours and helpers) without running it.
_src = (HERE / "gen_devcycle_deck.py").read_text()
_helpers = _src[_src.index("ACC = "):_src.index("# 1 cover")]
ns: dict = {"slides": []}
exec(compile("import json, pathlib\n" + _helpers, "gen_devcycle_deck helpers", "exec"), ns)
ACC, ACC_L, BG, INK, SUB, CARD, LINE, MONO, SANS = (ns[k] for k in ["ACC", "ACC_L", "BG", "INK", "SUB", "CARD", "LINE", "MONO", "SANS"])
frame, card, table, b = ns["frame"], ns["card"], ns["table"], ns["b"]
slides = ns["slides"]

TEAL, ORANGE, PURPLE = "#2F7D8C", "#B85A1E", "#8A5BB8"
RED, RED_BG = "#C2413A", "#FBE3E1"
AMBER, AMBER_BG = "#9A6B00", "#FFF1C9"
GREEN, GREEN_BG = "#2E7D4F", "#E1F3E7"


def dark(text):
    return f'<div style="background:{INK};color:{BG};border-radius:16px;padding:26px 36px;font-size:27px;line-height:1.6">{text}</div>'


def chip(text, kind="ok"):
    color, bg = {"ok": (SUB, "#EEF0F6"), "dup": (AMBER, AMBER_BG), "bad": (RED, RED_BG), "new": (GREEN, GREEN_BG)}[kind]
    return (f'<span style="display:inline-block;font-size:19px;line-height:1.3;padding:3px 9px;margin:3px 4px 0 0;border-radius:7px;'
            f'background:{bg};color:{color};border:1px solid {color}55">{text}</span>')


def step(name, chips=""):
    return (f'<div style="background:#FFF6C9;border:1px solid #E6D88A;border-radius:10px;padding:10px 12px">'
            f'<div style="font-size:22px;font-weight:700">{name}</div><div>{chips}</div></div>')


def storymap(cols, head_color=ACC):
    return '<div style="display:flex;gap:16px">' + "".join(
        f'<div style="flex:1;display:flex;flex-direction:column;gap:10px">'
        f'<div style="background:{head_color};color:{BG};border-radius:12px;padding:12px;font-size:24px;font-weight:700;text-align:center">{a}</div>'
        + "".join(step(n, c) for n, c in steps) + "</div>" for a, steps in cols) + "</div>"


def legend(items):
    return '<div style="display:flex;gap:28px;align-items:center;font-size:21px;color:' + SUB + '">' + "".join(
        f'<span>{chip(t, k)} {d}</span>' for t, k, d in items) + "</div>"


# 1 cover -------------------------------------------------------------------
slides.append(("cover", f"""<section id="cover" data-transition="fade" style="background:{INK};color:{BG};font-family:{SANS};padding:128px;display:flex;flex-direction:column;justify-content:space-between">
<div style="position:absolute;left:0;top:0;width:24px;height:1080px;background:{ACC}"></div>
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">DEV CYCLE · INVENTORY &amp; M1</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">マイタスクの棚卸しと<br>M1 設計</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">今の機能をストーリーマップに貼って「使い方がわからない」の原因を探し、<br>要求の層を足す最初の段階（M1）を設計する</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">設計書 docs/08-devcycle.md</p>
<p style="font-size:28px;color:#B9BED0">2026-11-02 · 確認待ち（未実装）</p>
</div>
</section>"""))

# 2 decided -----------------------------------------------------------------
frame("decided", "01 · 決まったこと", "今のアプリを拡張し、本人と AI だけで、マイタスクから試す",
      f"""<div style="display:flex;gap:32px">
{card("拡張して検証", "別アプリにしない。課題・MCP・質問・チェックポイント・受入検査をそのまま使い、要求の層を上に足す")}
{card("利用者は 2 人", "オーナー本人（Web 画面）と AI エージェント（MCP）。権限やレビューの仕組みは増やさない", color=TEAL)}
{card("題材はマイタスク", "わかりにくくなった本人の画面。まず今の機能を手順に貼り、その結果を最初のリリースにする", color=ORANGE)}
</div>
<div style="display:flex;flex-direction:column;gap:14px;font-size:27px;line-height:1.6;color:{SUB}">
<p>この資料の中身は 2 つ。</p>
<p>{b("① 棚卸し")}：ペルソナ・ジョブ・ストーリーマップを書き、今の機能を手順に貼る。どこにもつながらない機能や重複が、わかりにくさの原因として見えてくる</p>
<p>{b("② M1 設計")}：要求（L1〜L6）を持つテーブル・API・画面。実装はこの確認のあと</p>
</div>""")

# 3 personas & jobs ---------------------------------------------------------
frame("jobs", "02 · ペルソナとジョブ", "P1 オーナーと P2 AI エージェント、7 つのジョブ",
      table(["ID", "ペルソナ", "状況", "したいこと", "そうすれば"], [
          [b("J1"), "P1", "朝、作業を始める前", "今日手をつけるものを数件に絞りたい", "迷わずに始められる"],
          [b("J2"), "P1", "作業の合間", "自分の作業の状態とメモを素早く更新したい", "後で思い出す手間が減る"],
          [b("J3"), "P1", "手が回らない作業がある", "AI に任せられる形で起票し、任せたものを追いたい", "自分は判断に集中できる"],
          [b("J4"), "P1", "AI から質問や成果が届いた", "待たせずに答え、受け入れたい", "AI が止まらない"],
          [b("J5"), "P1", "週の終わり", "何が終わり、何が滞っているかを見たい", "次の週の注力を決められる"],
          [b("J6"), "P2", "作業を始めるとき", f"自分の担当と、それが{b('なぜ必要か')}を知りたい", "意図に合った実装ができる"],
          [b("J7"), "P2", "作業中に判断が要る", "人に聞くべきことだけを、答えやすい形で聞きたい", "待ち時間を減らせる"],
      ], ["7%", "10%", "23%", "38%", "22%"], size=24), gap=32)

# 4 P1 map with features ----------------------------------------------------
p1 = [
    ("A1 朝: 今日を決める", [
        ("自分の未完了だけを見る", chip("担当タブ") + chip("完了を隠す（既定オフ）", "dup")),
        ("注力プロジェクトに絞る", chip("注力のみ") + chip("全体ボード＋注力のみ", "dup")),
        ("期限・優先度の高いものを見る", chip("並び: 期日・優先度") + chip("優先度フィルタ")),
    ]),
    ("A2 日中: 進める", [
        ("状態を変える", chip("ステータス変更")),
        ("メモ・コメントを残す", chip("サイドピーク") + chip("コメント")),
        ("詳細を開いて確かめる", chip("サイドピーク")),
    ]),
    ("A3 AI に任せる", [
        ("起票して割り当てる", chip("新規（C）")),
        ("粒度を決める", chip("親・種別")),
        ("任せたものの様子を追う", chip("エージェント活動", "new") + chip("起票したもの？", "dup")),
    ]),
    ("A4 AI に答える", [
        ("質問に答える", chip("QA インボックス") + chip("回答待ち QA タブ", "dup")),
        ("成果を受け入れる", chip("受入検査", "new")),
    ]),
    ("A5 振り返る", [
        ("終わったものを見る", chip("全体ボード")),
        ("滞っているものを見る", chip("全体ボード")),
        ("ティアを見直す", chip("プロジェクト一覧")),
    ]),
]
orphans = "".join(chip(t, "bad") for t in ["グループ: 担当者（担当タブ）", "担当者フィルタ（担当タブ）", "並び: 手動（横断）", "ツリー表示", "配下フィルタ"])
frame("map-p1", "03 · 棚卸し: P1 のストーリーマップ", "今の機能を手順に貼ると、重複と行き場のない機能が見える",
      f"""{storymap(p1)}
<div style="display:flex;gap:18px;align-items:flex-start;background:{RED_BG};border:2px dashed {RED};border-radius:14px;padding:14px 20px">
<div style="font-size:24px;font-weight:700;color:{RED};white-space:nowrap;padding-top:4px">どの手順にもない</div><div>{orphans}</div></div>
{legend([("機能", "ok", "手順にある"), ("重複・ずれ", "dup", "まとめる・既定を変える"), ("孤立", "bad", "外す候補"), ("足した手順", "new", "例のマップになかった")])}""", gap=22)

# 5 P2 map ------------------------------------------------------------------
p2 = [
    ("B1 拾う", [("自分の担当を一覧する", chip("list_my_issues") + chip("search_issues"))]),
    ("B2 理解する", [("イシューと親を読む", chip("get_issue") + chip("get_issue_tree")),
                     ("なぜ必要かを読む", chip("手段がない", "bad"))]),
    ("B3 進める", [("状態を進める", chip("transition_issue") + chip("update_issue")),
                   ("チェックポイントを記録", chip("record_checkpoint")),
                   ("子タスクに分解する", chip("create_issue"))]),
    ("B4 聞く", [("QA を起票する", chip("ask_question")), ("回答を取得する", chip("get_question_answers") + chip("close_question"))]),
    ("B5 渡す", [("受入を依頼する", chip("request_acceptance"))]),
]
frame("map-p2", "04 · 棚卸し: P2 のストーリーマップ", "AI の道具はそろっている。欠けているのは「なぜ」だけ",
      f"""{storymap(p2, head_color=TEAL)}
{dark(f'エージェントは今、イシューの説明に書いてあることしか知らない。M1 で <b style="color:{ACC_L}">get_issue にストーリーと受け入れ条件を足し</b>、<br>「誰のどの場面のための作業か」を読めるようにする（J6）')}""", gap=30)

# 6 inventory table ---------------------------------------------------------
V = {"keep": f'<span style="color:{GREEN};font-weight:700">残す</span>', "dup": f'<span style="color:{AMBER};font-weight:700">まとめる</span>',
     "cut": f'<span style="color:{RED};font-weight:700">外す</span>', "fix": f'<span style="color:{AMBER};font-weight:700">見直す</span>',
     "def": f'<span style="color:{AMBER};font-weight:700">既定を変える</span>', "step": f'<span style="color:{RED};font-weight:700">手順が要る</span>'}
frame("inventory", "05 · 棚卸し: マイタスク（S01）", "手を入れたいのは 7 つ。残りはそのまま手順に対応する",
      table(["機能", "手順", "判定", "理由"], [
          ["タブ「担当」", "A1-1, A2-1", V["keep"], "マイタスクの本体"],
          ["タブ「起票したもの」", "A3-3 ？", V["fix"], "自分で持つものも混ざる。「任せたもの」（起票 = 自分・担当 = エージェント）に"],
          ["タブ「回答待ち QA」", "A4-1", V["dup"], "QA インボックスの「未回答 ＋ 自分宛て」と同じ。答えるのは S02 が向く"],
          ["表示「ツリー」", "？", V["step"], "横断のツリーはどの手順でもない。使わないなら外す"],
          ["グループ「担当者」・担当者フィルタ", "なし", V["cut"], "担当タブでは常に自分。選んでも意味のある結果にならない"],
          ["並び「手動」", "なし", V["cut"], "手動の順番は同じ親の中の並び。横断の一覧では意味がない"],
          ["「完了を隠す」", "A1-1", V["def"], "画面は既定オフ、MCP は未完了だけが既定。朝の用途ならオンに"],
          ["「配下」フィルタ", "なし", V["keep"] + "（低）", "ツリーと同じく、手順が決まらなければ外してよい"],
          ["新規・注力のみ・並び（期日/優先度）・ティア/種別/優先度フィルタ・ビュー保存・サイドピーク", "A1〜A3", V["keep"], "手順にそのまま対応する"],
      ], ["30%", "11%", "13%", "46%"], size=22), gap=28)

# 7 related screens ---------------------------------------------------------
frame("screens", "06 · 棚卸し: 関連する画面", "画面ごとの役割を、手順で言い切る",
      table(["画面", "手順", "判定", "メモ"], [
          [b("全体ボード（S14）"), "A5 振り返る", f'<span style="color:{AMBER};font-weight:700">役割を決める</span>', "「注力のみ」で朝にも使えてしまい、マイタスクとかぶる。マップ上は振り返りの画面"],
          [b("QA インボックス（S02）"), "A4-1", V["keep"], "答える場所はここに一本化"],
          [b("受入検査（S03）"), "A4-2", V["keep"], "例のマップに手順がなかった。棚卸しで足した"],
          [b("エージェント活動（S04）"), "A3-3", V["keep"], "同上。任せたものの様子を見る"],
          [b("プロジェクト一覧（S05）"), "A5-3", V["keep"], "ティアを見直す"],
      ], ["26%", "15%", "16%", "43%"], size=25)
      + dark(f'棚卸しの効き目：提案の例のマップに今の画面を貼ったら、<br><b style="color:{ACC_L}">足りない手順</b>（成果を受け入れる・任せたものの様子を追う・滞りを見る）が見つかった'))

# 8 findings & fixes --------------------------------------------------------
finds = [("1", "同じものが 2 か所", "「回答待ち QA」タブと QA インボックス"),
         ("2", "朝の画面が決まっていない", "マイタスクと「全体ボード ＋ 注力のみ」"),
         ("3", "意味のない選択肢", "担当タブのグループ「担当者」・担当者フィルタ、横断の「並び: 手動」"),
         ("4", "既定のずれ", "画面は完了も出し、MCP は未完了だけ"),
         ("5", "目的のわからないもの", "「起票したもの」タブ、マイタスクのツリー表示")]
fixes = [("T1", "「回答待ち QA」タブを外し、QA インボックスに一本化", "1"),
         ("T2", "朝 = マイタスク、振り返り = 全体ボードと決めてガイドに書く。「完了を隠す」を既定オンに", "2, 4"),
         ("T3", "担当タブでグループ「担当者」と担当者フィルタを出さない。マイタスクで「並び: 手動」を出さない", "3"),
         ("T4", "「起票したもの」を「任せたもの」（起票 = 自分・担当 = エージェント）に", "5"),
         ("T5", "マイタスクのツリー表示は、手順が決まらなければ外す", "5")]
fl = "".join(f'<div style="display:flex;gap:16px;padding:12px 0;border-bottom:1px solid {LINE}"><span style="font-family:{MONO};font-size:24px;color:{RED};width:28px">{n}</span>'
             f'<div><div style="font-size:25px;font-weight:700">{t}</div><div style="font-size:22px;color:{SUB};line-height:1.5">{d}</div></div></div>' for n, t, d in finds)
fx = "".join(f'<div style="display:flex;gap:16px;padding:12px 0;border-bottom:1px solid {LINE}"><span style="font-family:{MONO};font-size:24px;color:{GREEN};width:44px">{n}</span>'
             f'<div style="flex:1;font-size:23px;line-height:1.5">{t}</div><span style="font-size:20px;color:#7A8091;white-space:nowrap">← {r}</span></div>' for n, t, r in fixes)
frame("findings", "07 · わかりにくさの原因と、整理の案", "原因は 5 つ。M1 とは別の小さな変更で片づく",
      f"""<div style="display:flex;gap:48px">
<div style="flex:1"><h3 style="font-size:30px;color:{RED};margin-bottom:6px">見えてきたこと</h3>{fl}</div>
<div style="flex:1.15"><h3 style="font-size:30px;color:{GREEN};margin-bottom:6px">整理の案</h3>{fx}</div>
</div>""", gap=32)

# 9 M1 scope ----------------------------------------------------------------
frame("scope", "08 · M1 の範囲", "要求の層（L1〜L6）とつながりだけを入れる",
      table(["入れる", "入れない（後の段階）"], [
          ["ペルソナ・ジョブ・活動・手順・ストーリー・受け入れ条件（L1〜L6）", "ゴール・指標（L0）。M1 はプロジェクトの説明で代用"],
          ["ストーリーと手順、ストーリーとイシューのつながり", "設計要素（L7）、テストとの結びつけ・CI の取り込み（M2）"],
          ["ストーリーマップ画面と、つながりの抜けの表示", "関門の自動判定・AI の下書き・ガイドの生成（M3）"],
          ["イシュー詳細と MCP の get_issue に「なぜ（ストーリー・条件）」", "フィードバック（M4）"],
      ], ["52%", "48%"], size=26)
      + dark("M1 が終わると：マップの上で「どの手順が、どのストーリーと、どのイシューで満たされているか」が見え、<br>エージェントは作業の理由を読める"))

# 10 option A/B -------------------------------------------------------------
frame("options", "09 · 決めたいこと: 今の「受入基準」との関係", "要求は新しいテーブルに持ち、イシューから「つなぐ」（案 A）",
      f"""<div style="display:flex;gap:32px;flex:1">
{card("案 A（おすすめ）新しいテーブル", f"ストーリー（L5）と受け入れ条件（L6）は新しいテーブル。イシューはストーリーに{b('つながる')}。<br>イシューの受入基準は「その回の実装で確かめること」として今のまま。<br><br>{b('長所')}：イシューが Done になっても要求は残る。受入検査を壊さない<br>{b('短所')}：「受け入れ条件」と「受入基準」の 2 つの言葉。画面で区別が要る", extra="border-top:8px solid " + ACC)}
{card("案 B 今の仕組みを使う", f"ストーリー = 今の「ストーリー」種別のイシュー、受け入れ条件 = 今の受入基準に Given/When/Then を足す。<br><br>{b('長所')}：追加が少ない<br>{b('短所')}：ストーリーのイシューが Done で閉じると、要求も閉じたことになる。「要求は生き続ける」と合わない", color=SUB, extra="border-top:8px solid #9AA0B0")}
</div>
<p style="font-size:25px;color:{SUB};line-height:1.6">案 A では、イシューの受入基準を作るときに「つながっているストーリーの受け入れ条件から写す」ボタンを付ける（写すだけで、同期はしない）</p>""", gap=28)

# 11 data model -------------------------------------------------------------
frame("model", "10 · データモデル（マイグレーション 009）", "種類ごとに連番のキー。消さずにアーカイブ、変更は履歴に残す",
      f"""<div style="display:flex;gap:40px">
<div style="flex:0.9">{table(["種類", "キーの例", "テーブル"], [
          ["ペルソナ", "TAPP-P1", "req_personas"], ["ジョブ", "TAPP-J3", "req_jobs"], ["活動", "TAPP-A1", "req_activities"],
          ["手順", "TAPP-A1-2", "req_steps"], ["ストーリー", "TAPP-ST12", "req_stories"], ["受け入れ条件", "TAPP-AC42", "req_criteria"],
      ], ["30%", "32%", "38%"], size=24)}</div>
<div style="flex:1.1;display:flex;flex-direction:column;gap:14px;font-size:24px;line-height:1.55;color:{SUB}">
<p>{b("つながり")}：req_job_activities（ジョブ ⇄ 活動）、req_story_steps（ストーリー ⇄ 手順）、req_story_issues（ストーリー ⇄ イシュー）。どれも N:N</p>
<p>{b("キーの採番")}：req_counters（プロジェクト × 種類）。アーカイブしてもキーは使い回さない</p>
<p>{b("版")}：M1 は req_changes（誰が・いつ・何を）で代用。リリース時点の固定は M3 で検討</p>
<p>{b("リリース")}：ストーリーの release（自由な文字列、例 R1）。マップの横線はこの値でまとめる</p>
<p>{b("受け入れ条件")}：given / when / then の 3 つの欄。M2 でテストのタグ（@TAPP-AC42）につなぐ</p>
</div>
</div>""", gap=32)

# 12 API --------------------------------------------------------------------
frame("api", "11 · API と MCP", "マップ全体を 1 回で取り、各要素は個別に編集する",
      table(["メソッドとパス（/api/v1/projects/{key}/…）", "内容"], [
          [b("GET storymap"), "マップ全体（ペルソナ・ジョブ・活動・手順・ストーリー・条件の数・イシューの進捗）と抜け（gaps）"],
          ["GET/POST personas · jobs、PATCH/DELETE …/{キー}", "ペルソナ・ジョブ（ジョブは活動とのつながりも）"],
          ["POST activities、POST activities/{キー}/steps、PATCH/DELETE", "活動と手順。並べ替えは position"],
          ["GET/POST stories、GET/PATCH/DELETE stories/{キー}", "ストーリー（手順とのつながり・リリースも）"],
          ["POST stories/{キー}/criteria、PATCH/DELETE criteria/{キー}", "受け入れ条件"],
          ["PUT/DELETE stories/{キー}/issues/{イシューキー}", "ストーリーとイシューをつなぐ・外す"],
          ["GET /api/v1/issues/{キー}（既存）", "stories（キー・タイトル・条件）を足す"],
      ], ["46%", "54%"], size=23)
      + f"""<div style="display:flex;gap:28px">
{card("gaps（抜け）", "手順につながらないストーリー、ストーリーのない手順、条件のないストーリー、イシューのないストーリー、活動につながらないジョブ")}
{card("MCP", "get_issue に stories を足す（B2-2）。読み取りの get_storymap を足す。書き込みのツールは M3（AI の下書き）で", color=TEAL)}
</div>""", gap=26)

# 13 screen mock ------------------------------------------------------------
def scard(t, kind):
    color, bg = {"gray": ("#7A8091", "#EEF0F6"), "white": (INK, "#FFFFFF"), "amber": (AMBER, AMBER_BG), "green": (GREEN, GREEN_BG)}[kind]
    return f'<div style="background:{bg};border:2px solid {color};border-radius:8px;padding:6px 10px;font-size:19px;color:{INK};margin-top:6px">{t}</div>'


mock_cols = [("A1 朝", "自分の未完了", [("完了を隠すを既定に", "amber")], [("朝の 3 件を提案", "gray")]),
             ("A3 任せる", "様子を追う", [("任せたもの タブ", "white")], [("滞りを通知", "gray")]),
             ("A4 答える", "質問に答える", [("QA を一本化", "green")], []),
             ("A5 振り返る", "滞りを見る", [("ボードの役割を明記", "white")], [("週次の要約", "gray")])]
mc = "".join(
    f'<div style="flex:1;display:flex;flex-direction:column;gap:6px"><div style="background:{ACC};color:{BG};border-radius:8px;padding:8px;font-size:20px;font-weight:700;text-align:center">{a}</div>'
    f'<div style="background:#FFF6C9;border:1px solid #E6D88A;border-radius:8px;padding:6px 10px;font-size:19px">{s}</div>'
    f'<div style="min-height:96px">{"".join(scard(t, k) for t, k in r1)}</div>'
    f'<div style="border-top:2px dashed #9AA0B0;padding-top:4px;min-height:60px">{"".join(scard(t, k) for t, k in r2)}</div></div>' for a, s, r1, r2 in mock_cols)
frame("mock", "12 · 画面: S15 ストーリーマップ", "カードの色で、つながったイシューの進み具合がわかる",
      f"""<div style="display:flex;gap:32px">
<div style="flex:1.25;background:#FFFFFF;border:1px solid {LINE};border-radius:16px;padding:20px;display:flex;flex-direction:column;gap:12px">
<div style="display:flex;gap:12px;align-items:center;font-size:20px;color:{SUB}"><b style="color:{INK}">TAPP · ストーリーマップ</b><span>（イメージ・抜粋）</span><span>ペルソナ: P1 オーナー ▾</span><span style="margin-left:auto;color:{RED}">抜け 3 件</span></div>
<div style="display:flex;gap:10px;align-items:flex-start"><div style="width:44px;font-family:{MONO};font-size:18px;color:{SUB};padding-top:100px;line-height:4.4">R1<br>R2</div>{mc}</div>
</div>
<div style="flex:1;display:flex;flex-direction:column;gap:16px;font-size:23px;line-height:1.55;color:{SUB}">
<p>{b("URL")}：/p/:key/storymap?persona=P1。プロジェクトのイシュー一覧のヘッダーから入る</p>
<p>{b("色")}：灰 = 条件なし、白 = 着手可能、黄 = 実装中、緑 = つながったイシューがすべて完了（M2 からはテスト結果も）</p>
<p>{b("S16 ストーリー詳細")}：サイドピーク。ペルソナ → 手順 → ストーリー → 条件 → イシューを縦に。条件の追加・並べ替え、イシューのつなぎ外し</p>
<p>{b("S09 イシュー詳細")}：「このイシューが満たすストーリー」の欄を足す</p>
<p>ペルソナ・ジョブ・活動は S15 の左のパネルで編集（別の画面は作らない）</p>
</div>
</div>""", gap=28)

# 14 tests & trial ----------------------------------------------------------
frame("trial", "13 · テストと試し方", "API テストで振る舞いを固定し、本番の TAPP で回してみる",
      f"""<div style="display:flex;gap:28px">
{card("API テスト", "apitest/tests/storymap.test.ts：作成・更新・アーカイブ、キーの連番（使い回さない）、つながり、gaps、不正な入力（他プロジェクトの手順につなぐ等）")}
{card("画面テスト", "e2e/tests/storymap.spec.ts：マップの表示、ストーリーの追加と手順への配置、条件の追加、イシューとのつなぎ、色の変化", color=TEAL)}
{card("Go の単体テスト", "キーの採番", color=PURPLE)}
</div>
{dark(f'試し方：M1 をデプロイしたら、この資料のペルソナ・ジョブ・マップを本番の TAPP に API で登録する。<br>棚卸しの判定をストーリーとして置き、<b style="color:{ACC_L}">整理の案 T1〜T5 を最初のリリース R1</b> として M1 の仕組みで回す')}""", gap=32)

# 15 questions --------------------------------------------------------------
qs = [("1. 整理の案 T1〜T5", "どれを行うか。M1 より先に小さな PR で出すか、M1 の最初のリリース R1 として M1 の仕組みで回すか"),
      ("2. 案 A か案 B か", "ストーリーと受け入れ条件を新しいテーブルで持つ案 A（おすすめ）か、今のストーリーのイシューと受入基準を使う案 B か"),
      ("3. キーの形", "TAPP-P1・TAPP-ST12・TAPP-AC42 のような形でよいか"),
      ("4. M1 の範囲", "L1〜L6・つながり・マップ画面・get_issue の「なぜ」、でよいか")]
frame("questions", "14 · 確認したいこと", "答えをもらってから M1 を実装する",
      f"""<div style="display:flex;flex-direction:column;gap:20px;flex:1">{''.join(card(t, d) for t, d in qs)}</div>""", gap=32)

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "マイタスクの棚卸しと M1 設計",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
