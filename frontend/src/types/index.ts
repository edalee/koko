export interface GitHubPR {
  repo: string;
  number: number;
  title: string;
  author: string;
  reviewDecision: string;
  url: string;
}

export interface SessionTab {
  id: string;
  name: string;
  directory: string;
  createdAt: number;
  connected: boolean;
}

export interface SessionHistoryEntry {
  name: string;
  directory: string;
  createdAt: number;
  closedAt: number;
  lastMessage?: string;
}
