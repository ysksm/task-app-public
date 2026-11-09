import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type Issue = Schemas["Issue"];
export type IssueType = Schemas["IssueType"];
export type User = Schemas["User"];
export type UserRef = Schemas["UserRef"];
export type Project = Schemas["Project"];
export type ProjectTier = Schemas["ProjectTier"];
export type RequirementsMode = Schemas["RequirementsMode"];
export type State = Schemas["State"];
export type Label = Schemas["Label"];
export type Filter = Schemas["Filter"];
export type Comment = Schemas["Comment"];
export type Activity = Schemas["Activity"];
export type CreateIssueInput = Schemas["CreateIssueInput"];
export type UpdateIssueInput = Schemas["UpdateIssueInput"];
export type QA = Schemas["QA"];
export type QAItem = Schemas["QAItem"];
export type AnswerInput = Schemas["AnswerInput"];
export type Checkpoint = Schemas["Checkpoint"];
export type FeedEntry = Schemas["FeedEntry"];
export type Token = Schemas["Token"];
export type Acceptance = Schemas["Acceptance"];
export type AcceptanceReview = Schemas["AcceptanceReview"];
export type AcceptanceQueueItem = Schemas["AcceptanceQueueItem"];
export type ReviewInput = Schemas["ReviewInput"];
export type SavedView = Schemas["SavedView"];
export type SavedViewInput = Schemas["SavedViewInput"];
export type SavedViewPatch = Schemas["SavedViewPatch"];
export type StoryMap = Schemas["StoryMap"];
export type ReleaseSlice = Schemas["Release"];
export type ImportPlan = Schemas["ImportPlan"];
export type ImportItem = Schemas["ImportItem"];
export type Persona = Schemas["Persona"];
export type Job = Schemas["Job"];
export type StoryActivity = Schemas["StoryActivity"];
export type Step = Schemas["Step"];
export type Story = Schemas["Story"];
export type StoryStatus = Schemas["StoryStatus"];
export type Criterion = Schemas["Criterion"];
export type StoryRef = Schemas["StoryRef"];

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

// Sessions (01 §4.12). On 401 a browser on the server's machine takes an
// owner session without a password (POST /auth/local); anywhere else, or
// after an explicit logout, the login page is shown instead.
const LOGGED_OUT = "taskapp.loggedOut";

export function markLoggedOut(out: boolean) {
  try {
    if (out) localStorage.setItem(LOGGED_OUT, "1");
    else localStorage.removeItem(LOGGED_OUT);
  } catch {
    /* storage unavailable: automatic login stays on */
  }
}

function loggedOut(): boolean {
  try {
    return localStorage.getItem(LOGGED_OUT) === "1";
  } catch {
    return false;
  }
}

let loginPromise: Promise<boolean> | null = null;
function localLogin(): Promise<boolean> {
  if (loggedOut()) return Promise.resolve(false);
  loginPromise ??= fetch("/api/v1/auth/local", { method: "POST", credentials: "same-origin" })
    .then((r) => r.ok)
    .catch(() => false)
    .finally(() => setTimeout(() => (loginPromise = null), 1000));
  return loginPromise;
}

/** Sends the browser to the login page, coming back to the current page afterwards. */
export function goToLogin() {
  if (window.location.pathname === "/login") return;
  const next = window.location.pathname + window.location.search;
  window.location.assign("/login?next=" + encodeURIComponent(next));
}

/** Makes sure there is a session: automatic login where allowed, else the login page. */
export async function ensureSession(): Promise<boolean> {
  if (await localLogin()) return true;
  goToLogin();
  return false;
}

// Requests are cloned before sending so that they can be replayed (bodies are
// single-use streams).
const pending = new WeakMap<Request, Request>();
const retryAfterLogin: Middleware = {
  onRequest({ request }) {
    pending.set(request, request.clone());
  },
  async onResponse({ request, response }) {
    const copy = pending.get(request);
    pending.delete(request);
    if (response.status !== 401 || !copy) return response;
    if (!(await ensureSession())) return response;
    return fetch(copy);
  },
};

export const api = createClient<paths>({ baseUrl: "/api/v1", credentials: "same-origin" });
api.use(retryAfterLogin);

/** Unwraps an openapi-fetch result, throwing ApiError on failure. */
export async function unwrap<T>(p: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await p;
  if (error !== undefined || data === undefined) {
    const e = (error as { error?: { code: string; message: string } } | undefined)?.error;
    throw new ApiError(response.status, e?.code ?? "unknown", e?.message ?? response.statusText);
  }
  return data;
}
