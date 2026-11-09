import { useMutation, useQueries, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { useEffect } from "react";
import { api, ensureSession, unwrap, type AnswerInput, type CreateIssueInput, type Filter, type ProjectTier, type RequirementsMode, type ReviewInput, type SavedViewInput, type SavedViewPatch, type UpdateIssueInput } from "./client";

export const qk = {
  me: ["me"] as const,
  users: ["users"] as const,
  projects: ["projects"] as const,
  project: (key: string) => ["projects", key] as const,
  states: (key: string) => ["projects", key, "states"] as const,
  labels: (key: string) => ["projects", key, "labels"] as const,
  issues: ["issues"] as const,
  issueList: (params: object) => ["issues", "list", params] as const,
  issue: (key: string) => ["issues", "detail", key] as const,
  comments: (key: string) => ["issues", "detail", key, "comments"] as const,
  activity: (key: string) => ["issues", "detail", key, "activity"] as const,
  checkpoints: (key: string) => ["issues", "detail", key, "checkpoints"] as const,
  qa: (key: string) => ["issues", "qa", key] as const,
  qaList: (params: object) => ["issues", "qa-list", params] as const,
  feed: (params: object) => ["issues", "feed", params] as const,
  tokens: ["tokens"] as const,
  acceptance: (key: string) => ["issues", "acceptance", key] as const,
  acceptanceQueue: (project?: string) => ["issues", "acceptance-queue", project ?? ""] as const,
  views: ["views"] as const,
  backups: ["backups"] as const,
  storymaps: ["storymap"] as const,
  storymap: (key: string) => ["storymap", key] as const,
  story: (key: string, story: string) => ["storymap", key, "story", story] as const,
  // Under "storymap" so a storymap event refreshes them too.
  drafts: (key: string) => ["storymap", key, "drafts"] as const,
  importPlan: (key: string) => ["storymap", key, "import-plan"] as const,
};

export const useMe = () => useQuery({ queryKey: qk.me, queryFn: () => unwrap(api.GET("/me")), staleTime: Infinity });

export const useUsers = () =>
  useQuery({ queryKey: qk.users, queryFn: () => unwrap(api.GET("/users")).then((r) => r.users), staleTime: 60_000 });

export const useProjects = () =>
  useQuery({ queryKey: qk.projects, queryFn: () => unwrap(api.GET("/projects")).then((r) => r.projects) });

export const useProject = (key: string) =>
  useQuery({ queryKey: qk.project(key), enabled: !!key, queryFn: () => unwrap(api.GET("/projects/{key}", { params: { path: { key } } })) });

export const useStates = (key: string | undefined) =>
  useQuery({
    queryKey: qk.states(key ?? ""),
    enabled: !!key,
    staleTime: 60_000,
    queryFn: () => unwrap(api.GET("/projects/{key}/states", { params: { path: { key: key! } } })).then((r) => r.states),
  });

/** States of several projects at once (the cross-project board), keyed by project key. */
export function useStatesOf(keys: string[]) {
  return useQueries({
    queries: keys.map((key) => ({
      queryKey: qk.states(key),
      staleTime: 60_000,
      queryFn: () => unwrap(api.GET("/projects/{key}/states", { params: { path: { key } } })).then((r) => r.states),
    })),
    combine: (rs) => new Map(keys.map((k, i) => [k, rs[i].data ?? []] as const)),
  });
}

export const useLabels = (key: string | undefined) =>
  useQuery({
    queryKey: qk.labels(key ?? ""),
    enabled: !!key,
    queryFn: () => unwrap(api.GET("/projects/{key}/labels", { params: { path: { key: key! } } })).then((r) => r.labels),
  });

export interface IssueQuery {
  project?: string;
  filter?: Filter;
  sort?: "updated" | "created" | "priority" | "manual" | "key" | "due_date";
  includeAncestors?: boolean;
  limit?: number;
  enabled?: boolean;
}

/** Issues a list, board or tree loads at once; beyond that only the first ones are shown. */
export const ISSUE_LOAD_LIMIT = 2000;

export const useIssues = ({ enabled = true, ...q }: IssueQuery) =>
  useQuery({
    queryKey: qk.issueList(q),
    enabled,
    placeholderData: keepPreviousData,
    queryFn: () =>
      unwrap(
        api.GET("/issues", {
          params: {
            query: {
              project: q.project,
              filter: q.filter ? JSON.stringify(q.filter) : undefined,
              sort: q.sort,
              include_ancestors: q.includeAncestors,
              limit: q.limit ?? ISSUE_LOAD_LIMIT,
            },
          },
        }),
      ),
  });

export const useIssue = (key: string | undefined) =>
  useQuery({
    queryKey: qk.issue(key ?? ""),
    enabled: !!key,
    queryFn: () => unwrap(api.GET("/issues/{key}", { params: { path: { key: key! } } })),
  });

export const useComments = (key: string) =>
  useQuery({
    queryKey: qk.comments(key),
    queryFn: () => unwrap(api.GET("/issues/{key}/comments", { params: { path: { key } } })).then((r) => r.comments),
  });

export const useActivity = (key: string) =>
  useQuery({
    queryKey: qk.activity(key),
    queryFn: () => unwrap(api.GET("/issues/{key}/activity", { params: { path: { key } } })).then((r) => r.activity),
  });

export const useCheckpoints = (key: string) =>
  useQuery({
    queryKey: qk.checkpoints(key),
    queryFn: () => unwrap(api.GET("/issues/{key}/checkpoints", { params: { path: { key } } })).then((r) => r.checkpoints),
  });

export const useQA = (key: string | undefined) =>
  useQuery({
    queryKey: qk.qa(key ?? ""),
    enabled: !!key,
    queryFn: () => unwrap(api.GET("/qa/{key}", { params: { path: { key: key! } } })),
  });

export interface QAListParams {
  status?: "open" | "answered" | "closed";
  assignee?: string;
  asked_by?: string;
  project?: string;
}

export const useQAList = (params: QAListParams) =>
  useQuery({
    queryKey: qk.qaList(params),
    placeholderData: keepPreviousData,
    queryFn: () => unwrap(api.GET("/qa", { params: { query: { ...params, limit: 500 } } })).then((r) => r.questions),
  });

export function useAnswerQA(key: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: AnswerInput) => unwrap(api.POST("/qa/{key}/answers", { params: { path: { key } }, body })),
    onSuccess: (qa) => {
      qc.setQueryData(qk.qa(key), qa);
      qc.invalidateQueries({ queryKey: qk.issues });
    },
  });
}

