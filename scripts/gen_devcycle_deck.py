"""Generate docs/slides/src/devcycle/ (deck.json + slides/*.html)."""
import json
import pathlib

OUT = pathlib.Path(__file__).resolve().parent.parent / "docs" / "slides" / "src" / "devcycle"
ACC = "#5B58D6"     # accent
ACC_L = "#A9A7FF"
BG = "#F5F4EF"
INK = "#1B1F2A"
SUB = "#4A5160"
CARD = "#FCFBF8"
LINE = "#DDDBD3"
ZEBRA = "#ECEBE4"
MONO = "'JetBrains Mono', 'Courier New', monospace"
SANS = "'Noto Sans JP', Arial, sans-serif"

slides: list[tuple[str, str]] = []


def frame(sid, kicker, title, body, gap=40):
    slides.append((sid, f"""<section id="{sid}" data-transition="fade" style="background:{BG};color:{INK};font-family:{SANS};padding:96px 112px 128px;display:flex;flex-direction:column;gap:{gap}px">
<div style="display:flex;flex-direction:column;gap:12px">
<p style="font-family:{MONO};font-size:24px;color:{ACC};letter-spacing:2px">{kicker}</p>
<h2 style="font-size:56px;font-weight:700;line-height:1.25;color:{INK}">{title}</h2>
</div>
{body}
<p style="position:absolute;left:112px;bottom:52px;font-size:22px;color:#7A8091">__PAGE__</p>
</section>"""))


def card(title, text, color=ACC, extra=""):
    return (f'<div style="flex:1;display:flex;flex-direction:column;gap:12px;background:{CARD};border:1px solid {LINE};'
            f'border-radius:16px;padding:28px 32px;{extra}"><h3 style="font-size:30px;font-weight:700;color:{color}">{title}</h3>'
            f'<div style="font-size:24px;line-height:1.6;color:{SUB}">{text}</div></div>')


def table(headers, rows, widths, size=24):
    th = "".join(f'<th style="width:{w};color:{BG};padding:0.45em 0.7em">{h}</th>' for h, w in zip(headers, widths))
    trs = []
    for i, r in enumerate(rows):
        bg = f' style="background:{ZEBRA}"' if i % 2 else ""
        trs.append(f"<tr{bg}>" + "".join(f'<td style="padding:0.45em 0.7em">{c}</td>' for c in r) + "</tr>")
    return f'<table style="font-size:{size}px;line-height:1.45;color:{INK}"><tr style="background:{ACC}">{th}</tr>{"".join(trs)}</table>'


def b(s):
    return f'<b style="color:{INK}">{s}</b>'


# 1 cover -------------------------------------------------------------------
slides.append(("cover", f"""<section id="cover" data-transition="fade" style="background:{INK};color:{BG};font-family:{SANS};padding:128px;display:flex;flex-direction:column;justify-content:space-between">
<div style="position:absolute;left:0;top:0;width:24px;height:1080px;background:{ACC}"></div>
<p style="font-family:{MONO};font-size:28px;color:{ACC_L};letter-spacing:2px">PROPOSAL · DEV CYCLE</p>
<div style="display:flex;flex-direction:column;gap:32px">
<h1 style="font-size:88px;font-weight:900;line-height:1.2;color:{BG}">タスク管理から<br>アプリ開発アプリへ</h1>
<p style="font-size:38px;color:#B9BED0;line-height:1.5">ペルソナ・ジョブ・ストーリーマップ・受け入れ条件・設計・タスク・テストを<br>1 本の糸でつなぎ、開発サイクルを回す仕組みの提案</p>
</div>
<div style="display:flex;gap:48px;align-items:center">
<p style="font-size:28px;color:#B9BED0">AIエージェントと人間のタスク管理アプリ</p>
<p style="font-size:28px;color:#B9BED0">2026-11-01 · 提案（未決定）</p>
</div>
</section>"""))

