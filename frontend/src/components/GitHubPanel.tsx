import { GitPullRequest, Loader2 } from "lucide-react";
import type { GitHubPR } from "../types";

function reviewDot(decision: string) {
  switch (decision) {
    case "APPROVED":
      return <span className="size-1.5 rounded-full bg-success shrink-0" title="Approved" />;
    case "CHANGES_REQUESTED":
      return <span className="size-1.5 rounded-full bg-error shrink-0" title="Changes requested" />;
    case "REVIEW_REQUIRED":
      return <span className="size-1.5 rounded-full bg-warning shrink-0" title="Review required" />;
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

export default function GitHubPanel({ prs, loading, onPRClick }: GitHubPanelProps) {
  if (loading && prs.length === 0) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="size-5 text-accent animate-spin" />
      </div>
    );
  }

  if (prs.length === 0) {
    return <p className="text-xs text-muted-foreground text-center py-8">No open PRs</p>;
  }

  return (
    <div className="py-1">
      {prs.map((pr) => (
        // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: PR list item
        <div
          key={`${pr.repo}-${pr.number}`}
          className="flex items-center gap-2.5 px-3 py-2 hover:bg-white/[0.04] transition-colors cursor-pointer"
          onClick={() => onPRClick?.(pr)}
        >
          <GitPullRequest
            className={`size-3.5 shrink-0 ${pr.isDraft ? "text-tertiary" : "text-accent"}`}
          />
          <div className="flex-1 min-w-0">
            <p className="text-xs text-white truncate">{pr.title}</p>
            <p className="text-[10px] text-muted-foreground truncate">
              {pr.repo}#{pr.number} · {pr.author}
            </p>
          </div>
          {reviewDot(pr.reviewDecision)}
        </div>
      ))}
    </div>
  );
}
