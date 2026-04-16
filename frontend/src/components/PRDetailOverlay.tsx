import {
  AlertTriangle,
  Check,
  CheckCircle2,
  ChevronDown,
  Circle,
  Clock,
  Code2,
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
  MessageCircle,
  MessageSquare,
  Send,
  ShieldAlert,
  Users,
  X,
  XCircle,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import Markdown from "react-markdown";
import rehypeRaw from "rehype-raw";
import { HidePR, UnhidePR } from "../../wailsjs/go/main/ConfigService";
import {
  AddIssueComment,
  ApprovePR,
  FetchPRComments,
  FetchPRCommits,
  FetchPRFiles,
  FetchPRReviews,
  MergePR,
  ReplyToReviewComment,
} from "../../wailsjs/go/main/GitHubService";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { cn } from "../lib/utils";
import type {
  GitHubPR,
  PRCommentsData,
  PRCommentThread,
  PRCommit,
  PRFile,
  PRReview,
} from "../types";

interface PRDetailOverlayProps {
  open: boolean;
  prs: GitHubPR[];
  selectedPR: GitHubPR | null;
  onSelectPR: (pr: GitHubPR) => void;
  onClose: () => void;
  onRefresh: () => void;
  hiddenPRs: Set<string>;
  onHiddenChange: () => void;
  onOpenDiff?: (repo: string, number: number, path: string, files: PRFile[]) => void;
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
  details: ({ children }: { children?: React.ReactNode }) => (
    <details className="my-2 text-sm text-white/60 border border-white/[0.06] rounded-md overflow-hidden">
      {children}
    </details>
  ),
  summary: ({ children }: { children?: React.ReactNode }) => (
    <summary className="cursor-pointer px-3 py-1.5 text-white/70 font-medium hover:bg-white/[0.04] select-none">
      {children}
    </summary>
  ),
  img: ({ src, alt }: { src?: string; alt?: string }) =>
    src ? (
      <img
        src={src}
        alt={alt ?? ""}
        className="max-w-full rounded-md my-2 border border-white/[0.06]"
        loading="lazy"
      />
    ) : null,
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
  onOpenDiff,
}: PRDetailOverlayProps) {
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [files, setFiles] = useState<PRFile[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [reviews, setReviews] = useState<PRReview[]>([]);
  const [commits, setCommits] = useState<PRCommit[]>([]);
  const [commentsData, setCommentsData] = useState<PRCommentsData | null>(null);
  const [checksOpen, setChecksOpen] = useState(false);
  const [commitsOpen, setCommitsOpen] = useState(false);
  const [discussionOpen, setDiscussionOpen] = useState(true);
  const [codeCommentsOpen, setCodeCommentsOpen] = useState(true);
  const [replyingTo, setReplyingTo] = useState<number | null>(null);
  const [replyBody, setReplyBody] = useState("");
  const [replying, setReplying] = useState(false);
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
    setCommentsData(null);
    setReplyingTo(null);
    setReplyBody("");

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
    FetchPRComments(repo, number)
      .then((data) => setCommentsData(data))
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

  if (!open) return null;

  const pr = selectedPR;
  const isApproved = pr?.reviewDecision === "APPROVED";
  const checks = pr?.checks || [];
  const passedChecks = checks.filter((c) => c.conclusion === "SUCCESS").length;
  const failedChecks = checks.filter((c) => c.conclusion === "FAILURE").length;

  const visiblePRs = prs.filter((p) => !hiddenPRs.has(`${p.repo}#${p.number}`));
  const hiddenPRList = prs.filter((p) => hiddenPRs.has(`${p.repo}#${p.number}`));

  async function handleApprove() {
    if (!pr) return;
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
    if (!pr) return;
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

  async function handleReplyToThread(threadRootID: number) {
    if (!pr || !replyBody.trim()) return;
    setReplying(true);
    try {
      const newComment = await ReplyToReviewComment(
        pr.repo,
        pr.number,
        threadRootID,
        replyBody.trim(),
      );
      setCommentsData((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          reviewThreads: prev.reviewThreads.map((t) =>
            t.root.id === threadRootID ? { ...t, replies: [...t.replies, newComment] } : t,
          ),
        };
      });
      setReplyBody("");
      setReplyingTo(null);
    } catch {
      // ignore
    } finally {
      setReplying(false);
    }
  }

  async function handleAddIssueComment() {
    if (!pr || !replyBody.trim()) return;
    setReplying(true);
    try {
      const newComment = await AddIssueComment(pr.repo, pr.number, replyBody.trim());
      setCommentsData((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          issueComments: [...prev.issueComments, newComment],
        };
      });
      setReplyBody("");
      setReplyingTo(null);
    } catch {
      // ignore
    } finally {
      setReplying(false);
    }
  }

  // Group review threads by file path
  const threadsByFile: Record<string, PRCommentThread[]> = {};
  if (commentsData?.reviewThreads) {
    for (const thread of commentsData.reviewThreads) {
      const path = thread.root.path || "(file-level)";
      if (!threadsByFile[path]) threadsByFile[path] = [];
      threadsByFile[path].push(thread);
    }
  }

  const issueComments = commentsData?.issueComments || [];
  const reviewThreadCount = commentsData?.reviewThreads?.length || 0;

  return (
    <div className="fixed inset-0 z-50 flex">
      {/* Backdrop */}
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm animate-backdrop-in"
        onClick={onClose}
      />

      {/* Content */}
      <div className="relative flex w-full h-full animate-overlay-in">
        {/* Detail panel */}
        <div className="flex-1 flex flex-col overflow-hidden glass-overlay bg-[rgba(15,17,23,0.92)] m-4 mr-0 rounded-xl border border-white/[0.08] inset-highlight">
          {!pr ? (
            <div className="flex-1 flex flex-col items-center justify-center gap-3 text-muted-foreground">
              <GitPullRequest className="size-8 text-tertiary" />
              <p className="text-sm">No PR selected</p>
              <p className="text-xs text-tertiary">Select a PR from the sidebar</p>
            </div>
          ) : (
            <>
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
                      <span className="text-success">+{pr.additions}</span>
                      <span className="text-error">-{pr.deletions}</span>
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
                      "flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-all border",
                      isApproved
                        ? "text-success/50 border-success/20 cursor-default bg-success/[0.04]"
                        : "text-success border-success/30 hover:bg-success/10 hover:shadow-[0_0_12px_rgba(31,242,171,0.08)]",
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
                    className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-all border text-merge border-merge/30 hover:bg-merge/10 hover:shadow-[0_0_12px_color-mix(in_srgb,var(--color-merge)_15%,transparent)]"
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
                    className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-all border border-white/[0.08] text-muted-foreground hover:text-white hover:bg-white/[0.06] ml-auto"
                  >
                    <ExternalLink className="size-3.5" />
                    View on GitHub
                  </button>
                </div>

                {/* Description — right after actions */}
                {pr.body && (
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 flex-wrap">
                      <h3 className="text-xs text-muted-foreground font-medium uppercase tracking-wider">
                        Description
                      </h3>
                      {pr.labels &&
                        pr.labels.length > 0 &&
                        pr.labels.map((label) => (
                          <span
                            key={label}
                            className="text-[10px] px-2 py-0.5 rounded-full bg-white/[0.08] text-white/70 border border-white/[0.06]"
                          >
                            {label}
                          </span>
                        ))}
                    </div>
                    <div className="prose-sm bg-white/[0.03] rounded-xl p-4 border border-white/[0.06] inset-highlight max-h-[400px] overflow-auto">
                      {/* biome-ignore lint/suspicious/noExplicitAny: react-markdown component types */}
                      <Markdown rehypePlugins={[rehypeRaw]} components={mdComponents as any}>
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

                {/* Discussion (issue comments) */}
                {commentsData && (issueComments.length > 0 || replyingTo === -1) && (
                  <div className="space-y-2">
                    <button
                      type="button"
                      onClick={() => setDiscussionOpen(!discussionOpen)}
                      className="flex items-center gap-1.5 text-xs text-muted-foreground font-medium uppercase tracking-wider hover:text-white/70 transition-colors"
                    >
                      <ChevronDown
                        className={cn(
                          "size-3 transition-transform",
                          discussionOpen ? "" : "-rotate-90",
                        )}
                      />
                      <MessageCircle className="size-3" />
                      Discussion
                      <span className="text-tertiary normal-case ml-1">{issueComments.length}</span>
                    </button>
                    {discussionOpen && (
                      <div className="space-y-2">
                        {issueComments.map((comment) => (
                          <div
                            key={comment.id}
                            className="px-3 py-2.5 rounded-lg bg-white/[0.03] border border-white/[0.06] space-y-1.5"
                          >
                            <div className="flex items-center gap-2">
                              <span className="size-5 rounded-full bg-white/10 text-white/60 text-[10px] font-medium flex items-center justify-center shrink-0">
                                {authorInitial(comment.author)}
                              </span>
                              <span className="text-xs text-white/70 font-medium">
                                {comment.author}
                              </span>
                              {comment.authorType === "Bot" && (
                                <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-white/[0.08] text-white/40">
                                  Bot
                                </span>
                              )}
                              <span className="text-[10px] text-tertiary ml-auto">
                                {timeAgo(comment.createdAt)}
                              </span>
                            </div>
                            <div className="text-sm">
                              <Markdown
                                rehypePlugins={[rehypeRaw]}
                                // biome-ignore lint/suspicious/noExplicitAny: react-markdown component types
                                components={mdComponents as any}
                              >
                                {comment.body}
                              </Markdown>
                            </div>
                          </div>
                        ))}
                        {/* Add comment */}
                        {replyingTo === -1 ? (
                          <div className="space-y-2">
                            <textarea
                              className="w-full bg-white/[0.04] border border-white/[0.08] rounded-lg px-3 py-2 text-sm text-white/80 placeholder-white/30 resize-none focus:outline-none focus:border-accent/40"
                              rows={3}
                              placeholder="Add a comment..."
                              value={replyBody}
                              onChange={(e) => setReplyBody(e.target.value)}
                              onKeyDown={(e) => {
                                if (e.key === "Enter" && e.metaKey) handleAddIssueComment();
                              }}
                            />
                            <div className="flex items-center gap-2 justify-end">
                              <button
                                type="button"
                                onClick={() => {
                                  setReplyingTo(null);
                                  setReplyBody("");
                                }}
                                className="px-2.5 py-1 text-xs text-muted-foreground hover:text-white rounded-md transition-colors"
                              >
                                Cancel
                              </button>
                              <button
                                type="button"
                                disabled={replying || !replyBody.trim()}
                                onClick={handleAddIssueComment}
                                className="flex items-center gap-1.5 px-2.5 py-1 text-xs text-accent bg-accent/10 hover:bg-accent/20 rounded-md transition-colors disabled:opacity-40"
                              >
                                {replying ? (
                                  <Loader2 className="size-3 animate-spin" />
                                ) : (
                                  <Send className="size-3" />
                                )}
                                Comment
                              </button>
                            </div>
                          </div>
                        ) : (
                          <button
                            type="button"
                            onClick={() => {
                              setReplyingTo(-1);
                              setReplyBody("");
                            }}
                            className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-muted-foreground hover:text-white rounded-lg border border-dashed border-white/[0.08] hover:border-white/[0.15] transition-all w-full"
                          >
                            <MessageCircle className="size-3" />
                            Add a comment
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                )}

                {/* Code Comments (review threads) */}
                {reviewThreadCount > 0 && (
                  <div className="space-y-2">
                    <button
                      type="button"
                      onClick={() => setCodeCommentsOpen(!codeCommentsOpen)}
                      className="flex items-center gap-1.5 text-xs text-muted-foreground font-medium uppercase tracking-wider hover:text-white/70 transition-colors"
                    >
                      <ChevronDown
                        className={cn(
                          "size-3 transition-transform",
                          codeCommentsOpen ? "" : "-rotate-90",
                        )}
                      />
                      <Code2 className="size-3" />
                      Code Comments
                      <span className="text-tertiary normal-case ml-1">{reviewThreadCount}</span>
                    </button>
                    {codeCommentsOpen && (
                      <div className="space-y-4">
                        {Object.entries(threadsByFile).map(([filePath, threads]) => (
                          <div key={filePath} className="space-y-2">
                            <div className="flex items-center gap-1.5 text-xs text-white/50">
                              <FileText className="size-3" />
                              <span className="font-mono truncate">{filePath}</span>
                            </div>
                            {threads.map((thread) => {
                              const line = thread.root.line || thread.root.originalLine;
                              return (
                                <div
                                  key={thread.root.id}
                                  className="rounded-lg border border-white/[0.06] overflow-hidden"
                                >
                                  {/* Diff hunk context */}
                                  {thread.root.diffHunk && (
                                    <div className="bg-white/[0.02] px-3 py-1.5 border-b border-white/[0.06] overflow-auto max-h-[120px]">
                                      <pre className="text-[11px] font-mono leading-relaxed">
                                        {thread.root.diffHunk
                                          .split("\n")
                                          .slice(-5)
                                          .map((hunkLine, i) => (
                                            <div
                                              key={`${i}-${hunkLine}`}
                                              className={cn(
                                                hunkLine.startsWith("+")
                                                  ? "text-success/70"
                                                  : hunkLine.startsWith("-")
                                                    ? "text-error/70"
                                                    : hunkLine.startsWith("@@")
                                                      ? "text-accent/50"
                                                      : "text-white/30",
                                              )}
                                            >
                                              {hunkLine}
                                            </div>
                                          ))}
                                      </pre>
                                    </div>
                                  )}
                                  {/* Root comment */}
                                  <div className="px-3 py-2.5 bg-white/[0.03] space-y-1.5">
                                    <div className="flex items-center gap-2">
                                      <span className="size-5 rounded-full bg-white/10 text-white/60 text-[10px] font-medium flex items-center justify-center shrink-0">
                                        {authorInitial(thread.root.author)}
                                      </span>
                                      <span className="text-xs text-white/70 font-medium">
                                        {thread.root.author}
                                      </span>
                                      {thread.root.authorType === "Bot" && (
                                        <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-white/[0.08] text-white/40">
                                          Bot
                                        </span>
                                      )}
                                      {line > 0 && (
                                        <span className="text-[10px] text-accent/50 font-mono">
                                          L{line}
                                        </span>
                                      )}
                                      <span className="text-[10px] text-tertiary ml-auto">
                                        {timeAgo(thread.root.createdAt)}
                                      </span>
                                    </div>
                                    <div className="text-sm">
                                      <Markdown
                                        rehypePlugins={[rehypeRaw]}
                                        // biome-ignore lint/suspicious/noExplicitAny: react-markdown component types
                                        components={mdComponents as any}
                                      >
                                        {thread.root.body}
                                      </Markdown>
                                    </div>
                                  </div>
                                  {/* Replies */}
                                  {thread.replies.map((reply) => (
                                    <div
                                      key={reply.id}
                                      className="px-3 py-2.5 pl-7 bg-white/[0.02] border-t border-white/[0.04] space-y-1.5"
                                    >
                                      <div className="flex items-center gap-2">
                                        <span className="size-4 rounded-full bg-white/10 text-white/60 text-[9px] font-medium flex items-center justify-center shrink-0">
                                          {authorInitial(reply.author)}
                                        </span>
                                        <span className="text-xs text-white/70">
                                          {reply.author}
                                        </span>
                                        {reply.authorType === "Bot" && (
                                          <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-white/[0.08] text-white/40">
                                            Bot
                                          </span>
                                        )}
                                        <span className="text-[10px] text-tertiary ml-auto">
                                          {timeAgo(reply.createdAt)}
                                        </span>
                                      </div>
                                      <div className="text-sm">
                                        <Markdown
                                          rehypePlugins={[rehypeRaw]}
                                          // biome-ignore lint/suspicious/noExplicitAny: react-markdown component types
                                          components={mdComponents as any}
                                        >
                                          {reply.body}
                                        </Markdown>
                                      </div>
                                    </div>
                                  ))}
                                  {/* Reply action */}
                                  <div className="px-3 py-2 border-t border-white/[0.04]">
                                    {replyingTo === thread.root.id ? (
                                      <div className="space-y-2">
                                        <textarea
                                          className="w-full bg-white/[0.04] border border-white/[0.08] rounded-lg px-3 py-2 text-sm text-white/80 placeholder-white/30 resize-none focus:outline-none focus:border-accent/40"
                                          rows={2}
                                          placeholder="Write a reply..."
                                          value={replyBody}
                                          onChange={(e) => setReplyBody(e.target.value)}
                                          onKeyDown={(e) => {
                                            if (e.key === "Enter" && e.metaKey)
                                              handleReplyToThread(thread.root.id);
                                          }}
                                          // biome-ignore lint/a11y/noAutofocus: reply textarea should focus on open
                                          autoFocus
                                        />
                                        <div className="flex items-center gap-2 justify-end">
                                          <button
                                            type="button"
                                            onClick={() => {
                                              setReplyingTo(null);
                                              setReplyBody("");
                                            }}
                                            className="px-2.5 py-1 text-xs text-muted-foreground hover:text-white rounded-md transition-colors"
                                          >
                                            Cancel
                                          </button>
                                          <button
                                            type="button"
                                            disabled={replying || !replyBody.trim()}
                                            onClick={() => handleReplyToThread(thread.root.id)}
                                            className="flex items-center gap-1.5 px-2.5 py-1 text-xs text-accent bg-accent/10 hover:bg-accent/20 rounded-md transition-colors disabled:opacity-40"
                                          >
                                            {replying ? (
                                              <Loader2 className="size-3 animate-spin" />
                                            ) : (
                                              <Send className="size-3" />
                                            )}
                                            Reply
                                          </button>
                                        </div>
                                      </div>
                                    ) : (
                                      <button
                                        type="button"
                                        onClick={() => {
                                          setReplyingTo(thread.root.id);
                                          setReplyBody("");
                                        }}
                                        className="text-xs text-muted-foreground hover:text-white transition-colors"
                                      >
                                        Reply
                                      </button>
                                    )}
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        ))}
                      </div>
                    )}
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
                        <button
                          key={file.path}
                          type="button"
                          onClick={() => onOpenDiff?.(pr.repo, pr.number, file.path, files)}
                          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/[0.03] hover:bg-white/[0.06] border border-transparent hover:border-white/[0.06] transition-all text-left w-full group"
                        >
                          <FileText className="size-3.5 text-muted-foreground shrink-0" />
                          <span className="text-xs text-white/70 truncate flex-1 font-mono group-hover:text-white transition-colors">
                            {file.path}
                          </span>
                          <span className="text-[10px] text-success shrink-0">
                            +{file.additions}
                          </span>
                          <span className="text-[10px] text-error shrink-0">-{file.deletions}</span>
                        </button>
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
                        className={cn(
                          "size-3 transition-transform",
                          commitsOpen ? "" : "-rotate-90",
                        )}
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
                            <span className="text-[10px] text-tertiary shrink-0">
                              {commit.author}
                            </span>
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
                        className={cn(
                          "size-3 transition-transform",
                          checksOpen ? "" : "-rotate-90",
                        )}
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
              </div>
            </>
          )}
        </div>

        {/* PR list sidebar */}
        <div className="w-72 shrink-0 flex flex-col overflow-hidden glass-overlay bg-[rgba(15,17,23,0.92)] m-4 ml-2 rounded-xl border border-white/[0.08] inset-highlight">
          <div className="px-4 py-3 border-b border-white/[0.08]">
            <h3 className="text-sm text-white font-medium">
              Open PRs
              <span className="text-tertiary ml-1.5">{visiblePRs.length}</span>
            </h3>
          </div>
          <div className="flex-1 overflow-auto">
            {visiblePRs.map((p) => {
              const isSelected = pr != null && p.repo === pr.repo && p.number === pr.number;
              return (
                // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: PR list item
                <div
                  key={`${p.repo}-${p.number}`}
                  className={cn(
                    "px-4 py-3 cursor-pointer transition-all border-l-2 group",
                    isSelected
                      ? "bg-white/[0.06] border-accent glow-accent"
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
