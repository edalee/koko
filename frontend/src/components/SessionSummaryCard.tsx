import { GitBranch, GitPullRequest } from "lucide-react";
import type { main } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { type SavedSummary, timeAgo } from "../lib/summary";

/** A Jira key as a small mint chip. */
export function TicketChip({ ticket }: { ticket: string }) {
  return (
    <span className="shrink-0 px-1 rounded border border-accent/20 bg-accent/10 font-mono text-[10px] leading-relaxed text-accent">
      {ticket}
    </span>
  );
}

interface SessionSummaryCardProps {
  // The live summary, if fetched. saved is what the tab last kept.
  summary?: main.SessionSummary;
  saved: SavedSummary;
}

/**
 * The hover card of a session row (plan 036): the recap, branch, PR and
 * ticket, then the last prompts with their age.
 */
export default function SessionSummaryCard({ summary, saved }: SessionSummaryCardProps) {
  const title = summary?.title || saved.title;
  const recap = summary?.recap || saved.recap;
  const ticket = summary?.ticket || saved.ticket;
  const prompts = summary?.recentPrompts ?? [];
  const pr = summary?.pr;

  return (
    <div className="space-y-2.5 text-[12px]">
      {(title || ticket) && (
        <div className="flex items-start gap-1.5">
          {ticket && <TicketChip ticket={ticket} />}
          {title && <span className="text-[13px] font-medium text-white/90">{title}</span>}
        </div>
      )}
      {recap && <p className="leading-relaxed text-muted-foreground">{recap}</p>}

      {(summary?.branch || pr) && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-tertiary">
          {summary?.branch && (
            <span className="flex min-w-0 items-center gap-1">
              <GitBranch className="size-3 shrink-0" />
              <span className="truncate font-mono">{summary.branch}</span>
            </span>
          )}
          {pr && (
            <button
              type="button"
              onClick={() => BrowserOpenURL(pr.url)}
              className="flex items-center gap-1 text-muted-foreground hover:text-accent transition-colors"
            >
              <GitPullRequest className="size-3 shrink-0" />
              {pr.repo ? `${pr.repo}#${pr.number}` : `#${pr.number}`}
            </button>
          )}
        </div>
      )}

      {prompts.length > 0 && (
        <div className="space-y-1 border-t border-white/[0.08] pt-2">
          <p className="text-[10px] font-medium uppercase tracking-wider text-tertiary">
            Recent prompts
          </p>
          <ul className="space-y-1">
            {prompts.map((p, i) => (
              <li key={`${p.timestamp}-${i}`} className="flex items-baseline gap-2">
                <span className="flex-1 min-w-0 line-clamp-2 text-white/80">{p.text}</span>
                {p.timestamp > 0 && (
                  <span className="shrink-0 text-[10px] text-tertiary">{timeAgo(p.timestamp)}</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {!title && !recap && prompts.length === 0 && (
        <p className="text-tertiary">Nothing to summarise yet.</p>
      )}
    </div>
  );
}
