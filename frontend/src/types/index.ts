export interface PRCheck {
  name: string;
  status: string;
  conclusion: string;
}

export interface GitHubPR {
  repo: string;
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
  isDraft: boolean;
  labels: string[];
  checks: PRCheck[];
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
}

export interface SessionHistoryEntry {
  slug: string;
  name: string;
  directory: string;
  createdAt: number;
  closedAt: number;
  lastMessage?: string;
  claudeSessionId?: string;
}
