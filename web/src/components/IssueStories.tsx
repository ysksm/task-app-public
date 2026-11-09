import { useNavigate } from "@tanstack/react-router";
import { Map as MapIcon } from "lucide-react";
import type { Criterion, StoryRef } from "../api/client";

/** S09: the stories (requirements layer) an issue helps satisfy — why it exists. */
export function IssueStories({ projectKey, stories }: { projectKey: string; stories: StoryRef[] }) {
  const navigate = useNavigate();
  if (stories.length === 0) return null;
  return (
    <section className="mt-6" data-testid="issue-stories">
      <div className="mb-2 flex items-center gap-2">
        <MapIcon size={14} className="text-muted" />
        <h3 className="text-[13px] font-medium">満たすストーリー</h3>
        <span className="text-[12px] text-faint">{stories.length}</span>
        <span className="text-[11.5px] text-faint">このイシューが何のためにあるか</span>
      </div>
      <div className="space-y-2">
        {stories.map((s) => (
          <div key={s.key} className="rounded-lg border border-line px-3 py-2 text-[13px]">
            <button onClick={() => navigate({ to: "/p/$key/storymap", params: { key: projectKey }, search: { peek: s.key } })}
              className="flex items-baseline gap-2 text-left hover:underline">
              <span className="font-mono text-[12px] text-faint">{s.key}</span><span className="font-medium">{s.title}</span>
            </button>
            {s.body && <p className="mt-0.5 line-clamp-3 whitespace-pre-wrap text-[12.5px] text-muted" data-testid="issue-story-body">{s.body}</p>}
            {(s.i_want || s.so_that) && (
              <p className="mt-0.5 text-[12.5px] text-muted">{s.i_want}{s.so_that && <> — そうすれば {s.so_that}</>}</p>
            )}
            {s.criteria.length > 0 && (
              <ol className="mt-1.5 space-y-0.5 text-[12.5px]">
                {s.criteria.map((c) => <li key={c.key} className="flex gap-2"><span className="shrink-0 font-mono text-[11px] text-faint">{c.key.split("-")[1]}</span><span>{criterionText(c)}</span></li>)}
              </ol>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}

/** One line for an acceptance criterion, also used when copying into an issue's acceptance checklist. */
export function criterionText(c: Pick<Criterion, "given" | "when" | "then">): string {
  // Option D: a one-line criterion stays one line.
  if (!c.given && !c.when) return c.then;
  return [c.given && `【前提】${c.given}`, c.when && `【操作】${c.when}`, `【結果】${c.then}`].filter(Boolean).join(" ");
}
