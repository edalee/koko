import { GitPullRequest, RefreshCw, Loader2 } from "lucide-react";
import { useGitHub } from "../hooks/useGitHub";
import { cn } from "../lib/utils";

interface GitHubPanelProps {
  expanded: boolean;
  onToggle: () => void;
}

function reviewBadge(decision: string) {
  switch (decision) {
    case "APPROVED":
      return <span className="text-[10px] text-success">approved</span>;
    case "CHANGES_REQUESTED":
      return <span className="text-[10px] text-error">changes</span>;
    case "REVIEW_REQUIRED":
      return <span className="text-[10px] text-warning">review</span>;
    default:
      return null;
  }
}

export default function GitHubPanel({ expanded, onToggle }: GitHubPanelProps) {
  const { prs, loading, refresh } = useGitHub();

  return (
    <div className="bg-surface overflow-hidden">
      {/* Header */}
      <button
        className="flex w-full items-center gap-2 px-3 py-2 text-xs hover:bg-tab-hover transition-colors"
        onClick={onToggle}
      >
        <GitPullRequest className="h-3.5 w-3.5 text-accent" />
        <span className="font-medium text-foreground">GitHub</span>
        <span className="ml-auto text-muted-foreground">
          {loading ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            `${prs.length} PRs`
          )}
        </span>
        <span
          className="p-0.5 rounded hover:bg-border transition-colors"
          onClick={(e) => {
            e.stopPropagation();
            refresh();
          }}
        >
          <RefreshCw className={cn("h-3 w-3 text-muted-foreground", loading && "animate-spin")} />
        </span>
      </button>

      {/* Expanded content */}
      {expanded && (
        <div className="border-t border-border/30 max-h-64 overflow-y-auto">
          {prs.length === 0 && !loading && (
            <p className="px-3 py-2 text-xs text-muted-foreground">No open PRs</p>
          )}
          {prs.map((pr) => (
            <div
              key={`${pr.repo}-${pr.number}`}
              className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-tab-hover transition-colors"
            >
              <span className="text-muted-foreground shrink-0">
                {pr.repo}#{pr.number}
              </span>
              <span className="truncate text-foreground">{pr.title}</span>
              <span className="ml-auto shrink-0">{reviewBadge(pr.reviewDecision)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