export function useCloseQA(key: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST("/qa/{key}/close", { params: { path: { key } } })),
    onSuccess: (qa) => {
      qc.setQueryData(qk.qa(key), qa);
      qc.invalidateQueries({ queryKey: qk.issues });
    },
  });
}

export interface FeedParams {
  agent?: string;
  project?: string;
  low_confidence?: boolean;
}

export const useAgentFeed = (params: FeedParams) =>
  useQuery({
    queryKey: qk.feed(params),
    placeholderData: keepPreviousData,
    queryFn: () => unwrap(api.GET("/agents/activity", { params: { query: { ...params, limit: 200 } } })).then((r) => r.entries),
  });

export const useTokens = () =>
  useQuery({ queryKey: qk.tokens, queryFn: () => unwrap(api.GET("/tokens")).then((r) => r.tokens) });

export function useIssueToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { user?: string; name: string }) => unwrap(api.POST("/tokens", { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.tokens }),
  });
}

/** Automatic backups; refreshed every minute while shown. */
/** Counts, storage and memory for the capacity section (owner only). */
export const useCapacity = () =>
  useQuery({ queryKey: ["capacity"], queryFn: () => unwrap(api.GET("/capacity")), refetchInterval: 60_000 });

export const useBackups = () =>
  useQuery({ queryKey: qk.backups, queryFn: () => unwrap(api.GET("/backups")), refetchInterval: 60_000 });

export function useCreateBackup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST("/backups")),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.backups }),
  });
}

/** The raw secret of an active token, kept encrypted on the server (409 not_available for old ones). */
export function useRevealToken() {
  return useMutation({
    mutationFn: (id: string) => unwrap(api.POST("/tokens/{id}/reveal", { params: { path: { id } } })),
  });
}

