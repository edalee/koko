import {
  Check,
  CheckCircle2,
  Circle,
  Clock,
  ExternalLink,
  FileText,
  GitBranch,
  GitMerge,
  GitPullRequest,
  Loader2,
  Tag,
  X,
  XCircle,
} from "lucide-react";
import { useEffect, useState } from "react";
import Markdown from "react-markdown";
import { ApprovePR, MergePR } from "../../wailsjs/go/main/GitHubService";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { cn } from "../lib/utils";
import type { GitHubPR } from "../types";

interface PRDetailOverlayProps {
  open: boolean;
  prs: GitHubPR[];
  selectedPR: GitHubPR | null;
  onSelectPR: (pr: GitHubPR) => void;
  onClose: () => void;
  onRefresh: () => void;
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

export default function PRDetailOverlay({
  open,
  prs,
  selectedPR,
  onSelectPR,
  onClose,
  onRefresh,
}: PRDetailOverlayProps) {
  const [busyAction, setBusyAction] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [open, onClose]);

  if (!open || !selectedPR) return null;

  const pr = selectedPR;
  const isApproved = pr.reviewDecision === "APPROVED";
  const checks = pr.checks || [];
  const passedChecks = checks.filter((c) => c.conclusion === "SUCCESS").length;
  const failedChecks = checks.filter((c) => c.conclusion === "FAILURE").length;

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
        <div className="flex-1 flex flex-col overflow-hidden glass-overlay m-4 mr-0 rounded-xl border border-white/[0.08]">
          {/* Header */}
          <div className="flex items-start gap-3 px-6 py-4 border-b border-white/[0.08]">
            <GitPullRequest className="size-5 text-accent mt-0.5 shrink-0" />
            <div className="flex-1 min-w-0">
              <h2 className="text-lg text-white font-medium leading-tight">{pr.title}</h2>
              <div className="flex items-center gap-2 mt-2 flex-wrap">
                <span className="text-xs text-muted-foreground">
                  {pr.repo}#{pr.number}
                </span>
                <span className="text-xs text-muted-foreground">by {pr.author}</span>
                {reviewBadge(pr.reviewDecision)}
                {pr.isDraft && (
                  <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-white/10 text-white/50">
                    Draft
                  </span>
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
            {/* Branch + stats */}
            <div className="flex items-center gap-4 flex-wrap">
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

            {/* CI Checks */}
            {checks.length > 0 && (
              <div className="space-y-2">
                <h3 className="text-xs text-muted-foreground font-medium uppercase tracking-wider">
                  Checks
                  <span className="ml-2 text-tertiary normal-case">
                    {passedChecks}/{checks.length} passed
                    {failedChecks > 0 && (
                      <span className="text-error ml-1">({failedChecks} failed)</span>
                    )}
                  </span>
                </h3>
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
              </div>
            )}

            {/* Description */}
            {pr.body && (
              <div className="space-y-2">
                <h3 className="text-xs text-muted-foreground font-medium uppercase tracking-wider">
                  Description
                </h3>
                <div className="prose-sm bg-white/[0.03] rounded-lg p-4 border border-white/[0.06] max-h-[400px] overflow-auto">
                  <Markdown
                    components={{
                      h1: ({ children }) => (
                        <h1 className="text-base text-white font-semibold mt-3 mb-1">{children}</h1>
                      ),
                      h2: ({ children }) => (
                        <h2 className="text-sm text-white font-semibold mt-3 mb-1">{children}</h2>
                      ),
                      h3: ({ children }) => (
                        <h3 className="text-sm text-white/80 font-medium mt-2 mb-1">{children}</h3>
                      ),
                      h4: ({ children }) => (
                        <h4 className="text-xs text-white/70 font-medium mt-2 mb-1">{children}</h4>
                      ),
                      p: ({ children }) => (
                        <p className="text-sm text-white/60 leading-relaxed mb-2">{children}</p>
                      ),
                      ul: ({ children }) => (
                        <ul className="text-sm text-white/60 list-disc pl-5 mb-2 space-y-0.5">
                          {children}
                        </ul>
                      ),
                      ol: ({ children }) => (
                        <ol className="text-sm text-white/60 list-decimal pl-5 mb-2 space-y-0.5">
                          {children}
                        </ol>
                      ),
                      li: ({ children }) => <li className="leading-relaxed">{children}</li>,
                      code: ({ children, className }) =>
                        className ? (
                          <code className="block text-xs font-mono bg-white/[0.06] rounded-md p-3 my-2 text-white/70 overflow-auto">
                            {children}
                          </code>
                        ) : (
                          <code className="text-xs font-mono bg-white/[0.06] rounded px-1 py-0.5 text-accent/80">
                            {children}
                          </code>
                        ),
                      pre: ({ children }) => <>{children}</>,
                      a: ({ href, children }) => (
                        <a
                          href={href}
                          className="text-accent hover:underline"
                          onClick={(e) => {
                            e.preventDefault();
                            if (href) {
                              import("../../wailsjs/runtime/runtime").then((r) =>
                                r.BrowserOpenURL(href),
                              );
                            }
                          }}
                        >
                          {children}
                        </a>
                      ),
                      strong: ({ children }) => (
                        <strong className="text-white/80 font-medium">{children}</strong>
                      ),
                      blockquote: ({ children }) => (
                        <blockquote className="border-l-2 border-white/[0.1] pl-3 my-2 text-white/50 italic">
                          {children}
                        </blockquote>
                      ),
                      hr: () => <hr className="border-white/[0.06] my-3" />,
                      table: ({ children }) => (
                        <table className="text-xs text-white/60 border-collapse my-2 w-full">
                          {children}
                        </table>
                      ),
                      th: ({ children }) => (
                        <th className="text-left text-white/70 font-medium px-2 py-1 border-b border-white/[0.08]">
                          {children}
                        </th>
                      ),
                      td: ({ children }) => (
                        <td className="px-2 py-1 border-b border-white/[0.04]">{children}</td>
                      ),
                    }}
                  >
                    {pr.body}
                  </Markdown>
                </div>
              </div>
            )}

            {/* Mergeable status */}
            {pr.mergeable && pr.mergeable !== "UNKNOWN" && (
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
          </div>
        </div>

        {/* PR list sidebar */}
        <div className="w-72 shrink-0 flex flex-col overflow-hidden glass-overlay m-4 ml-2 rounded-xl border border-white/[0.08]">
          <div className="px-4 py-3 border-b border-white/[0.08]">
            <h3 className="text-sm text-white font-medium">
              Open PRs
              <span className="text-tertiary ml-1.5">{prs.length}</span>
            </h3>
          </div>
          <div className="flex-1 overflow-auto">
            {prs.map((p) => {
              const isSelected = p.repo === pr.repo && p.number === pr.number;
              return (
                // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: PR list item
                <div
                  key={`${p.repo}-${p.number}`}
                  className={cn(
                    "px-4 py-3 cursor-pointer transition-colors border-l-2",
                    isSelected
                      ? "bg-white/[0.06] border-accent"
                      : "border-transparent hover:bg-white/[0.04]",
                  )}
                  onClick={() => onSelectPR(p)}
                >
                  <p
                    className={cn("text-sm truncate", isSelected ? "text-white" : "text-white/70")}
                  >
                    {p.title}
                  </p>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="text-[10px] text-muted-foreground">
                      {p.repo}#{p.number}
                    </span>
                    <span className="text-[10px] text-tertiary">{p.author}</span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
