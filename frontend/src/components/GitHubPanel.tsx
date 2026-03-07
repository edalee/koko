import { GitPullRequest, Loader2, RefreshCw } from "lucide-react";
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
}

export default function GitHubPanel({ prs, loading, refresh }: GitHubPanelProps) {
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
      {prs.map((pr) => (
        // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: PR card opens in browser
        <div
          key={`${pr.repo}-${pr.number}`}
          className="p-3 bg-white/5 rounded-lg border border-border hover:bg-white/10 transition-colors cursor-pointer"
          onClick={() => BrowserOpenURL(pr.url)}
        >
          <div className="flex items-start gap-3">
            <GitPullRequest className="size-4 text-accent mt-0.5 shrink-0" />
            <div className="flex-1 min-w-0">
              <p className="text-sm text-white truncate mb-1">{pr.title}</p>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">
                  {pr.repo}#{pr.number}
                </span>
                {reviewBadge(pr.reviewDecision)}
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
