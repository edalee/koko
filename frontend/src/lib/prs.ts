import type { GitHubPR } from "../types";

// isHiddenPR checks the "owner/repo#n" key, and the "repo#n" key that hid a
// PR before 0.5.9, so PRs hidden then stay hidden.
export function isHiddenPR(hidden: Set<string>, pr: Pick<GitHubPR, "repo" | "number">): boolean {
  const short = pr.repo.slice(pr.repo.lastIndexOf("/") + 1);
  return hidden.has(`${pr.repo}#${pr.number}`) || hidden.has(`${short}#${pr.number}`);
}

export const PR_SECTIONS: { key: GitHubPR["section"]; name: string }[] = [
  { key: "mine", name: "My PRs" },
  { key: "team", name: "Team PRs" },
];
