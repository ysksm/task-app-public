import { useQuery } from "@tanstack/react-query";
import { LogIn, Monitor } from "lucide-react";
import { useState } from "react";
import { markLoggedOut } from "../api/client";

type Options = { local_login: boolean; password_login: boolean };

const REMEMBER_KEY = "taskapp.remember-login";

/** Where to go after logging in: only same-site paths. */
function nextPath(): string {
  const next = new URLSearchParams(window.location.search).get("next") ?? "/my";
  return next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/login") ? next : "/my";
}

/** Login page (01 §4.12). Plain fetch: the API client would retry 401s. */
export function LoginPage() {
  const { data: options } = useQuery({
    queryKey: ["auth-options"],
    queryFn: async (): Promise<Options> => (await fetch("/api/v1/auth/options")).json(),
  });
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // The last choice of "keep me logged in" on this browser.
  const [remember, setRemember] = useState(() => {
    try {
      return localStorage.getItem(REMEMBER_KEY) === "1";
    } catch {
      return false;
    }
  });

  const enter = () => {
    markLoggedOut(false);
    // A full load so that every query starts again with the new session.
    window.location.replace(nextPath());
  };
  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: username.trim(), password, remember }),
      });
      if (res.ok) {
        try {
          localStorage.setItem(REMEMBER_KEY, remember ? "1" : "0");
        } catch {
          /* private mode: just not remembered */
        }
        return enter();
      }
      const body = await res.json().catch(() => null);
      setError(res.status === 429 ? "失敗が続いたため、1 分ほど待ってからやり直してください" :
        res.status === 401 ? "ユーザー名かパスワードが違います" : body?.error?.message ?? `ログインできませんでした（${res.status}）`);
    } catch {
      setError("サーバーに接続できません");
    } finally {
      setBusy(false);
    }
  };
  const local = async () => {
    const res = await fetch("/api/v1/auth/local", { method: "POST" });
    if (res.ok) enter();
    else setError("このPCからの自動ログインはできません");
  };

  const input = "w-full rounded-md border border-line bg-bg px-2.5 py-1.5 text-[13.5px] outline-none focus:border-accent";
  return (
    <div className="flex h-full items-center justify-center bg-sidebar px-4">
      <div className="w-full max-w-[360px] rounded-xl border border-line bg-bg p-6 shadow-sm">
        <div className="mb-5 flex items-center gap-2">
          <span className="flex h-6 w-6 items-center justify-center rounded-md bg-accent text-[12px] font-bold text-white">t</span>
          <h1 className="text-[16px] font-semibold">taskapp にログイン</h1>
        </div>
        <form className="space-y-3" onSubmit={(e) => (e.preventDefault(), submit())}>
          <label className="block text-[12.5px] text-muted">
            ユーザー名
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoCapitalize="none" autoFocus className={`${input} mt-1`} />
          </label>
          <label className="block text-[12.5px] text-muted">
            パスワード
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" className={`${input} mt-1`} />
          </label>
          <label className="flex cursor-pointer items-center gap-2 text-[12.5px] text-muted">
            <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} className="accent-[var(--accent)]" />
            ログインしたままにする（30 日）
          </label>
          {error && <div role="alert" className="text-[12.5px] text-danger">{error}</div>}
          <button disabled={busy || !username.trim() || !password}
            className="flex w-full items-center justify-center gap-1.5 rounded-md bg-accent px-3 py-1.5 text-[13px] font-medium text-white disabled:opacity-40">
            <LogIn size={14} />ログイン
          </button>
        </form>
        {options?.local_login && (
          <button onClick={local}
            className="mt-3 flex w-full items-center justify-center gap-1.5 rounded-md border border-line px-3 py-1.5 text-[13px] hover:bg-hover">
            <Monitor size={14} />このPCのオーナーとして入る
          </button>
        )}
        {options && !options.password_login && !options.local_login && (
          <p className="mt-4 text-[12px] leading-relaxed text-faint">
            まだ誰もパスワードを設定していません。サーバーの PC で 設定 › アカウント からパスワードを設定するか、
            <code className="rounded bg-hover px-1">taskapp passwd --user ユーザー名</code> を実行してください。
          </p>
        )}
      </div>
    </div>
  );
}
