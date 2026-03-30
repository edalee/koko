import {
  AlertTriangle,
  Check,
  CheckCircle2,
  ChevronDown,
  Circle,
  Clock,
  ExternalLink,
  Eye,
  EyeOff,
  FileCode,
  FileText,
  GitBranch,
  GitCommit,
  GitMerge,
  GitPullRequest,
  Loader2,
  MessageSquare,
  ShieldAlert,
  Tag,
  Users,
  X,
  XCircle,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import Markdown from "react-markdown";
import { HidePR, UnhidePR } from "../../wailsjs/go/main/ConfigService";
import {
  ApprovePR,
  FetchPRCommits,
  FetchPRFiles,
  FetchPRReviews,
  MergePR,
} from "../../wailsjs/go/main/GitHubService";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { cn } from "../lib/utils";
import type { GitHubPR, PRCommit, PRFile, PRReview } from "../types";

interface PRDetailOverlayProps {
  open: boolean;
  prs: GitHubPR[];
  selectedPR: GitHubPR | null;
  onSelectPR: (pr: GitHubPR) => void;
  onClose: () => void;
  onRefresh: () => void;
  hiddenPRs: Set<string>;
  onHiddenChange: () => void;
}

function timeAgo(iso: string): string {
  if (!iso) return "";
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

function reviewBadge(decision: string) {
  switch (decision) {
    case "APPROVED":
      return (
        <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-success/15 text-success">
          Approved
        </span>
      );
    case "CHANGES_REQUESTED":
      return (
        <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-error/15 text-error">
          Changes requested
        </span>
      );
    case "REVIEW_REQUIRED":
      return (
        <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-warning/15 text-warning">
          Review required
        </span>
      );
    default:
      return null;
  }
}

function reviewStateBadge(state: string) {
  switch (state) {
    case "APPROVED":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-success/15 text-success">
          Approved
        </span>
      );
    case "CHANGES_REQUESTED":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-error/15 text-error">
          Changes
        </span>
      );
    case "COMMENTED":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-white/10 text-white/50">
          Commented
        </span>
      );
    case "PENDING":
      return (
        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-warning/15 text-warning">
          Pending
        </span>
      );
    default:
      return null;
  }
}

function checkIcon(conclusion: string) {
  switch (conclusion) {
    case "SUCCESS":
      return <CheckCircle2 className="size-3.5 text-success" />;
    case "FAILURE":
      return <XCircle className="size-3.5 text-error" />;
    case "CANCELLED":
    case "SKIPPED":
      return <Circle className="size-3.5 text-tertiary" />;
    default:
      return <Clock className="size-3.5 text-warning" />;
  }
}

function mergeStateLabel(status: string) {
  switch (status) {
    case "CLEAN":
      return null;
    case "BLOCKED":
      return (
        <span className="flex items-center gap-1.5 text-xs text-warning">
          <ShieldAlert className="size-3.5" />
          Blocked by required reviews
        </span>
      );
    case "DIRTY":
      return (
        <span className="flex items-center gap-1.5 text-xs text-error">
          <XCircle className="size-3.5" />
          Has merge conflicts
        </span>
      );
    case "UNSTABLE":
      return (
        <span className="flex items-center gap-1.5 text-xs text-warning">
          <AlertTriangle className="size-3.5" />
          CI checks failing
        </span>
      );
    case "BEHIND":
      return (
        <span className="flex items-center gap-1.5 text-xs text-warning">
          <GitBranch className="size-3.5" />
          Behind base branch
        </span>
      );
    default:
      return null;
  }
}

function authorInitial(name: string) {
  return (name || "?")[0].toUpperCase();
}

