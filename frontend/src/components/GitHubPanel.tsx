import { Check, ExternalLink, GitMerge, GitPullRequest, Loader2, RefreshCw } from "lucide-react";
import { useState } from "react";
import { ApprovePR, MergePR } from "../../wailsjs/go/main/GitHubService";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { cn } from "../lib/utils";
import type { GitHubPR } from "../types";

function reviewBadge(decision: string) {
  switch (decision) {
    case "APPROVED":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-success/15 text-success">
          approved
        </span>
      );
    case "CHANGES_REQUESTED":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-error/15 text-error">
          changes
        </span>
      );
    case "REVIEW_REQUIRED":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-warning/15 text-warning">
          review
        </span>
      );
    default:
      return null;
  }
}

interface GitHubPanelProps {
  prs: GitHubPR[];
  loading: boolean;
  refresh: () => void;
  onPRClick?: (pr: GitHubPR) => void;
}

export default function GitHubPanel({ prs, loading, refresh, onPRClick }: GitHubPanelProps) {
  const [busyAction, setBusyAction] = useState<string | null>(null);

  async function handleApprove(pr: GitHubPR) {
    const key = `approve-${pr.repo}-${pr.number}`;
    setBusyAction(key);
    try {
      await ApprovePR(pr.repo, pr.number);
      refresh();
    } catch {
      // gh CLI error — user may not have permission
    } finally {
      setBusyAction(null);
    }
  }

  async function handleMerge(pr: GitHubPR) {
    const key = `merge-${pr.repo}-${pr.number}`;
    setBusyAction(key);
    try {
      await MergePR(pr.repo, pr.number);
      refresh();
    } catch {
      // gh CLI error — merge may not be allowed
    } finally {
      setBusyAction(null);
    }
  }

  return (
    <div className="p-4 space-y-2">
      <div className="flex items-center justify-end mb-2">
        <button
          type="button"
          onClick={refresh}
          className="p-1 rounded hover:bg-white/10 transition-colors"
        >
          <RefreshCw className={cn("size-3.5 text-muted-foreground", loading && "animate-spin")} />
        </button>
      </div>

      {loading && prs.length === 0 && (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="size-5 text-accent animate-spin" />
        </div>
      )}
      {prs.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground text-center py-4">No open PRs</p>
      )}
      {prs.map((pr) => {
        const approveKey = `approve-${pr.repo}-${pr.number}`;
        const mergeKey = `merge-${pr.repo}-${pr.number}`;
        const isApproved = pr.reviewDecision === "APPROVED";

        return (
          // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: PR card click
          <div
            key={`${pr.repo}-${pr.number}`}
            className="p-3 bg-white/[0.06] glass-card rounded-xl border border-border inset-highlight hover:bg-white/[0.09] hover:border-white/[0.12] transition-all group cursor-pointer"
            onClick={() => onPRClick?.(pr)}
          >
            <div className="flex items-start gap-3">
              <GitPullRequest className="size-4 text-accent mt-0.5 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm text-white truncate mb-1">{pr.title}</p>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">
                    {pr.repo}#{pr.number}
                  </span>
                  <span className="text-xs text-muted-foreground/50">&middot;</span>
                  <span className="text-xs text-muted-foreground">{pr.author}</span>
                  {reviewBadge(pr.reviewDecision)}
                </div>
              </div>
            </div>

            {/* Action buttons — visible on hover */}
            <div className="flex items-center gap-1.5 mt-2 pt-2 border-t border-white/[0.06] opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                type="button"
                title="Approve"
                disabled={busyAction === approveKey || isApproved}
                onClick={() => handleApprove(pr)}
                className={cn(
                  "flex items-center gap-1 px-2 py-1 text-[11px] rounded-md transition-colors",
                  isApproved
                    ? "text-success/50 cursor-default"
                    : "text-success hover:bg-success/10",
                )}
              >
                {busyAction === approveKey ? (
                  <Loader2 className="size-3 animate-spin" />
                ) : (
                  <Check className="size-3" />
                )}
                Approve
              </button>

              <button
                type="button"
                title="Squash & merge"
                disabled={busyAction === mergeKey}
                onClick={() => handleMerge(pr)}
                className="flex items-center gap-1 px-2 py-1 text-[11px] rounded-md text-purple-400 hover:bg-purple-400/10 transition-colors"
              >
                {busyAction === mergeKey ? (
                  <Loader2 className="size-3 animate-spin" />
                ) : (
                  <GitMerge className="size-3" />
                )}
                Merge
              </button>

              <button
                type="button"
                title="Open in browser"
                onClick={() => BrowserOpenURL(pr.url)}
                className="flex items-center gap-1 px-2 py-1 text-[11px] rounded-md text-muted-foreground hover:text-white hover:bg-white/[0.06] transition-colors ml-auto"
              >
                <ExternalLink className="size-3" />
                View
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