export function useRevokeToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { response } = await api.DELETE("/tokens/{id}", { params: { path: { id } } });
      if (!response.ok) throw new Error(`revoke failed: ${response.status}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.tokens }),
  });
}

export function useAddUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { kind: "human" | "agent"; username: string; display_name?: string; agent_model?: string }) =>
      unwrap(api.POST("/users", { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.users }),
  });
}

export function useSetPassword() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ username, ...body }: { username: string; password: string; current_password?: string }) =>
      unwrap(api.PUT("/users/{username}/password", { params: { path: { username } }, body })),
    onSuccess: () => (qc.invalidateQueries({ queryKey: qk.users }), qc.invalidateQueries({ queryKey: qk.me })),
  });
}

export function useCreateIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateIssueInput) => unwrap(api.POST("/issues", { body })),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.issues });
      qc.invalidateQueries({ queryKey: qk.projects });
    },
  });
}

export function useUpdateIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, ...body }: UpdateIssueInput & { key: string }) =>
      unwrap(api.PATCH("/issues/{key}", { params: { path: { key } }, body })),
    onSuccess: (issue) => {
      qc.setQueryData(qk.issue(issue.key), issue);
      qc.invalidateQueries({ queryKey: qk.issues });
      qc.invalidateQueries({ queryKey: qk.projects });
    },
  });
}

export function useDeleteIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (key: string) => unwrap(api.DELETE("/issues/{key}", { params: { path: { key } } })),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.issues });
      qc.invalidateQueries({ queryKey: qk.projects });
    },
  });
}

export function useAddComment(key: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => unwrap(api.POST("/issues/{key}/comments", { params: { path: { key } }, body: { body } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.issue(key) }),
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { key: string; name: string; description?: string; tier?: ProjectTier; requirements_mode?: RequirementsMode }) => unwrap(api.POST("/projects", { body: { allow_skip_levels: true, ...body } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.projects }),
  });
}

/** Subscribes to server-sent events and refetches affected data. */
export function useLiveUpdates() {
  const qc = useQueryClient();
  useEffect(() => {
    let es: EventSource | null = null;
    let retry: ReturnType<typeof setTimeout>;
    // Bursts of events (an agent recording several changes, an import) are
    // coalesced into one refetch.
    let pending = { issues: false, projects: false, storymap: false };
    let flush: ReturnType<typeof setTimeout> | undefined;
    const schedule = (issues: boolean, storymap = false) => {
      pending = { issues: pending.issues || issues, projects: pending.projects || !storymap, storymap: pending.storymap || storymap };
      flush ??= setTimeout(() => {
        if (pending.issues) qc.invalidateQueries({ queryKey: qk.issues });
        // Story cards are coloured by the state of their linked issues.
        if (pending.issues || pending.storymap) qc.invalidateQueries({ queryKey: qk.storymaps });
        if (pending.projects) qc.invalidateQueries({ queryKey: qk.projects });
        pending = { issues: false, projects: false, storymap: false };
        flush = undefined;
      }, 300);
    };
    const connect = () => {
      es = new EventSource("/api/v1/events");
      es.onmessage = (msg) => {
        const e = JSON.parse(msg.data) as { type: string; project_key?: string; issue_key?: string };
        if (e.type.startsWith("issue.")) schedule(true);
        else if (e.type.startsWith("project.")) schedule(false);
        else if (e.type.startsWith("storymap.")) schedule(false, true);
      };
      es.onerror = () => {
        es?.close();
        // Sessions live in server memory, so a server restart invalidates the
        // cookie: re-establish it (or go to the login page) before reconnecting.
        retry = setTimeout(async () => {
          const me = await fetch("/api/v1/me").catch(() => null);
          if (me?.status === 401 && !(await ensureSession())) return;
          connect();
          qc.invalidateQueries();
        }, 2000);
      };
    };
    connect();
    return () => {
      clearTimeout(retry);
      clearTimeout(flush);
      es?.close();
    };
  }, [qc]);
}

// ---- acceptance ----

export const useAcceptance = (key: string | undefined) =>
  useQuery({
    queryKey: qk.acceptance(key ?? ""),
    enabled: !!key,
    queryFn: () => unwrap(api.GET("/issues/{key}/acceptance", { params: { path: { key: key! } } })),
  });

export const useAcceptanceQueue = (project?: string) =>
  useQuery({
    queryKey: qk.acceptanceQueue(project),
    placeholderData: keepPreviousData,
    queryFn: () => unwrap(api.GET("/acceptance", { params: { query: project ? { project } : {} } })).then((r) => r.items),
  });

export function useSetCriteria(key: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (criteria: { id?: string | null; description: string }[]) =>
      unwrap(api.PUT("/issues/{key}/acceptance/criteria", { params: { path: { key } }, body: { criteria } })),
    onSuccess: (a) => {
      qc.setQueryData(qk.acceptance(key), a);
      qc.invalidateQueries({ queryKey: qk.issues });
    },
  });
}

export function useReviewAcceptance(key: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ReviewInput) => unwrap(api.POST("/issues/{key}/acceptance/reviews", { params: { path: { key } }, body })),
    onSuccess: (r) => {
      qc.setQueryData(qk.acceptance(key), r.acceptance);
      qc.invalidateQueries({ queryKey: qk.issues });
    },
  });
}

// ---- saved views ----

export const useViews = () =>
  useQuery({ queryKey: qk.views, queryFn: () => unwrap(api.GET("/views")).then((r) => r.views) });

export function useCreateView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: SavedViewInput) => unwrap(api.POST("/views", { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.views }),
  });
}

export function useUpdateView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: SavedViewPatch & { id: string }) => unwrap(api.PATCH("/views/{id}", { params: { path: { id } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.views }),
  });
}

export function useDeleteView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { response } = await api.DELETE("/views/{id}", { params: { path: { id } } });
      if (!response.ok) throw new Error(`delete failed: ${response.status}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.views }),
  });
}