# 2 problem -----------------------------------------------------------------
frame("problem", "01 · きっかけ", "マイタスクの使い方が、作った本人にもわからなくなってきた",
      f"""<div style="display:flex;gap:32px;flex:1">
{card("起きていること", "ティア・保存ビュー・リスト/ツリー・ボード・マップ・QA受信箱…と機能が増えるたびに、<br>「どれをいつ使うのか」の分担があいまいになっている")}
{card("原因", f"要求仕様（docs/01-requirements.md）は {b('「何ができるか」')}で整理されている。<br>{b('「誰が・どんな場面で・何のために」')}が書かれていないので、機能の置き場所と役割を判断する軸がない")}
{card("やりたいこと", "アプリを作るときにユースケースを考え、それを満たしていく進め方を開発に組み込みたい。<br>さらに、それを支える仕組みをアプリ自身に持たせたい")}
</div>
<div style="background:{INK};color:{BG};border-radius:16px;padding:28px 36px;font-size:28px;line-height:1.6">
この資料の提案：タスクを管理するアプリから一段上がり、{'<b style="color:' + ACC_L + '">要求から実装・検証までを 1 本の糸でつなぎ、開発サイクルを回すアプリ</b>'}にする
</div>""")

# 3 methods -----------------------------------------------------------------
frame("methods", "02 · 世の中の手法", "要求を扱う手法は、それぞれ別の問いに答えている",
      table(["手法", "何を決めるか", "特徴", "このアプリとの相性"], [
          [b("ペルソナ"), "誰が使うか", "利用者像を 2〜3 人つくり、迷ったら「この人なら使うか？」で判断する", "◎ 利用者は実在（本人と AI エージェント）。架空にせず実在の役割で書ける"],
          [b("ジョブ理論（JTBD）"), "なぜ使うか", "「〇〇な状況で、〇〇したい。そうすれば〇〇できる」。機能ではなく状況から考える", "◎ 「朝、今日やることを決めたい」のような場面が書ける"],
          [b("ユースケース（Cockburn）"), "目的までの手順", "基本の流れと例外の流れを手順として書く", "○ 厳密だが重い。API 仕様と相性がよい"],
          [b("ストーリーマッピング（Patton）"), "使う流れ全体と、どこまで作るか", "横に利用の流れ（活動 → 手順）、その下にストーリー。横線でリリース単位に切る", "◎ 「使い方」の全体像そのもの。今回の悩みに一番効く"],
          [b("インパクトマッピング"), "機能は本当に必要か", "ゴール → 誰が → 行動がどう変わるか → 機能、の木", "○ 機能を減らす判断に使える"],
          [b("BDD / 例示マッピング"), "完成の条件", "受け入れ条件を Given/When/Then で書き、テストにつなげる", "◎ 既存の e2e・apitest にそのままつながる"],
          [b("トレーサビリティ"), "要求とテストの対応", "要求 ID → 画面 → テストの対応表", "○ 要求 ID はすでにある。拡張するだけ"],
      ], ["19%", "15%", "35%", "31%"], size=23), gap=32)

# 4 core idea ---------------------------------------------------------------
frame("core", "03 · 中心となる考え方", "要求は「生き続ける仕様」、タスクは「使い切るもの」",
      f"""<div style="display:flex;gap:32px;flex:1">
{card("要求（L0〜L6）", f"ペルソナ・ジョブ・活動・手順・ストーリー・受け入れ条件。<br>{b('完了で閉じない。')}版を重ねて育つ。<br>「この機能は何のためにあるか」をアプリの中に残す役目", extra="border-top:8px solid " + ACC)}
{card("タスク（L8）", f"要求を満たすために一時的に発生し、{b('終われば閉じる。')}<br>人か AI エージェントが担当する。<br>今のタスクアプリが扱っているのはこちらだけ", color="#2F7D8C", extra="border-top:8px solid #2F7D8C")}
</div>
<div style="background:{INK};color:{BG};border-radius:16px;padding:28px 36px;font-size:28px;line-height:1.6">
今のアプリが使いにくくなったのは「使い切る側」しか持っていないから。<br>要求の側を足し、両者を<b style="color:{ACC_L}">つなぐ</b>ことで、機能の存在理由と完成度が見えるようにする
</div>""")

# 5 layers ------------------------------------------------------------------
def layer_group(label, color, items):
    rows = "".join(
        f'<div style="display:flex;gap:20px;align-items:baseline;padding:10px 0;border-bottom:1px solid {LINE}">'
        f'<span style="font-family:{MONO};font-size:22px;color:{color};width:64px;flex:none">{k}</span>'
        f'<span style="font-size:25px;font-weight:700;width:230px;flex:none">{n}</span>'
        f'<span style="font-size:23px;color:{SUB}">{d}</span></div>' for k, n, d in items)
    return (f'<div style="display:flex;gap:24px"><div style="width:120px;flex:none;background:{color};color:{BG};border-radius:12px;'
            f'display:flex;align-items:center;justify-content:center;font-size:30px;font-weight:700">{label}</div>'
            f'<div style="flex:1">{rows}</div></div>')


