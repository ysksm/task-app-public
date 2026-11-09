import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { api, unwrap } from "../api/client";
import { AlertTriangle, ArrowRight, Bot, Check, Gauge, Copy, DatabaseBackup, Download, Eye, EyeOff, KeyRound, MessageCircleQuestion, MonitorSmartphone, Plus, Settings, Trash2, X } from "lucide-react";
import { useState, type ReactNode } from "react";
import { ISSUE_LOAD_LIMIT, useAddUser, useAgentFeed, useBackups, useCapacity, useCreateBackup, useIssueToken, useMe, useQAList, useRevealToken, useRevokeToken, useSetPassword, useTokens, useUsers, type QAListParams } from "../api/hooks";
import type { ApiError, User } from "../api/client";
import { CheckpointCard } from "../components/CheckpointCard";
import { Avatar, TypeIcon } from "../components/icons";
import { IssueDetail } from "../components/IssueDetail";
import { QAPanel } from "../components/QAPanel";
import { relativeTime } from "../lib/meta";

export function Header({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <header className="flex h-11 shrink-0 items-center gap-2 border-b border-line px-4">
      {icon}
      <h1 className="text-[14px] font-medium">{title}</h1>
      {children}
    </header>
  );
}

export function Tabs<T extends string>({ value, options, onChange }: { value: T; options: [T, string][]; onChange: (v: T) => void }) {
  return (
    <div className="ml-3 flex gap-1">
      {options.map(([v, l]) => (
        <button key={v} onClick={() => onChange(v)}
          className={`rounded-md px-2.5 py-1 text-[12px] ${value === v ? "bg-active text-fg" : "text-muted hover:bg-hover hover:text-fg"}`}>{l}</button>
      ))}
    </div>
  );
}

// ---- QA inbox ----

export type InboxTab = "open" | "answered" | "closed" | "all";