// ---- project settings ----

function useProjectMutation<T>(key: string, fn: (arg: T) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.project(key) });
      qc.invalidateQueries({ queryKey: qk.projects });
      qc.invalidateQueries({ queryKey: qk.issues });
    },
  });
}

async function noContent(p: Promise<{ response: Response; error?: unknown }>) {
  const { response, error } = await p;
  if (!response.ok) {
    const msg = (error as { error?: { message?: string } } | undefined)?.error?.message ?? `failed: ${response.status}`;
    throw new Error(msg);
  }
}

export const useUpdateProject = (key: string) =>
  useProjectMutation(key, (body: { name?: string; description?: string; allow_skip_levels?: boolean; tier?: ProjectTier; requirements_mode?: RequirementsMode }) =>
    unwrap(api.PATCH("/projects/{key}", { params: { path: { key } }, body })));

export const useCreateState = (key: string) =>
  useProjectMutation(key, (body: { name: string; category: "backlog" | "unstarted" | "started" | "completed" | "canceled"; color?: string }) =>
    unwrap(api.POST("/projects/{key}/states", { params: { path: { key } }, body })));

export const useUpdateState = (key: string) =>
  useProjectMutation(key, ({ id, ...body }: { id: number; name?: string; color?: string; position?: number; is_default?: true; is_acceptance?: true }) =>
    unwrap(api.PATCH("/projects/{key}/states/{id}", { params: { path: { key, id } }, body })));

export const useDeleteState = (key: string) =>
  useProjectMutation(key, (id: number) => noContent(api.DELETE("/projects/{key}/states/{id}", { params: { path: { key, id } } })));

export const useCreateLabel = (key: string) =>
  useProjectMutation(key, (body: { name: string; color?: string }) => unwrap(api.POST("/projects/{key}/labels", { params: { path: { key } }, body })));

export const useUpdateLabel = (key: string) =>
  useProjectMutation(key, ({ id, ...body }: { id: string; name?: string; color?: string }) =>
    unwrap(api.PATCH("/projects/{key}/labels/{id}", { params: { path: { key, id } }, body })));

export const useDeleteLabel = (key: string) =>
  useProjectMutation(key, (id: string) => noContent(api.DELETE("/projects/{key}/labels/{id}", { params: { path: { key, id } } })));

// ---- requirements layer: story map (docs/08-devcycle.md) ----

export const useStoryMap = (key: string) =>
  useQuery({ queryKey: qk.storymap(key), enabled: !!key, queryFn: () => unwrap(api.GET("/projects/{key}/storymap", { params: { path: { key } } })) });

export const useStory = (key: string, story: string | undefined) =>
  useQuery({
    queryKey: qk.story(key, story ?? ""),
    enabled: !!key && !!story,
    queryFn: () => unwrap(api.GET("/projects/{key}/stories/{rkey}", { params: { path: { key, rkey: story! } } })),
  });

// ---- drafts of a story map (docs/08-devcycle.md §5.9) ----

export const useDrafts = (key: string, enabled = true) =>
  useQuery({ queryKey: qk.drafts(key), enabled: !!key && enabled, queryFn: () => unwrap(api.GET("/projects/{key}/drafts", { params: { path: { key } } })).then((r) => r.drafts) });

export const useImportPlan = (draft: string, enabled: boolean) =>
  useQuery({ queryKey: qk.importPlan(draft), enabled: !!draft && enabled, queryFn: () => unwrap(api.GET("/projects/{key}/import-plan", { params: { path: { key: draft } } })) });

export function useDraftOps() {
  const qc = useQueryClient();
  const refresh = <T,>(p: Promise<T>) =>
    p.then((v) => {
      qc.invalidateQueries({ queryKey: qk.storymaps });
      qc.invalidateQueries({ queryKey: qk.projects });
      return v;
    });
  return {
    create: (target: string, body: { from: "copy" | "empty"; name?: string }) =>
      refresh(unwrap(api.POST("/projects/{key}/drafts", { params: { path: { key: target } }, body }))),
    apply: (draft: string, force: boolean) =>
      refresh(unwrap(api.POST("/projects/{key}/import", { params: { path: { key: draft } }, body: { force } }))),
    discard: (draft: string) =>
      refresh(unwrap(api.PATCH("/projects/{key}", { params: { path: { key: draft } }, body: { archived: true } }))),
  };
}