frame("layers", "04 · 粒度の階層", "なぜ → 何を → どう → 確認、の 11 段で上から下へ落とす",
      f"""<div style="display:flex;flex-direction:column;gap:14px">
{layer_group("なぜ", "#8A5BB8", [("L0", "プロダクト", "ビジョン・ゴール・指標"), ("L1", "ペルソナ", "誰が使うか"), ("L2", "ジョブ", "どんな状況で何をしたいか（JTBD）")])}
{layer_group("何を", ACC, [("L3", "活動", "ストーリーマップの背骨（例：朝、今日を決める）"), ("L4", "手順", "活動を分けたもの（例：注力だけを見る）"), ("L5", "ストーリー", "手順を楽にする機能の単位 ＋ リリースへの割り当て"), ("L6", "受け入れ条件", "Given/When/Then。仕様の最小単位")])}
{layer_group("どう", "#2F7D8C", [("L7", "設計要素", "画面・API・データ（例：S14、PATCH /projects、projects.tier）"), ("L8", "開発タスク", "実装・テスト作成。人か AI エージェントに割り当てる（既存の課題）")])}
{layer_group("確認", "#B85A1E", [("L9", "検証", "テスト（e2e / apitest）とその実行結果"), ("L10", "フィードバック", "使ってみての気づき → L2〜L5 に戻る")])}
</div>""", gap=28)

# 6 links -------------------------------------------------------------------
def box(t, color=ACC, w=200):
    return (f'<div style="width:{w}px;flex:none;background:{CARD};border:3px solid {color};border-radius:14px;padding:18px 8px;'
            f'text-align:center;font-size:26px;font-weight:700">{t}</div>')


def arr(lbl, vertical=False):
    if vertical:
        return (f'<div style="display:flex;flex-direction:column;align-items:center;gap:4px;font-family:{MONO};font-size:20px;color:{SUB}">'
                f'<x-shape kind="arrow-down" style="width:28px;height:44px;background:{SUB}"></x-shape>{lbl}</div>')
    return (f'<div style="flex:1;display:flex;flex-direction:column;align-items:center;gap:4px;font-family:{MONO};font-size:20px;color:{SUB}">'
            f'<x-shape kind="arrow-right" style="width:52px;height:28px;background:{SUB}"></x-shape>{lbl}</div>')


chain = "".join([
    box("ペルソナ", "#8A5BB8", 170), arr("1:N"), box("ジョブ", "#8A5BB8", 150), arr("N:N"), box("活動", ACC, 130), arr("1:N"),
    box("手順", ACC, 130), arr("N:N"), box("ストーリー", ACC, 190), arr("1:N"), box("受け入れ条件", ACC, 220), arr("N:N"), box("テスト", "#B85A1E", 140),
])
frame("links", "05 · つなぎ方", "すべての項目を ID でつなぎ、どこからでも上下にたどれる",
      f"""<div style="position:relative;height:520px">
<div style="position:absolute;left:0;right:0;top:40px;display:flex;align-items:center">{chain}</div>
<div style="position:absolute;left:1012px;top:150px;display:flex;flex-direction:column;align-items:center;gap:8px">{arr("1:N", True)}{box("開発タスク<br><span style='font-size:20px;font-weight:400'>既存の課題</span>", "#2F7D8C", 200)}</div>
<div style="position:absolute;left:1270px;top:150px;display:flex;flex-direction:column;align-items:center;gap:8px">{arr("N:N", True)}{box("設計要素<br><span style='font-size:20px;font-weight:400'>画面 / API / データ</span>", "#2F7D8C", 260)}</div>
<div style="position:absolute;left:0;top:420px;display:flex;align-items:center;gap:20px">{box("フィードバック", "#B85A1E", 220)}<x-shape kind="arrow-right" style="width:80px;height:28px;background:#B85A1E"></x-shape><span style="font-size:24px;color:{SUB}">手順・ストーリーに戻して次のサイクルへ</span></div>
</div>
<div style="display:flex;gap:32px">
{card("上からたどる", "このペルソナのジョブは、どのストーリーで満たされ、どのテストで保証されているか")}
{card("下からたどる", "このテスト（画面・API）は、誰のどの場面のためにあるのか。答えられなければ孤立")}
</div>""", gap=24)

