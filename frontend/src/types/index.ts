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
  title: string;
  createdAt: number;
}