// Markdown component overrides (extracted to reduce JSX nesting)
const mdComponents = {
  h1: ({ children }: { children?: React.ReactNode }) => (
    <h1 className="text-base text-white font-semibold mt-3 mb-1">{children}</h1>
  ),
  h2: ({ children }: { children?: React.ReactNode }) => (
    <h2 className="text-sm text-white font-semibold mt-3 mb-1">{children}</h2>
  ),
  h3: ({ children }: { children?: React.ReactNode }) => (
    <h3 className="text-sm text-white/80 font-medium mt-2 mb-1">{children}</h3>
  ),
  h4: ({ children }: { children?: React.ReactNode }) => (
    <h4 className="text-xs text-white/70 font-medium mt-2 mb-1">{children}</h4>
  ),
  p: ({ children }: { children?: React.ReactNode }) => (
    <p className="text-sm text-white/60 leading-relaxed mb-2">{children}</p>
  ),
  ul: ({ children }: { children?: React.ReactNode }) => (
    <ul className="text-sm text-white/60 list-disc pl-5 mb-2 space-y-0.5">{children}</ul>
  ),
  ol: ({ children }: { children?: React.ReactNode }) => (
    <ol className="text-sm text-white/60 list-decimal pl-5 mb-2 space-y-0.5">{children}</ol>
  ),
  li: ({ children }: { children?: React.ReactNode }) => (
    <li className="leading-relaxed">{children}</li>
  ),
  code: ({ children, className }: { children?: React.ReactNode; className?: string }) =>
    className ? (
      <code className="block text-xs font-mono bg-white/[0.06] rounded-md p-3 my-2 text-white/70 overflow-auto">
        {children}
      </code>
    ) : (
      <code className="text-xs font-mono bg-white/[0.06] rounded px-1 py-0.5 text-accent/80">
        {children}
      </code>
    ),
  pre: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  a: ({ href, children }: { href?: string; children?: React.ReactNode }) => (
    <a
      href={href}
      className="text-accent hover:underline"
      onClick={(e) => {
        e.preventDefault();
        if (href) {
          import("../../wailsjs/runtime/runtime").then((r) => r.BrowserOpenURL(href));
        }
      }}
    >
      {children}
    </a>
  ),
  strong: ({ children }: { children?: React.ReactNode }) => (
    <strong className="text-white/80 font-medium">{children}</strong>
  ),
  blockquote: ({ children }: { children?: React.ReactNode }) => (
    <blockquote className="border-l-2 border-white/[0.1] pl-3 my-2 text-white/50 italic">
      {children}
    </blockquote>
  ),
  hr: () => <hr className="border-white/[0.06] my-3" />,
  table: ({ children }: { children?: React.ReactNode }) => (
    <table className="text-xs text-white/60 border-collapse my-2 w-full">{children}</table>
  ),
  th: ({ children }: { children?: React.ReactNode }) => (
    <th className="text-left text-white/70 font-medium px-2 py-1 border-b border-white/[0.08]">
      {children}
    </th>
  ),
  td: ({ children }: { children?: React.ReactNode }) => (
    <td className="px-2 py-1 border-b border-white/[0.04]">{children}</td>
  ),
};