# 7 example -----------------------------------------------------------------
cols = [("朝：今日を決める", ["注力だけ見る", "期限の近いものを見る"]),
        ("日中：進める", ["状態を変える", "メモを残す"]),
        ("AI に任せる", ["起票して割り当てる", "粒度を決める"]),
        ("AI に答える", ["QA受信箱を見る", "Yes/No で答える"]),
        ("振り返る", ["終わったものを見る", "その他を整理する"])]
grid = "".join(
    f'<div style="flex:1;display:flex;flex-direction:column;gap:12px">'
    f'<div style="background:{ACC};color:{BG};border-radius:12px;padding:16px;font-size:26px;font-weight:700;text-align:center">{a}</div>'
    + "".join(f'<div style="background:#FFF6C9;border:1px solid #E6D88A;border-radius:10px;padding:14px;font-size:24px;text-align:center">{s}</div>' for s in steps)
    + "</div>" for a, steps in cols)
frame("example", "06 · 例：マイタスクに当てはめる", "ペルソナ P1（オーナー）のストーリーマップ",
      f"""<div style="display:flex;gap:28px">
{card("P1 オーナー（あなた）", "複数のプロジェクトを並行して進め、作業を AI に任せる")}
{card("P2 AI エージェント", "MCP で作業を拾い、チェックポイントを記録し、迷ったら質問する", color="#2F7D8C")}
</div>
<div style="display:flex;gap:20px">{grid}</div>
<p style="font-size:26px;line-height:1.6;color:{SUB}">並べると「リスト/ツリーの切り替えは、どの手順で使うのか」に答えられるかがはっきりする。<br>答えられない機能は、削るか、隠すか、手順を足すかを決める。<span style="color:#7A8091">（内容は例。これから一緒に決める）</span></p>""", gap=28)

# 8 cycle -------------------------------------------------------------------
stages = [("① 発見", "ペルソナ・ジョブ・フィードバックを整理"), ("② 定義", "ストーリーマップに手順とストーリーを置く"),
          ("③ 分解", "受け入れ条件 → 設計要素 → タスク"), ("④ 作る", "AI エージェント中心に実装"),
          ("⑤ 確かめる", "CI の結果を取り込み、人が受け入れ確認"), ("⑥ 学ぶ", "使った気づきをフィードバックに")]
row = []
for i, (n, d) in enumerate(stages):
    row.append(f'<div style="flex:1;display:flex;flex-direction:column;gap:12px;background:{CARD};border:1px solid {LINE};border-top:8px solid {ACC};border-radius:16px;padding:24px">'
               f'<h3 style="font-size:32px;color:{ACC}">{n}</h3><p style="font-size:23px;line-height:1.55;color:{SUB}">{d}</p></div>')
    if i < len(stages) - 1:
        row.append(f'<x-shape kind="arrow-right" style="width:34px;height:26px;background:{SUB};align-self:center"></x-shape>')
frame("cycle", "07 · 開発サイクル", "6 つの段階を回し、学びを次の発見に戻す",
      f"""<div style="display:flex;gap:12px">{''.join(row)}</div>
<div style="display:flex;align-items:center;gap:20px;font-size:26px;color:#B85A1E;padding-left:8px">
<x-shape kind="arrow-left" style="width:44px;height:30px;background:#B85A1E"></x-shape><div style="flex:1;height:8px;margin-left:-6px;background:#B85A1E;border-radius:4px"></div><span style="white-space:nowrap">⑥ から ① へ</span></div>
<div style="display:flex;gap:28px">
{card("段階ごとに関門を置く", "次へ進む条件をシステムが自動で判定する（次のページ）。人が毎回チェックリストを見なくてよい")}
{card("どこが抜けているかが常に見える", "着手可能でないストーリー、テストのない条件、つながっていない機能が一覧になる")}
</div>""", gap=32)