type Body<P extends keyof import("./schema").paths, M extends "post" | "patch"> =
  import("./schema").paths[P][M] extends { requestBody?: { content: { "application/json": infer B } } } ? B : never;

/** Every write to the story map. Each call refreshes the map, the open story and issue details. */
export function useStoryMapOps(key: string) {
  const qc = useQueryClient();
  const done = <T,>(p: Promise<T>) =>
    p.then((v) => {
      qc.invalidateQueries({ queryKey: qk.storymap(key) });
      qc.invalidateQueries({ queryKey: qk.issues });
      return v;
    });
  const del = async (path: string) => {
    const res = await fetch(`/api/v1/projects/${encodeURIComponent(key)}/${path}`, { method: "DELETE" });
    if (!res.ok) throw new Error((await res.json().catch(() => null))?.error?.message ?? `failed: ${res.status}`);
  };
  const k = (rkey: string) => ({ params: { path: { key, rkey } } });
  const p = { params: { path: { key } } };
  return {
    createRelease: (body: Body<"/projects/{key}/releases", "post">) => done(unwrap(api.POST("/projects/{key}/releases", { ...p, body }))),
    updateRelease: (rkey: string, body: Body<"/projects/{key}/releases/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/releases/{rkey}", { ...k(rkey), body }))),
    deleteRelease: (rkey: string) => done(del(`releases/${rkey}`)),
    createPersona: (body: Body<"/projects/{key}/personas", "post">) => done(unwrap(api.POST("/projects/{key}/personas", { ...p, body }))),
    updatePersona: (rkey: string, body: Body<"/projects/{key}/personas/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/personas/{rkey}", { ...k(rkey), body }))),
    archivePersona: (rkey: string) => done(del(`personas/${rkey}`)),
    createJob: (body: Body<"/projects/{key}/jobs", "post">) => done(unwrap(api.POST("/projects/{key}/jobs", { ...p, body }))),
    updateJob: (rkey: string, body: Body<"/projects/{key}/jobs/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/jobs/{rkey}", { ...k(rkey), body }))),
    archiveJob: (rkey: string) => done(del(`jobs/${rkey}`)),
    createActivity: (body: Body<"/projects/{key}/activities", "post">) => done(unwrap(api.POST("/projects/{key}/activities", { ...p, body }))),
    updateActivity: (rkey: string, body: Body<"/projects/{key}/activities/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/activities/{rkey}", { ...k(rkey), body }))),
    archiveActivity: (rkey: string) => done(del(`activities/${rkey}`)),
    createStep: (activity: string, body: Body<"/projects/{key}/activities/{rkey}/steps", "post">) =>
      done(unwrap(api.POST("/projects/{key}/activities/{rkey}/steps", { ...k(activity), body }))),
    updateStep: (rkey: string, body: Body<"/projects/{key}/steps/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/steps/{rkey}", { ...k(rkey), body }))),
    archiveStep: (rkey: string) => done(del(`steps/${rkey}`)),
    createStory: (body: Body<"/projects/{key}/stories", "post">) => done(unwrap(api.POST("/projects/{key}/stories", { ...p, body }))),
    updateStory: (rkey: string, body: Body<"/projects/{key}/stories/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/stories/{rkey}", { ...k(rkey), body }))),
    archiveStory: (rkey: string) => done(del(`stories/${rkey}`)),
    createCriterion: (story: string, body: Body<"/projects/{key}/stories/{rkey}/criteria", "post">) =>
      done(unwrap(api.POST("/projects/{key}/stories/{rkey}/criteria", { ...k(story), body }))),
    updateCriterion: (rkey: string, body: Body<"/projects/{key}/criteria/{rkey}", "patch">) => done(unwrap(api.PATCH("/projects/{key}/criteria/{rkey}", { ...k(rkey), body }))),
    archiveCriterion: (rkey: string) => done(del(`criteria/${rkey}`)),
    linkIssue: (story: string, issueKey: string) =>
      done(unwrap(api.PUT("/projects/{key}/stories/{rkey}/issues/{issueKey}", { params: { path: { key, rkey: story, issueKey } } }))),
    unlinkIssue: (story: string, issueKey: string) =>
      done(unwrap(api.DELETE("/projects/{key}/stories/{rkey}/issues/{issueKey}", { params: { path: { key, rkey: story, issueKey } } }))),
  };
}
