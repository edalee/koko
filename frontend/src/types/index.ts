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
