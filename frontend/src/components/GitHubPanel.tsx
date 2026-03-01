import { GitPullRequest, RefreshCw, Loader2 } from "lucide-react";
import { useGitHub } from "../hooks/useGitHub";
import { cn } from "../lib/utils";

function reviewBadge(decision: string) {
  switch (decision) {
    case "APPROVED":
      return <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-green-500/15 text-green-400">approved</span>;
    case "CHANGES_REQUESTED":
      return <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-red-500/15 text-red-400">changes</span>;
    case "REVIEW_REQUIRED":
      return <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-yellow-500/15 text-yellow-400">review</span>;
    default:
      return null;
  }
}

export default function GitHubPanel() {
  const { prs, loading, refresh } = useGitHub();

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="flex items-center justify-between p-3 border-b border-border">
        <h3 className="text-white text-sm">Pull Requests</h3>
        <button
          onClick={refresh}
          className="p-1 rounded hover:bg-white/10 transition-colors"
        >
          <RefreshCw className={cn("size-3.5 text-gray-400", loading && "animate-spin")} />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {loading && prs.length === 0 && (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 text-accent animate-spin" />
          </div>
        )}
        {prs.length === 0 && !loading && (
          <p className="text-xs text-gray-400 text-center py-4">No open PRs</p>
        )}
        {prs.map((pr) => (
          <div
            key={`${pr.repo}-${pr.number}`}
            className="p-3 bg-white/5 rounded-lg border border-border hover:bg-white/10 transition-colors cursor-pointer"
          >
            <div className="flex items-start gap-3">
              <GitPullRequest className="size-4 text-accent mt-0.5 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm text-white truncate mb-1">{pr.title}</p>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-gray-400">
                    {pr.repo}#{pr.number}
                  </span>
                  {reviewBadge(pr.reviewDecision)}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