export default function PRDetailOverlay({
  open,
  prs,
  selectedPR,
  onSelectPR,
  onClose,
  onRefresh,
  hiddenPRs,
  onHiddenChange,
}: PRDetailOverlayProps) {
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [files, setFiles] = useState<PRFile[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [reviews, setReviews] = useState<PRReview[]>([]);
  const [commits, setCommits] = useState<PRCommit[]>([]);
  const [checksOpen, setChecksOpen] = useState(false);
  const [commitsOpen, setCommitsOpen] = useState(false);
  const [showHidden, setShowHidden] = useState(false);
  // Track which PR's detail data is loaded to avoid redundant fetches
  const [loadedPRKey, setLoadedPRKey] = useState<string>("");

  const prKey = selectedPR ? `${selectedPR.repo}#${selectedPR.number}` : "";

  // Load detail data when selected PR changes
  useEffect(() => {
    if (!open || !selectedPR || prKey === loadedPRKey) return;
    setLoadedPRKey(prKey);

    const { repo, number } = selectedPR;
    setFilesLoading(true);
    setFiles([]);
    setReviews([]);
    setCommits([]);

    FetchPRFiles(repo, number)
      .then((f) => setFiles(f || []))
      .catch(() => {})
      .finally(() => setFilesLoading(false));
    FetchPRReviews(repo, number)
      .then((r) => setReviews(r || []))
      .catch(() => {});
    FetchPRCommits(repo, number)
      .then((c) => setCommits(c || []))
      .catch(() => {});
  }, [open, selectedPR, prKey, loadedPRKey]);

  // Auto-expand checks if any failed
  useEffect(() => {
    if (!selectedPR) return;
    const checks = selectedPR.checks || [];
    const hasFailed = checks.some((c) => c.conclusion === "FAILURE");
    setChecksOpen(hasFailed);
  }, [selectedPR]);

  // Reset loaded key when overlay closes
  useEffect(() => {
    if (!open) setLoadedPRKey("");
  }, [open]);

  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [open, onClose]);

  const handleHide = useCallback(
    async (repo: string, number: number, e: React.MouseEvent) => {
      e.stopPropagation();
      await HidePR(repo, number);
      onHiddenChange();
    },
    [onHiddenChange],
  );

  const handleUnhide = useCallback(
    async (repo: string, number: number, e: React.MouseEvent) => {
      e.stopPropagation();
      await UnhidePR(repo, number);
      onHiddenChange();
    },
    [onHiddenChange],
  );

  if (!open || !selectedPR) return null;

  const pr = selectedPR;
  const isApproved = pr.reviewDecision === "APPROVED";
  const checks = pr.checks || [];
  const passedChecks = checks.filter((c) => c.conclusion === "SUCCESS").length;
  const failedChecks = checks.filter((c) => c.conclusion === "FAILURE").length;

  const visiblePRs = prs.filter((p) => !hiddenPRs.has(`${p.repo}#${p.number}`));
  const hiddenPRList = prs.filter((p) => hiddenPRs.has(`${p.repo}#${p.number}`));

  async function handleApprove() {
    setBusyAction("approve");
    try {
      await ApprovePR(pr.repo, pr.number);
      onRefresh();
    } catch {
      // ignore
    } finally {
      setBusyAction(null);
    }
  }

  async function handleMerge() {
    setBusyAction("merge");
    try {
      await MergePR(pr.repo, pr.number);
      onRefresh();
    } catch {
      // ignore
    } finally {
      setBusyAction(null);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex">
      {/* Backdrop */}
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop */}
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={onClose} />

      {/* Content */}
      <div className="relative flex w-full h-full">
        {/* Detail panel */}
        <div className="flex-1 flex flex-col overflow-hidden glass-overlay bg-[rgba(15,17,23,0.92)] m-4 mr-0 rounded-xl border border-white/[0.08]">
          {/* Header — title + author prominent */}
          <div className="flex items-start gap-3 px-6 py-4 border-b border-white/[0.08]">
            <GitPullRequest className="size-5 text-accent mt-0.5 shrink-0" />
            <div className="flex-1 min-w-0">
              <h2 className="text-lg text-white font-medium leading-tight">{pr.title}</h2>
              <div className="flex items-center gap-3 mt-2">
                {/* Author prominent */}
                <div className="flex items-center gap-1.5">
                  <span className="size-5 rounded-full bg-accent/20 text-accent text-[10px] font-medium flex items-center justify-center shrink-0">
                    {authorInitial(pr.author)}
                  </span>
                  <span className="text-sm text-white/80 font-medium">{pr.author}</span>
                </div>
                <span className="text-xs text-muted-foreground">
                  {pr.repo}#{pr.number}
                </span>
                {reviewBadge(pr.reviewDecision)}
                {pr.isDraft && (
                  <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-white/10 text-white/50">
                    Draft
                  </span>
                )}
                {pr.assignees?.length > 0 && (
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Users className="size-3" />
                    {pr.assignees.join(", ")}
                  </span>
                )}
              </div>
              {/* Branch + stats row */}
              <div className="flex items-center gap-4 mt-2 flex-wrap">
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <GitBranch className="size-3.5" />
                  <span className="font-mono">{pr.headRef}</span>
                  <span className="text-tertiary">→</span>
                  <span className="font-mono">{pr.baseRef}</span>
                </div>
                <div className="flex items-center gap-3 text-xs">
                  <span className="text-green-400">+{pr.additions}</span>
                  <span className="text-red-400">-{pr.deletions}</span>
                  <span className="text-muted-foreground">
                    <FileText className="size-3 inline mr-1" />
                    {pr.changedFiles} files
                  </span>
                </div>
                {pr.createdAt && (
                  <span className="text-[10px] text-tertiary">{timeAgo(pr.createdAt)}</span>
                )}
              </div>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="p-1.5 rounded-md text-muted-foreground hover:text-white hover:bg-white/10 transition-colors shrink-0"
            >
              <X className="size-5" />
            </button>
          </div>

          {/* Scrollable body */}
          <div className="flex-1 overflow-auto px-6 py-4 space-y-5">
            {/* Merge state */}
            {pr.mergeStateStatus && mergeStateLabel(pr.mergeStateStatus)}

            {/* Mergeable status */}
            {pr.mergeable && pr.mergeable !== "UNKNOWN" && !pr.mergeStateStatus && (
              <div className="flex items-center gap-2 text-xs">
                {pr.mergeable === "MERGEABLE" ? (
                  <>
                    <CheckCircle2 className="size-3.5 text-success" />
                    <span className="text-success">No conflicts</span>
                  </>
                ) : (
                  <>
                    <XCircle className="size-3.5 text-error" />
                    <span className="text-error">Has conflicts</span>
                  </>
                )}
              </div>
            )}

            {/* Actions */}
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={busyAction === "approve" || isApproved}
                onClick={handleApprove}
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-colors border",
                  isApproved
                    ? "text-success/50 border-success/20 cursor-default"
                    : "text-success border-success/30 hover:bg-success/10",
                )}
              >
                {busyAction === "approve" ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <Check className="size-3.5" />
                )}
                {isApproved ? "Approved" : "Approve"}
              </button>

              <button
                type="button"
                disabled={busyAction === "merge"}
                onClick={handleMerge}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-colors border text-purple-400 border-purple-400/30 hover:bg-purple-400/10"
              >
                {busyAction === "merge" ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <GitMerge className="size-3.5" />
                )}
                Squash & Merge
              </button>

              <button
                type="button"
                onClick={() => BrowserOpenURL(pr.url)}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-colors border border-white/[0.08] text-muted-foreground hover:text-white hover:bg-white/[0.06] ml-auto"
              >
                <ExternalLink className="size-3.5" />
                View on GitHub
              </button>
            </div>

            {/* Description — right after actions */}
            {pr.body && (
              <div className="space-y-2">
                <h3 className="text-xs text-muted-foreground font-medium uppercase tracking-wider">
                  Description
                </h3>
                <div className="prose-sm bg-white/[0.03] rounded-lg p-4 border border-white/[0.06] max-h-[400px] overflow-auto">
                  {/* biome-ignore lint/suspicious/noExplicitAny: react-markdown component types */}
                  <Markdown components={mdComponents as any}>
                    {pr.body.replace(/<!--[\s\S]*?-->/g, "").trim()}
                  </Markdown>
                </div>
              </div>
            )}

            {/* Reviews */}
            {reviews.length > 0 && (
              <div className="space-y-2">
                <h3 className="text-xs text-muted-foreground font-medium uppercase tracking-wider">
                  <MessageSquare className="size-3 inline mr-1.5" />
                  Reviews
                  <span className="ml-2 text-tertiary normal-case">{reviews.length}</span>
                </h3>
                <div className="grid gap-1">
                  {reviews.map((review) => (
                    <div
                      key={`${review.author}-${review.state}`}
                      className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-white/[0.03]"
                    >
                      <span className="size-5 rounded-full bg-white/10 text-white/60 text-[10px] font-medium flex items-center justify-center shrink-0">
                        {authorInitial(review.author)}
                      </span>
                      <span className="text-xs text-white/70">{review.author}</span>
                      <span className="ml-auto">{reviewStateBadge(review.state)}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* File Changes */}
            <div className="space-y-2">
              <h3 className="text-xs text-muted-foreground font-medium uppercase tracking-wider">
                <FileCode className="size-3 inline mr-1.5" />
                Files changed
                <span className="ml-2 text-tertiary normal-case">
                  {filesLoading ? "..." : files.length}
                </span>
              </h3>
              {filesLoading ? (
                <div className="flex items-center gap-2 px-3 py-2">
                  <Loader2 className="size-3.5 text-accent animate-spin" />
                  <span className="text-xs text-muted-foreground">Loading files...</span>
                </div>
              ) : files.length > 0 ? (
                <div className="grid gap-0.5 max-h-[300px] overflow-auto">
                  {files.map((file) => (
                    <div
                      key={file.path}
                      className="flex items-center gap-2 px-3 py-1 rounded-md bg-white/[0.03] group"
                    >
                      <FileText className="size-3.5 text-muted-foreground shrink-0" />
                      <span className="text-xs text-white/70 truncate flex-1 font-mono">
                        {file.path}
                      </span>
                      <span className="text-[10px] text-green-400 shrink-0">+{file.additions}</span>
                      <span className="text-[10px] text-red-400 shrink-0">-{file.deletions}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-tertiary px-3">No file data available</p>
              )}
            </div>

            {/* Commits — collapsible */}
            {commits.length > 0 && (
              <div className="space-y-2">
                <button
                  type="button"
                  onClick={() => setCommitsOpen(!commitsOpen)}
                  className="flex items-center gap-1.5 text-xs text-muted-foreground font-medium uppercase tracking-wider hover:text-white/70 transition-colors"
                >
                  <ChevronDown
                    className={cn("size-3 transition-transform", commitsOpen ? "" : "-rotate-90")}
                  />
                  <GitCommit className="size-3" />
                  Commits
                  <span className="text-tertiary normal-case ml-1">{commits.length}</span>
                </button>
                {commitsOpen && (
                  <div className="grid gap-0.5 max-h-[200px] overflow-auto">
                    {commits.map((commit) => (
                      <div
                        key={commit.sha}
                        className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-white/[0.03]"
                      >
                        <span className="text-[10px] text-accent/60 font-mono shrink-0">
                          {commit.sha.slice(0, 7)}
                        </span>
                        <span className="text-xs text-white/70 truncate flex-1">
                          {commit.message}
                        </span>
                        <span className="text-[10px] text-tertiary shrink-0">{commit.author}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* CI Checks — collapsible */}
            {checks.length > 0 && (
              <div className="space-y-2">
                <button
                  type="button"
                  onClick={() => setChecksOpen(!checksOpen)}
                  className="flex items-center gap-1.5 text-xs text-muted-foreground font-medium uppercase tracking-wider hover:text-white/70 transition-colors"
                >
                  <ChevronDown
                    className={cn("size-3 transition-transform", checksOpen ? "" : "-rotate-90")}
                  />
                  Checks
                  <span className="text-tertiary normal-case ml-1">
                    {passedChecks}/{checks.length} passed
                    {failedChecks > 0 && (
                      <span className="text-error ml-1">({failedChecks} failed)</span>
                    )}
                  </span>
                </button>
                {checksOpen && (
                  <div className="grid gap-1">
                    {checks.map((check) => (
                      <div
                        key={check.name}
                        className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-white/[0.03]"
                      >
                        {checkIcon(check.conclusion)}
                        <span className="text-xs text-white/70 truncate">{check.name}</span>
                        <span className="text-[10px] text-tertiary ml-auto">
                          {check.conclusion?.toLowerCase() || check.status?.toLowerCase()}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Labels */}
            {pr.labels && pr.labels.length > 0 && (
              <div className="flex items-center gap-2 flex-wrap">
                <Tag className="size-3 text-tertiary" />
                {pr.labels.map((label) => (
                  <span
                    key={label}
                    className="text-[10px] px-2 py-0.5 rounded-full bg-white/[0.08] text-white/70 border border-white/[0.06]"
                  >
                    {label}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* PR list sidebar */}
        <div className="w-72 shrink-0 flex flex-col overflow-hidden glass-overlay bg-[rgba(15,17,23,0.92)] m-4 ml-2 rounded-xl border border-white/[0.08]">
          <div className="px-4 py-3 border-b border-white/[0.08]">
            <h3 className="text-sm text-white font-medium">
              Open PRs
              <span className="text-tertiary ml-1.5">{visiblePRs.length}</span>
            </h3>
          </div>
          <div className="flex-1 overflow-auto">
            {visiblePRs.map((p) => {
              const isSelected = p.repo === pr.repo && p.number === pr.number;
              return (
                // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: PR list item
                <div
                  key={`${p.repo}-${p.number}`}
                  className={cn(
                    "px-4 py-3 cursor-pointer transition-colors border-l-2 group",
                    isSelected
                      ? "bg-white/[0.06] border-accent"
                      : "border-transparent hover:bg-white/[0.04]",
                  )}
                  onClick={() => onSelectPR(p)}
                >
                  <div className="flex items-center gap-2">
                    <p
                      className={cn(
                        "text-sm truncate flex-1",
                        isSelected ? "text-white" : "text-white/70",
                      )}
                    >
                      {p.title}
                    </p>
                    <button
                      type="button"
                      onClick={(e) => handleHide(p.repo, p.number, e)}
                      className="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-white/10 transition-all shrink-0"
                      title="Hide this PR"
                    >
                      <EyeOff className="size-3 text-muted-foreground" />
                    </button>
                  </div>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="text-[10px] text-muted-foreground">
                      {p.repo}#{p.number}
                    </span>
                    <span className="text-[10px] text-tertiary">{p.author}</span>
                  </div>
                </div>
              );
            })}

            {/* Hidden PRs toggle */}
            {hiddenPRList.length > 0 && (
              <>
                <button
                  type="button"
                  onClick={() => setShowHidden(!showHidden)}
                  className="flex items-center gap-1.5 px-4 py-2 w-full text-left text-[10px] text-tertiary hover:text-white/50 transition-colors border-t border-white/[0.06]"
                >
                  <Eye className="size-3" />
                  {showHidden ? "Hide" : "Show"} {hiddenPRList.length} hidden
                </button>
                {showHidden &&
                  hiddenPRList.map((p) => (
                    // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: hidden PR item
                    <div
                      key={`hidden-${p.repo}-${p.number}`}
                      className="px-4 py-2 cursor-pointer transition-colors opacity-40 hover:opacity-70 group"
                      onClick={() => onSelectPR(p)}
                    >
                      <div className="flex items-center gap-2">
                        <p className="text-xs truncate flex-1 text-white/50">{p.title}</p>
                        <button
                          type="button"
                          onClick={(e) => handleUnhide(p.repo, p.number, e)}
                          className="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-white/10 transition-all shrink-0"
                          title="Show this PR"
                        >
                          <Eye className="size-3 text-muted-foreground" />
                        </button>
                      </div>
                      <span className="text-[10px] text-muted-foreground">
                        {p.repo}#{p.number}
                      </span>
                    </div>
                  ))}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
