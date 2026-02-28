export interface GitHubPR {
  repo: string;
  number: number;
  title: string;
  author: string;
  reviewDecision: string;
}

export interface SessionTab {
  id: string;
  title: string;
}
