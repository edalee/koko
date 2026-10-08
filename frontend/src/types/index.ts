export interface PRCheck {
  name: string;
  status: string;
  conclusion: string;
}

export interface GitHubPR {
  repo: string; // "owner/repo"
  number: number;
  title: string;
  author: string;
  reviewDecision: string;
  url: string;
  body: string;
  additions: number;
  deletions: number;
  changedFiles: number;
  headRef: string;
  baseRef: string;
  createdAt: string;
  updatedAt: string;
  mergeable: string;
  mergeStateStatus: string;
  isDraft: boolean;
  labels: string[];
  assignees: string[];
  checks: PRCheck[];
  section: "mine" | "team";
}

export interface PRFile {
  path: string;
  additions: number;
  deletions: number;
}

export interface PRReview {
  author: string;
  state: string;
  submittedAt: string;
  body: string;
}

export interface PRCommit {
  sha: string;
  message: string;
  author: string;
  date: string;
}

export interface WorkflowRun {
  id: number;
  name: string;
  status: string;
  conclusion: string;
  event: string;
  createdAt: string;
  updatedAt: string;
  htmlUrl: string;
}

export interface BranchCI {
  branch: string;
  repo: string;
  runs: WorkflowRun[];
}

export interface PRComment {
  id: number;
  inReplyToId: number;
  author: string;
  authorType: string;
  body: string;
  createdAt: string;
  htmlUrl: string;
  path: string;
  line: number;
  originalLine: number;
  side: string;
  diffHunk: string;
  subjectType: string;
}

export interface PRCommentThread {
  root: PRComment;
  replies: PRComment[];
}

export interface PRCommentsData {
  reviewThreads: PRCommentThread[];
  issueComments: PRComment[];
}

export interface SessionTab {
  id: string;
  slug: string;
  name: string;
  directory: string;
  createdAt: number;
  connected: boolean;
  claudeSessionId?: string;
  lastMsg?: string;
  // The session's worktree: one Koko created, or one opened from the
  // Worktrees module. Used on session close to offer to clean it up.
  worktreePath?: string;
  // True only when Koko created the worktree. Settings removes only these in
  // bulk, never a worktree the user made by hand.
  worktreeCreated?: boolean;
  // Why the last reconnect failed, shown on the reconnect card. Not persisted.
  reconnectError?: string;
}

export interface SessionHistoryEntry {
  slug: string;
  name: string;
  directory: string;
  createdAt: number;
  closedAt: number;
  lastMessage?: string;
  claudeSessionId?: string;
  // A worktree Koko created for the session and the user kept at close.
  // Settings > General can remove these later (plan 028 step 7b). The folder
  // may since have gone, if the user deleted it by hand.
  worktreePath?: string;
}