export function QAInboxPage({ tab, mine, selected, setSearch }: {
  tab: InboxTab;
  mine: boolean;
  selected?: string;
  setSearch: (s: { tab?: InboxTab; all?: boolean; key?: string }) => void;
}) {
  const params: QAListParams = { ...(tab !== "all" ? { status: tab } : {}), ...(mine ? { assignee: "me" } : {}) };
  const { data: list = [], isLoading } = useQAList(params);
  const navigate = useNavigate();
  const current = selected ?? list[0]?.key;
  const openIssue = (k: string) => navigate({ to: "/p/$key/issues", params: { key: k.split("-")[0] }, search: { peek: k } });
  return (
    <div className="flex h-full flex-col">
      <Header icon={<MessageCircleQuestion size={15} className="text-muted" />} title="QA インボックス">
        <Tabs value={tab} onChange={(t) => setSearch({ tab: t, all: !mine || undefined })}
          options={[["open", "未回答"], ["answered", "回答済"], ["closed", "クローズ"], ["all", "すべて"]]} />
        <label className="flex items-center gap-1.5 text-[12px] text-muted">
          <input type="checkbox" checked={mine} onChange={(e) => setSearch({ tab, all: !e.target.checked || undefined })} className="accent-[var(--accent)]" />
          自分宛てのみ
        </label>
      </Header>
      <div className="flex min-h-0 flex-1">
        <div className="w-[340px] shrink-0 overflow-y-auto border-r border-line">
          {isLoading && <div className="p-4 text-faint">読み込み中...</div>}
          {!isLoading && list.length === 0 && <div className="p-6 text-center text-[13px] text-faint">QA はありません</div>}
          {list.map((q) => {
            const answered = q.items.filter((i) => i.answer).length;
            return (
              <button key={q.key} onClick={() => setSearch({ tab, all: !mine || undefined, key: q.key })}
                className={`block w-full border-b border-line/60 px-4 py-3 text-left hover:bg-hover ${q.key === current ? "bg-active" : ""}`}>
                <div className="mb-1 flex items-center gap-1.5 text-[11.5px] text-faint">
                  {q.is_blocking && q.status === "open" && <AlertTriangle size={12} className="text-[#e5484d]" />}
                  <span className="font-mono">{q.key}</span>
                  <span>·</span>
                  <Avatar user={q.asked_by} size={13} />
                  <span>{q.asked_by.display_name}</span>
                  <span className="ml-auto">{relativeTime(q.created_at)}</span>
                </div>
                <div className="line-clamp-2 text-[13px] leading-snug">{q.title}</div>
                <div className="mt-1 flex items-center gap-2 text-[11.5px] text-faint">
                  <span>質問 {q.items.length} 件</span>
                  {answered > 0 && <span>· 回答 {answered}/{q.items.length}</span>}
                  {q.target_key && <span className="ml-auto font-mono">→ {q.target_key}</span>}
                </div>
              </button>
            );
          })}
        </div>
        <div className="min-w-0 flex-1 overflow-y-auto">
          {current ? (
            <div className="mx-auto max-w-[760px] px-6 py-5">
              <QAInboxDetail qaKey={current} onOpenIssue={openIssue} />
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function QAInboxDetail({ qaKey, onOpenIssue }: { qaKey: string; onOpenIssue: (k: string) => void }) {
  const { data: list } = useQAList({});
  const title = list?.find((q) => q.key === qaKey)?.title;
  return (
    <>
      <div className="mb-3 flex items-center gap-2">
        <TypeIcon type="qa" size={16} />
        <h2 className="text-[18px] font-semibold">{title}</h2>
        <button onClick={() => onOpenIssue(qaKey)} className="ml-auto flex items-center gap-1 text-[12px] text-muted hover:text-fg">
          {qaKey} を開く <ArrowRight size={12} />
        </button>
      </div>
      <QAPanel key={qaKey} qaKey={qaKey} onOpenIssue={onOpenIssue} />
    </>
  );
}

// ---- agent activity ----

export function AgentsPage({ agent, low, peek, setSearch }: {
  agent?: string;
  low: boolean;
  peek?: string;
  setSearch: (s: { agent?: string; low?: boolean; peek?: string }) => void;
}) {
  const { data: users = [] } = useUsers();
  const { data: entries = [], isLoading } = useAgentFeed({ agent, low_confidence: low || undefined });
  const agents = users.filter((u) => u.kind === "agent");
  const open = (k: string) => setSearch({ agent, low: low || undefined, peek: k });
  let lastDay = "";
  return (
    <div className="flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <Header icon={<Bot size={15} className="text-muted" />} title="エージェント活動">
          <div className="ml-3 flex gap-1">
            <button onClick={() => setSearch({ low: low || undefined })}
              className={`rounded-md px-2.5 py-1 text-[12px] ${!agent ? "bg-active" : "text-muted hover:bg-hover"}`}>すべて</button>
            {agents.map((a) => (
              <button key={a.username} onClick={() => setSearch({ agent: a.username, low: low || undefined })}
                className={`flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] ${agent === a.username ? "bg-active" : "text-muted hover:bg-hover"}`}>
                <Avatar user={a} size={14} />{a.display_name}
              </button>
            ))}
          </div>
          <label className="flex items-center gap-1.5 text-[12px] text-muted">
            <input type="checkbox" checked={low} onChange={(e) => setSearch({ agent, low: e.target.checked || undefined })} className="accent-[var(--accent)]" />
            <AlertTriangle size={12} className="text-[#e5484d]" /> 確信度「低」のみ
          </label>
        </Header>
        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[820px] space-y-2.5 px-6 py-5">
            {isLoading && <div className="text-faint">読み込み中...</div>}
            {!isLoading && entries.length === 0 && <div className="py-10 text-center text-faint">エージェントの活動はまだありません</div>}
            {entries.map((e, i) => {
              const day = new Date(e.at).toLocaleDateString("ja-JP", { month: "long", day: "numeric", weekday: "short" });
              const showDay = day !== lastDay;
              lastDay = day;
              return (
                <div key={i}>
                  {showDay && <div className="mb-2 mt-4 text-[12px] font-medium text-faint first:mt-0">{day}</div>}
                  {e.kind === "checkpoint" && e.checkpoint ? (
                    <CheckpointCard cp={e.checkpoint} issueLabel={`${e.issue.key} ${e.issue.title}`} onOpenIssue={open} />
                  ) : (
                    <button onClick={() => open(e.issue.key)} className="flex w-full items-center gap-2 rounded-lg border border-line bg-elev px-3.5 py-2.5 text-left text-[12.5px] hover:border-line-strong">
                      <Avatar user={e.actor} size={16} />
                      <span className="font-medium">{e.actor.display_name}</span>
                      {e.kind === "question" && e.question ? (
                        <>
                          <span className="text-muted">が質問</span>
                          <TypeIcon type="qa" size={13} />
                          <span className="truncate">{e.issue.title}</span>
                          {e.question.is_blocking && <span className="rounded bg-[#e5484d22] px-1.5 text-[11px] text-[#e5484d]">ブロッキング</span>}
                          <span className="text-faint">{{ open: "未回答", answered: "回答済", closed: "クローズ" }[e.question.status]}</span>
                        </>
                      ) : (
                        <>
                          <TypeIcon type={e.issue.type} size={13} />
                          <span className="font-mono text-[11.5px] text-muted">{e.issue.key}</span>
                          <span className="truncate">{e.issue.title}</span>
                          <span className="text-muted">を {e.change?.from} → <span className="text-fg">{e.change?.to}</span></span>
                        </>
                      )}
                      <span className="ml-auto shrink-0 text-faint">{relativeTime(e.at)}</span>
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
      {peek && (
        <div className="w-[min(720px,50%)] shrink-0 border-l border-line">
          <IssueDetail key={peek} issueKey={peek} mode="peek" onClose={() => setSearch({ agent, low: low || undefined })} onOpen={open} />
        </div>
      )}
    </div>
  );
}

// ---- settings ----

export function SettingsPage() {
  const { data: me } = useMe();
  const isOwner = !!me?.is_owner;
  const [pwFor, setPwFor] = useState<string | null>(null);
  const { data: users = [] } = useUsers();
  const { data: tokens = [] } = useTokens();
  const issue = useIssueToken();
  const revoke = useRevokeToken();
  const addUser = useAddUser();
  const [tokenUser, setTokenUser] = useState("");
  const [tokenName, setTokenName] = useState("");
  const reveal = useRevealToken();
  // The token shown in the box and filled into the MCP / Jira commands below.
  const [shown, setShown] = useState<{ id: string; token: string; user: string; name: string } | null>(null);
  const [agentName, setAgentName] = useState("");
  const [agentModel, setAgentModel] = useState("");
  const origin = window.location.origin;
  const { data: server } = useQuery({ queryKey: ["server"], queryFn: () => unwrap(api.GET("/server")), staleTime: Infinity });
  const bin = server?.binary ?? "taskapp";
  const dataArg = server?.data_dir ? ` --data ${server.data_dir}` : "";
  const tokenFor = shown?.token ?? "<token>";
  const jiraEmail = users.find((u) => (shown ? u.username === shown.user : u.is_owner))?.email ?? "";
  const input = "rounded-md border border-line bg-bg px-2 py-1 text-[13px] outline-none focus:border-accent";

  return (
    <div className="h-full overflow-y-auto">
      <Header icon={<Settings size={15} className="text-muted" />} title="設定" />
      <div className="mx-auto max-w-[860px] space-y-8 px-6 py-6">
        {me && <AccountSection me={me} />}

        <section>
          <h2 className="mb-2 text-[14px] font-semibold">メンバーとエージェント</h2>
          <div className="rounded-lg border border-line">
            {users.map((u) => (
              <div key={u.id} data-testid={`member-${u.username}`} className="flex items-center gap-3 border-b border-line/60 px-3 py-2 text-[13px] last:border-0">
                <Avatar user={u} size={20} />
                <span className="font-medium">{u.display_name}</span>
                <span className="font-mono text-[12px] text-faint">@{u.username}</span>
                <span className="text-[12px] text-muted">{u.kind === "agent" ? `エージェント${u.agent_model ? `（${u.agent_model}）` : ""}` : u.is_owner ? "オーナー" : "人間"}</span>
                <span className="flex-1" />
                {u.kind === "human" && (
                  <span className={`text-[11.5px] ${u.has_password ? "text-muted" : "text-faint"}`}>{u.has_password ? "パスワード設定済み" : "パスワード未設定"}</span>
                )}
                {isOwner && u.kind === "human" && u.id !== me?.id && (
                  <button onClick={() => setPwFor(pwFor === u.username ? null : u.username)}
                    className="rounded-md border border-line px-2 py-0.5 text-[11.5px] text-muted hover:bg-hover hover:text-fg">パスワードを設定</button>
                )}
              </div>
            ))}
          </div>
          {isOwner && pwFor && <div className="mt-2"><PasswordForm username={pwFor} onDone={() => setPwFor(null)} /></div>}
          {isOwner && <form className="mt-2 flex items-center gap-2" onSubmit={(e) => {
            e.preventDefault();
            addUser.mutate({ kind: "agent", username: agentName.trim().toLowerCase(), display_name: agentName.trim(), agent_model: agentModel.trim() || undefined },
              { onSuccess: () => (setAgentName(""), setAgentModel("")) });
          }}>
            <input value={agentName} onChange={(e) => setAgentName(e.target.value)} placeholder="エージェント名（例: gemini）" className={`${input} w-56`} />
            <input value={agentModel} onChange={(e) => setAgentModel(e.target.value)} placeholder="モデル（任意）" className={`${input} w-44`} />
            <button disabled={!agentName.trim()} className="flex items-center gap-1 rounded-md border border-line px-2.5 py-1 text-[12px] hover:bg-hover disabled:opacity-40"><Plus size={13} />エージェントを追加</button>
            {addUser.error && <span className="text-[12px] text-danger">{(addUser.error as Error).message}</span>}
          </form>}
        </section>

        <RemoteSection remoteUrl={server?.remote_url} bin={bin} dataArg={dataArg} />

        <section>
          <h2 className="mb-2 text-[14px] font-semibold">API トークン</h2>
          <p className="mb-2 text-[12.5px] text-muted">エージェント（MCP）や外部ツールの認証に使います。発行したトークンは一覧の目のボタンで後から表示でき、下の MCP・Jira のコマンドにも入ります。</p>
          <form className="mb-2 flex items-center gap-2" onSubmit={(e) => {
            e.preventDefault();
            issue.mutate({ user: tokenUser || undefined, name: tokenName.trim() }, {
              onSuccess: (r) => (setShown({ id: r.info.id, token: r.token, user: r.info.user.username, name: r.info.name }), reveal.reset(), setTokenName("")),
            });
          }}>
            {isOwner && <select value={tokenUser} onChange={(e) => setTokenUser(e.target.value)} className={input}>
              <option value="">自分</option>
              {users.map((u) => <option key={u.id} value={u.username}>{u.display_name}{u.kind === "agent" ? "（エージェント）" : ""}</option>)}
            </select>}
            <input value={tokenName} onChange={(e) => setTokenName(e.target.value)} placeholder="用途（例: claude-code）" className={`${input} w-56`} />
            <button disabled={!tokenName.trim()} className="flex items-center gap-1 rounded-md bg-accent px-2.5 py-1 text-[12px] font-medium text-white disabled:opacity-40"><KeyRound size={13} />発行</button>
            {issue.error && <span className="text-[12px] text-danger">{(issue.error as Error).message}</span>}
          </form>
          {shown && (
            <div className="mb-2 rounded-lg border border-[#3fae6a66] bg-[#3fae6a14] p-3 text-[12.5px]" data-testid="shown-token">
              <div className="mb-1 flex items-center gap-1 font-medium">
                <Check size={13} className="text-[#3fae6a]" />@{shown.user} のトークン「{shown.name}」。下の MCP・Jira のコマンドにも入っています。
                <span className="flex-1" />
                <button title="隠す" onClick={() => setShown(null)} className="rounded p-0.5 text-faint hover:bg-hover hover:text-fg"><X size={13} /></button>
              </div>
              <CopyLine text={shown.token} />
            </div>
          )}
          {reveal.error && (
            <div className="mb-2 text-[12.5px] text-danger">
              {(reveal.error as ApiError).code === "not_available"
                ? "このトークンは表示できません（後から表示できるようになる前に発行したか、鍵ファイル token.key が変わりました）。発行し直してください。"
                : (reveal.error as Error).message}
            </div>
          )}
          <div className="rounded-lg border border-line">
            {tokens.length === 0 && <div className="px-3 py-2 text-[12.5px] text-faint">トークンはありません</div>}
            {tokens.map((t) => (
              <div key={t.id} data-testid={`token-${t.name}`} className={`flex items-center gap-3 border-b border-line/60 px-3 py-2 text-[13px] last:border-0 ${shown?.id === t.id ? "bg-accent/10" : ""}`}>
                <Avatar user={t.user} size={18} />
                <span className="w-32 truncate">{t.name}</span>
                <span className="font-mono text-[12px] text-faint">{t.prefix}…</span>
                <span className="text-[12px] text-muted">@{t.user.username}</span>
                <span className="ml-auto text-[12px] text-faint">{t.last_used_at ? `最終使用 ${relativeTime(t.last_used_at)}` : "未使用"}</span>
                {shown?.id === t.id ? (
                  <button title="隠す" onClick={() => setShown(null)} className="rounded p-1 text-accent hover:bg-hover"><EyeOff size={14} /></button>
                ) : (
                  <button title="表示してコマンドに使う" disabled={reveal.isPending}
                    onClick={() => reveal.mutate(t.id, { onSuccess: (r) => setShown({ id: t.id, token: r.token, user: t.user.username, name: t.name }) })}
                    className="rounded p-1 text-faint hover:bg-hover hover:text-fg"><Eye size={14} /></button>
                )}
                <button title="失効" onClick={() => window.confirm(`トークン「${t.name}」を失効しますか？`) && revoke.mutate(t.id, { onSuccess: () => shown?.id === t.id && setShown(null) })}
                  className="rounded p-1 text-faint hover:bg-hover hover:text-danger"><Trash2 size={14} /></button>
              </div>
            ))}
          </div>
        </section>

        <McpSection token={tokenFor} origin={origin} remoteUrl={server?.remote_url} bin={bin} dataArg={dataArg} />

        <section>
          <h2 className="mb-2 text-[14px] font-semibold">Jira 互換 API（ORCA 連携）</h2>
          <p className="mb-2 text-[12.5px] text-muted">
            Jira REST API v3 互換のエンドポイント（<code className="rounded bg-hover px-1">/rest/api/3</code>）を提供しています。ORCA の Settings → Jira で
            <b className="font-medium text-fg"> Cloud </b>を選び、次の値を入力してください。トークンは上の「API トークン」で発行したものを使います。
          </p>
          <div className="grid grid-cols-[110px_1fr] items-center gap-x-3 gap-y-2 text-[12.5px]">
            <span className="text-faint">サイト URL</span>
            <CopyLine text={origin} />
            <span className="text-faint">メールアドレス</span>
            <CopyLine text={jiraEmail} />
            <span className="text-faint">API トークン</span>
            <CopyLine text={tokenFor} />
          </div>
          <p className="mt-2 text-[12px] text-faint">
            ORCA からの操作は「Jira API」経由として履歴に残り、画面に即時反映されます。QA（Question）は MCP からのみ起票できます。
          </p>
        </section>

        {isOwner && <CapacitySection />}

        {isOwner && <section>
          <h2 className="mb-2 text-[14px] font-semibold">データ（バックアップ・エクスポート）</h2>
          <p className="mb-2 text-[12.5px] text-muted">
            データは <code className="rounded bg-hover px-1">{server?.data_dir ?? "~/.taskapp"}</code> の SQLite ファイルにあります。
            エクスポートはすべてのデータ（ID・API トークンのハッシュと暗号化した控えを含む）を JSON にしたもので、空のデータディレクトリに取り込むと元どおりに復元できます。トークンの控えを読む鍵（データディレクトリの token.key）はエクスポートにもバックアップにも入らないので、後からトークンを表示したいときは、復元先のデータディレクトリにも置いてください（取り込みの前でも後でも可。なくてもトークンでの認証はそのまま使えます）。
          </p>
          <a href="/api/v1/export" className="inline-flex items-center gap-1 rounded-md border border-line px-2.5 py-1 text-[12.5px] hover:bg-hover">
            <Download size={13} />JSON をエクスポート
          </a>
          <AutoBackup bin={bin} dataArg={dataArg} />
          <div className="mt-3 space-y-2 text-[12.5px]">
            <div>
              <div className="mb-1 text-faint">エクスポートから復元（新しい空のデータディレクトリへ）</div>
              <CopyLine text={`${bin} import --data ~/.taskapp-restored --in taskapp-export.json`} />
            </div>
          </div>
        </section>}
      </div>
    </div>
  );
}

/** The signed-in user and their password for logging in from other devices (01 §4.12). */
function AccountSection({ me }: { me: User }) {
  return (
    <section data-testid="account">
      <h2 className="mb-2 text-[14px] font-semibold">アカウント</h2>
      <div className="mb-2 flex items-center gap-2 text-[13px]">
        <Avatar user={me} size={20} /><span className="font-medium">{me.display_name}</span>
        <span className="font-mono text-[12px] text-faint">@{me.username}</span>
        {me.kind === "human" && <span className="ml-2 text-[12px] text-muted">{me.has_password ? "パスワード設定済み" : "パスワード未設定"}</span>}
      </div>
      {me.kind === "human" && (
        <>
          <p className="mb-2 text-[12.5px] text-muted">ほかの端末（スマホ・別の PC）から画面に入るときは、ユーザー名とこのパスワードでログインします。8 文字以上。</p>
          <PasswordForm username="me" askCurrent={me.has_password && !me.is_owner} />
        </>
      )}
    </section>
  );
}

function PasswordForm({ username, askCurrent, onDone }: { username: string; askCurrent?: boolean; onDone?: () => void }) {
  const set = useSetPassword();
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const mismatch = again !== "" && password !== again;
  const input = "rounded-md border border-line bg-bg px-2 py-1 text-[13px] outline-none focus:border-accent";
  return (
    <form className="flex flex-wrap items-center gap-2" onSubmit={(e) => {
      e.preventDefault();
      set.mutate({ username, password, ...(askCurrent ? { current_password: current } : {}) }, {
        onSuccess: () => (setCurrent(""), setPassword(""), setAgain(""), onDone?.()),
      });
    }}>
      {username !== "me" && <span className="text-[12.5px] text-muted">@{username} の新しいパスワード</span>}
      {askCurrent && <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} placeholder="今のパスワード" autoComplete="current-password" className={`${input} w-40`} />}
      <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="新しいパスワード" autoComplete="new-password" className={`${input} w-44`} />
      <input type="password" value={again} onChange={(e) => setAgain(e.target.value)} placeholder="もう一度" autoComplete="new-password" className={`${input} w-36`} />
      <button disabled={password.length < 8 || password !== again || (askCurrent && !current) || set.isPending}
        className="flex items-center gap-1 rounded-md border border-line px-2.5 py-1 text-[12px] hover:bg-hover disabled:opacity-40"><KeyRound size={13} />パスワードを設定</button>
      {mismatch && <span className="text-[12px] text-danger">一致しません</span>}
      {set.error && <span className="text-[12px] text-danger">{(set.error as Error).message}</span>}
      {set.isSuccess && !set.isPending && <span className="text-[12px] text-[#3fae6a]">設定しました</span>}
    </form>
  );
}

/** How to reach this server from phones, other PCs and their agents (01 §4.12). */
function RemoteSection({ remoteUrl, bin, dataArg }: { remoteUrl?: string; bin: string; dataArg: string }) {
  return (
    <section data-testid="remote-access">
      <h2 className="mb-2 flex items-center gap-1.5 text-[14px] font-semibold"><MonitorSmartphone size={15} />ほかの端末から使う</h2>
      {remoteUrl ? (
        <div className="space-y-2 text-[12.5px]">
          <p className="text-muted">ほかの端末のブラウザでこの URL を開き、ユーザー名とパスワードでログインします。エージェントは API トークンで MCP（HTTP）につなぎます。</p>
          <CopyLine text={remoteUrl} />
        </div>
      ) : (
        <div className="space-y-2 text-[12.5px]">
          <p className="text-muted">今はこの PC からだけ接続できます。ほかの端末から使うには、外部から接続できるアドレスで起動し直します。</p>
          <CopyLine text={`${bin} serve${dataArg} --host 0.0.0.0`} />
        </div>
      )}
      <p className="mt-2 text-[12px] text-faint">
        通信は暗号化されません。自宅や社内の信頼できるネットワーク、または Tailscale などの VPN の中だけで使ってください。パスワードなしで入れるのはこの PC のブラウザだけです。
      </p>
    </section>
  );
}

const MCP_SCOPES = [
  { key: "user", label: "user", note: "自分だけ・すべてのプロジェクトで使えます（おすすめ）。~/.claude.json に保存されます。" },
  { key: "local", label: "local", note: "自分だけ・コマンドを実行したディレクトリのプロジェクトだけで使えます（--scope を省略したときと同じ）。" },
  { key: "project", label: "project", note: "リポジトリの .mcp.json に書かれ、git でメンバーと共有されます。トークンは書かず、各自が環境変数 TASKAPP_TOKEN に入れます。" },
] as const;
type McpScope = (typeof MCP_SCOPES)[number]["key"];

/** `claude mcp add` lines for each Claude Code scope; the project scope keeps the token out of .mcp.json. */
function McpSection({ token, origin, remoteUrl, bin, dataArg }: { token: string; origin: string; remoteUrl?: string; bin: string; dataArg: string }) {
  const [scope, setScope] = useState<McpScope>("user");
  const shared = scope === "project";
  const header = shared ? `'Authorization: Bearer \${TASKAPP_TOKEN}'` : `"Authorization: Bearer ${token}"`;
  const env = shared ? `'TASKAPP_TOKEN=\${TASKAPP_TOKEN}'` : `TASKAPP_TOKEN=${token}`;
  const add = `claude mcp add --scope ${scope}`;
  return (
    <section data-testid="mcp">
      <h2 className="mb-2 text-[14px] font-semibold">エージェントの接続（MCP）</h2>
      <p className="mb-2 text-[12.5px] text-muted">Claude Code などの MCP クライアントに登録すると、エージェントがチェックポイントの記録や QA の起票をできるようになります。</p>
      <div className="mb-1 flex items-center gap-2 text-[12.5px]">
        <span className="text-faint">登録先（スコープ）</span>
        <div className="flex overflow-hidden rounded-md border border-line">
          {MCP_SCOPES.map((s) => (
            <button key={s.key} onClick={() => setScope(s.key)} data-testid={`mcp-scope-${s.key}`}
              className={`px-2.5 py-0.5 font-mono text-[12px] ${scope === s.key ? "bg-accent text-white" : "text-muted hover:bg-hover"}`}>{s.label}</button>
          ))}
        </div>
      </div>
      <p className="mb-2 text-[12px] text-faint">{MCP_SCOPES.find((s) => s.key === scope)!.note}</p>
      <div className="space-y-2 text-[12.5px]">
        {shared && (
          <div>
            <div className="mb-1 text-faint">先に各自のシェルで（~/.zshrc などに書いておくと便利です）</div>
            <CopyLine text={`export TASKAPP_TOKEN=${token}`} />
          </div>
        )}
        <div>
          <div className="mb-1 text-faint">stdio（このマシン上のエージェント）</div>
          <CopyLine text={`${add} taskapp -e ${env} -- ${bin} mcp${dataArg}`} />
        </div>
        <div>
          <div className="mb-1 text-faint">HTTP</div>
          <CopyLine text={`${add} --transport http taskapp ${origin}/mcp --header ${header}`} />
        </div>
        {remoteUrl && remoteUrl !== origin && (
          <div>
            <div className="mb-1 text-faint">HTTP（ほかの端末のエージェント）</div>
            <CopyLine text={`${add} --transport http taskapp ${remoteUrl}/mcp --header ${header}`} />
          </div>
        )}
        <div>
          <div className="mb-1 text-faint">登録を消す・確かめる</div>
          <CopyLine text={`claude mcp remove taskapp -s ${scope}`} />
          <div className="mt-1"><CopyLine text="claude mcp list" /></div>
        </div>
      </div>
    </section>
  );
}

// The scale taskapp was checked at (01 NFR-03, 05 P5-3) and what the screens load at once.
const CHECKED_ISSUES = 10_000;
const CHECKED_MAP_NODES = 1_000;

const bytes = (n: number) =>
  n >= 1 << 30 ? `${(n / (1 << 30)).toFixed(1)} GB` : n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`;
const num = (n: number) => n.toLocaleString("ja-JP");

/** How close the data is to the scale taskapp was checked at, and how much disk is left. */
function CapacitySection() {
  const { data: c } = useCapacity();
  if (!c) return null;
  const { counts, storage, memory } = c;
  const diskUsed = storage.disk_total_bytes != null && storage.disk_free_bytes != null ? storage.disk_total_bytes - storage.disk_free_bytes : null;
  return (
    <section data-testid="capacity">
      <h2 className="mb-2 flex items-center gap-1.5 text-[14px] font-semibold"><Gauge size={15} />利用状況と上限</h2>
      <p className="mb-3 text-[12.5px] text-muted">
        動作を確かめた規模に対して、いまどこまで来ているかの目安です。70% で黄色、90% で赤になります。上限を超えても止まりはしませんが、遅くなったり、画面に出ないイシューが出たりします。
      </p>
      <div className="space-y-3 rounded-lg border border-line p-3 text-[12.5px]">
        <Meter testid="cap-issues" label="イシュー（全体）" used={counts.issues} limit={CHECKED_ISSUES} fmt={num}
          note={`未完了 ${num(counts.open_issues)} 件。性能を確かめた規模（一覧・フィルタが 0.3 秒以内）は 1 万件。`} />
        <Meter testid="cap-load" label="全体ボード・一覧に一度に出せる数" used={counts.issues} limit={ISSUE_LOAD_LIMIT} fmt={num}
          note={`絞り込まない全体ボードと一覧は、先頭 ${num(ISSUE_LOAD_LIMIT)} 件までを表示します。超えたら「完了を隠す」やフィルタで絞ります（未完了は ${num(counts.open_issues)} 件）。`} />
        <Meter testid="cap-map" label={`最大のプロジェクト${counts.largest_project ? `（${counts.largest_project}）` : ""}`} used={counts.largest_project_issues} limit={CHECKED_MAP_NODES} fmt={num}
          note="マインドマップの操作を確かめた規模は 1,000 ノード。300 を超えるとストーリーまで畳んで開きます。" />
        {diskUsed != null && storage.disk_total_bytes != null ? (
          <Meter testid="cap-disk" label="ディスク" used={diskUsed} limit={storage.disk_total_bytes} fmt={bytes}
            note={`taskapp のデータ ${bytes(storage.data_dir_bytes ?? 0)}（うちデータベース ${bytes(storage.db_bytes ?? 0)}、バックアップ ${bytes(storage.backups_bytes ?? 0)}）。1 万件でデータベースは約 9 MB です。`} />
        ) : (
          <div className="text-faint">ディスクの空きはこの環境では取得できません。データ {bytes(storage.data_dir_bytes ?? 0)}</div>
        )}
        <div className="flex gap-6 border-t border-line pt-2 text-muted">
          <span>プロジェクト {num(counts.projects)}</span>
          <span>メンバー・エージェント {num(counts.users)}</span>
          <span>履歴 {num(counts.activities)} 件</span>
          <span>メモリ {bytes(memory.sys_bytes)}</span>
        </div>
      </div>
    </section>
  );
}

function Meter({ label, used, limit, fmt, note, testid }: { label: string; used: number; limit: number; fmt: (n: number) => string; note: string; testid: string }) {
  const pct = limit > 0 ? (used / limit) * 100 : 0;
  const level = pct >= 90 ? "danger" : pct >= 70 ? "warn" : "ok";
  const color = { ok: "#3fae6a", warn: "#d4a017", danger: "var(--danger, #d64545)" }[level];
  return (
    <div data-testid={testid} data-level={level}>
      <div className="mb-1 flex items-baseline gap-2">
        <span className="font-medium">{label}</span>
        <span className="text-muted">{fmt(used)} / {fmt(limit)}</span>
        <span className="ml-auto font-mono text-[12px]" style={{ color }}>{pct < 0.1 && used > 0 ? "<0.1" : pct.toFixed(pct < 10 ? 1 : 0)}%</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-hover">
        <div className="h-full rounded-full" style={{ width: `${Math.min(100, pct)}%`, background: color }} />
      </div>
      <div className="mt-1 text-[11.5px] text-faint">{note}</div>
    </div>
  );
}

/** Automatic backup schedule, the backups and "back up now" (NFR-02). */
function AutoBackup({ bin, dataArg }: { bin: string; dataArg: string }) {
  const { data: st } = useBackups();
  const create = useCreateBackup();
  if (!st) return null;
  const every = (s: number) => (s % 86400 === 0 ? `${s / 86400} 日` : s % 3600 === 0 ? `${s / 3600} 時間` : s % 60 === 0 ? `${s / 60} 分` : `${s} 秒`);
  const fmt = (iso: string) => new Date(iso).toLocaleString("ja-JP", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" });
  const size = (n: number) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`);
  return (
    <div className="mt-4 rounded-lg border border-line p-3 text-[12.5px]" data-testid="auto-backup">
      <div className="mb-2 flex items-center gap-2">
        <span className="font-medium">自動バックアップ</span>
        <span className={`rounded px-1.5 text-[11px] font-medium ${st.enabled ? "bg-[#3fae6a1f] text-[#3fae6a]" : "bg-hover text-faint"}`}>{st.enabled ? "有効" : "無効"}</span>
        <span className="flex-1" />
        <button onClick={() => create.mutate()} disabled={create.isPending}
          className="flex items-center gap-1 rounded-md border border-line px-2.5 py-1 hover:bg-hover disabled:opacity-40">
          <DatabaseBackup size={13} />今すぐバックアップ
        </button>
      </div>
      <dl className="grid grid-cols-[120px_1fr] gap-y-1 text-muted">
        <dt className="text-faint">間隔・保持数</dt>
        <dd>{st.enabled ? `${every(st.interval_seconds)}ごと（変更があったときだけ）` : "自動では取りません（--backup-interval 0）"}、新しい {st.keep || "すべて"}{st.keep ? " 件" : ""}を保持</dd>
        <dt className="text-faint">保存先</dt>
        <dd className="break-all font-mono text-[12px]">{st.dir}</dd>
        <dt className="text-faint">最新</dt>
        <dd>{st.last_backup_at ? `${fmt(st.last_backup_at)}（${relativeTime(st.last_backup_at)}）` : "まだありません"}</dd>
        {st.enabled && st.next_backup_at && (<><dt className="text-faint">次回の確認</dt><dd>{fmt(st.next_backup_at)}</dd></>)}
      </dl>
      {(st.last_error || create.error) && <div className="mt-2 text-danger">バックアップに失敗しました: {st.last_error || (create.error as Error).message}</div>}
      {create.data && <div className="mt-2 text-[#3fae6a]">{create.data.name} を作成しました</div>}
      {st.files.length > 0 && (
        <ul className="mt-2 divide-y divide-line rounded-md border border-line">
          {st.files.map((f) => (
            <li key={f.name} className="flex items-center gap-3 px-2.5 py-1">
              <span className="flex-1 font-mono text-[12px]">{f.name}</span>
              <span className="text-faint">{size(f.size)}</span>
            </li>
          ))}
        </ul>
      )}
      <div className="mt-3 mb-1 text-faint">バックアップから戻す（サーバーを止めてから。今のデータは taskapp.db.before-restore-… として残ります）</div>
      <CopyLine text={`${bin} restore${dataArg} --from ${st.dir}/${st.files[0]?.name ?? "taskapp-YYYYMMDD-HHMMSS.db"}`} />
    </div>
  );
}

function CopyLine({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-center gap-2 rounded-md border border-line bg-bg px-2.5 py-1.5">
      <code className="flex-1 overflow-x-auto whitespace-nowrap font-mono text-[12px]">{text}</code>
      <button onClick={() => copyText(text).then(() => (setCopied(true), setTimeout(() => setCopied(false), 1500)))}
        className="flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[11.5px] text-muted hover:bg-hover hover:text-fg">
        {copied ? <Check size={12} /> : <Copy size={12} />}{copied ? "コピー済み" : "コピー"}
      </button>
    </div>
  );
}

/** Clipboard write that also works over plain http from other devices (no navigator.clipboard there). */
async function copyText(text: string): Promise<void> {
  if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(text);
  const area = document.createElement("textarea");
  area.value = text;
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.appendChild(area);
  area.select();
  document.execCommand("copy");
  area.remove();
}