# 9 gates -------------------------------------------------------------------
frame("gates", "08 · 関門", "次へ進む条件を、つながりの有無から自動で判定する",
      table(["段階", "やること", "次へ進む条件"], [
          [b("① 発見"), "ペルソナ・ジョブ・フィードバックを整理する", "ジョブが 1 つ以上の活動に結びついている"],
          [b("② 定義"), "ストーリーマップに手順とストーリーを置く", f"{b('着手可能')}：手順につながっている／受け入れ条件が 1 つ以上／設計要素が特定済み"],
          [b("③ 分解"), "受け入れ条件 → 設計要素 → 開発タスクに落とす", "すべての受け入れ条件に担当タスクと予定テストがある"],
          [b("④ 作る"), "実装する（AI エージェント中心。チェックポイントと質問は既存の仕組み）", "タスクがすべて完了"],
          [b("⑤ 確かめる"), "CI のテスト結果を取り込み、人が受け入れ確認", f"{b('完了')}：全条件のテストが成功し、ガイドも更新済み"],
          [b("⑥ 学ぶ"), "使ってみての気づきをフィードバックとして登録", "影響するストーリーが特定され、① に戻る"],
      ], ["14%", "43%", "43%"], size=25))

# 10 automation -------------------------------------------------------------
auto = [("カバレッジ付きストーリーマップ", "緑＝検証済み、黄＝実装中、灰＝未定義、赤＝テスト失敗。マップを見るだけで完成度がわかる"),
        ("孤立チェック", "どの手順にもつながらない機能・画面・テストを一覧に。「何のための機能か」を検出する"),
        ("使い方ガイドの自動生成", "活動を章、手順を節として書き出す。docs/06-guide.md を置き換える"),
        ("トレース表とスライドの生成", "要求 → 設計 → テストの対応表と設計スライド。今の手作業の docs/ を置き換える"),
        ("テストとの結びつけ", "テスト名にタグ（例 @AC-142）。CI の Playwright / vitest のレポートを取り込み、条件ごとに成否を反映"),
        ("コードとの結びつけ", "コミットや PR の本文に ID を書くと自動で紐づく")]
cards = [card(t, d) for t, d in auto]
frame("automation", "09 · 自動でできるようにしたいこと", "つながっているから、状態の集計と書類づくりは機械に任せられる",
      f"""<div style="display:flex;flex-direction:column;gap:24px;flex:1">
<div style="display:flex;gap:24px;flex:1">{''.join(cards[:3])}</div>
<div style="display:flex;gap:24px;flex:1">{''.join(cards[3:])}</div>
</div>""", gap=32)

# 11 AI ---------------------------------------------------------------------
frame("ai", "10 · AI エージェントの役割", "下書きは AI、決めるのは人",
      table(["作業", "AI", "人"], [
          ["ストーリーから受け入れ条件を下書きする", "○ 下書き", "承認する"],
          ["受け入れ条件から設計要素とタスクに分解する", "○ 下書き", "承認する"],
          ["実装とテストを書く", "○ 実施", "必要に応じて質問に答える"],
          ["フィードバックから影響するストーリーを推定する", "○ 推定", "優先度を決める"],
          ["ゴール・ペルソナ・リリースの線引き", "提案のみ", b("決める")],
      ], ["50%", "18%", "32%"], size=26)
      + f"""<div style="background:{INK};color:{BG};border-radius:16px;padding:28px 36px;font-size:27px;line-height:1.6">
AI は MCP で上の階層まで読めるので、<b style="color:{ACC_L}">「なぜこれを作るのか」を知ったうえで作業できる</b>。<br>判断の質が上がり、質問やチェックポイントも要求 ID に結びつく
</div>""")

# 12 screens ----------------------------------------------------------------
scr = [("① プロダクトのホーム", "ゴールと指標、サイクルの現在地、関門で止まっている項目"),
       ("② ペルソナとジョブ", "誰が、どんな状況で何をしたいか"),
       ("③ ストーリーマップ", "メイン画面。横に活動と手順、縦にリリース。カバレッジの色付き"),
       ("④ ストーリー詳細", "ペルソナから手順・条件・設計・タスク・テストまでを 1 画面で縦に見せる"),
       ("⑤ 健全性", "孤立・抜け・失敗テストの一覧"),
       ("⑥ フィードバック受信箱", "使って気づいたことを登録し、ストーリーに振り分ける"),
       ("⑦ 課題・ボード・QA受信箱", "既存。L8 の開発タスクを扱う場所としてそのまま使う")]
sc = [card(t, d, color=("#2F7D8C" if t.startswith("⑦") else ACC)) for t, d in scr]
frame("screens", "11 · 画面構成のイメージ", "ストーリーマップを中心に、上下の階層へ移る",
      f"""<div style="display:flex;flex-direction:column;gap:24px;flex:1">
<div style="display:flex;gap:24px;flex:1">{''.join(sc[:4])}</div>
<div style="display:flex;gap:24px;flex:1">{''.join(sc[4:])}</div>
</div>""", gap=32)

# 13 build ------------------------------------------------------------------
frame("build", "12 · 作り方", "今のタスクアプリを土台に、要求の層を上に足す",
      f"""<div style="display:flex;gap:32px;flex:1">
{card("そのまま使うもの", "課題・MCP・質問・チェックポイント・受け入れ依頼。<br>L8（開発タスク）と L9（検証）の受け皿になる", color="#2F7D8C")}
{card("新しく足すもの", f"要求（L0〜L6）と設計要素（L7）は{b('課題タイプとしてではなく別のテーブル')}で持つ。<br>完了で閉じない・版を重ねる、というライフサイクルが課題と違うため")}
{card("今までどおり", "API を先に定義し、apitest で振る舞いを固定する。<br>将来の Rust への書き直しでも同じテストが使える")}
</div>
<div style="background:{INK};color:{BG};border-radius:16px;padding:28px 36px;font-size:27px;line-height:1.6">
別アプリにしない理由：開発タスク・AI エージェント接続・質問の仕組みをゼロから作り直すことになり、<br>しかも要求と開発タスクのつなぎ目が 2 つのアプリにまたがってしまう
</div>""")

# 14 milestones -------------------------------------------------------------
frame("milestones", "13 · 段階的な進め方", "まずこのアプリ自身の要求で試す",
      table(["段階", "内容", "試す対象"], [
          [b("M1"), "要求モデル（L0〜L6）、ストーリーマップ表示、ストーリーとタスクの紐付け", "このアプリ自身（TAPP）の要求を登録する。まずマイタスク周り"],
          [b("M2"), "受け入れ条件とテストタグの紐付け、CI 結果の取り込み、カバレッジ色", "既存の e2e と apitest にタグを付ける"],
          [b("M3"), "関門の自動判定、AI による分解の下書き、ガイドとスライドの生成", "docs/ の手作業を置き換えられるか確かめる"],
          [b("M4"), "フィードバックの循環、指標", "日々使った気づきを登録して次のサイクルを回す"],
      ], ["10%", "55%", "35%"], size=26)
      + f"""<p style="font-size:26px;line-height:1.6;color:{SUB}">M1 の前に、アプリを変えずに「今の機能をストーリーマップに貼る棚卸し」を文書で行うと、<br>使い方がわかりにくくなった原因（孤立・重複した機能）が先に見える</p>""")

# 15 questions --------------------------------------------------------------
qs = [("1. 別アプリか、拡張か", "おすすめは今のタスクアプリの拡張。開発タスク・MCP・質問の仕組みをそのまま使えるため"),
      ("2. 利用者", "あなたと AI エージェントだけを想定してよいか。将来チームで使うなら、権限やレビューの設計が変わる"),
      ("3. 最初の題材", "このアプリ自身の要求（特にわかりにくくなったマイタスク周り）で M1 を試してよいか")]
frame("questions", "14 · 確認したいこと", "方向が決まったら、設計書とスライドに詳しくまとめる",
      f"""<div style="display:flex;flex-direction:column;gap:24px;flex:1">{''.join(card(t, d) for t, d in qs)}</div>""")

# write ---------------------------------------------------------------------
(OUT / "slides").mkdir(parents=True, exist_ok=True)
for old in (OUT / "slides").glob("*.html"):
    old.unlink()
n = len(slides)
for k, (sid, html) in enumerate(slides, 1):
    (OUT / "slides" / f"{sid}.html").write_text(html.replace("__PAGE__", f"{k} / {n}") + "\n")
deck = {
    "v": 4, "lists": "css", "title": "アプリ開発アプリ構想（提案）",
    "order": [s for s, _ in slides],
    "faces": {
        "noto-sans-jp": {"family": "Noto Sans JP", "href": "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700;900&display=swap"},
        "jetbrains-mono": {"family": "JetBrains Mono", "href": "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap"},
    },
}
(OUT / "deck.json").write_text(json.dumps(deck, ensure_ascii=False) + "\n")
print(n, "slides")
