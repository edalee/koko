# Plan 032: PR panel sources like the review worker's

## Problem

The PR panel (the PR overlay and the PR count in the sidebar) works badly outside one team at Epidemic Sound:

- With no repos set, it lists a built-in set of 16 DRM repos, whatever team you are in.
- It lists open PRs per repo only. Your own PRs and PRs waiting for your review in other repos are missing.
- Every action (approve, merge, files, diff, reviews, commits, comments) prefixes `epidemicsound/`, so a PR in any other org breaks.
- It runs one `gh pr list` per repo, one after another, every 60 seconds.
- A failed repo is skipped without a word, so an empty list looks the same as no PRs.

## Goal

The panel shows the same PRs the review worker would look at, set the same way: your PRs, and your team's. It works for any org, and says when a source fails.

## Settings

Settings > GitHub gets the review worker's two scopes, stored in `config.json` as `prPanel`:

| Setting | Default | Meaning |
|---|---|---|
| `mine.enabled`, `mine.maxAgeDays` | on, 30 | Your open PRs, drafts included, opened in the last N days |
| `team.enabled`, `team.maxAgeDays` | on, 14 | PRs that ask for your review, and every open PR in `team.repos`, opened in the last N days |
| `team.team` | none | `org/team-slug`. When set, team PRs are only those opened by the team's members |
| `team.repos` | none | `owner/repo` names. A bare name means the team's org |

Bots' PRs and drafts are left out of team PRs, as in the worker. A button copies the review worker's scopes, so both can match with one click.

## Fetching

One GraphQL search per source, with every field the panel shows, in place of one `gh pr list` per repo:

- mine: `is:pr is:open archived:false author:@me created:>=<date>`
- review requests: `is:pr is:open archived:false draft:false review-requested:@me created:>=<date>`
- repos: the same with `repo:a repo:b ...`, ten repos per query, so the query stays under GitHub's length limit

The team's members are cached for an hour. A PR found by more than one source shows once, in the first section.

`FetchPRs` returns the PRs and an error per failed source. The panel shows the errors above the list.

## Full repo names

`GitHubPR.repo` is `owner/repo`. Every action takes it as it is. A bare name is an error, not `epidemicsound/<name>`. Hidden PR keys become `owner/repo#n`. The panel also checks the old `repo#n` key, so PRs hidden before stay hidden.

## Sections

The PR list groups PRs under "My PRs" and "Team PRs", with a count each.

## Migration

A non-empty `githubRepos` from before becomes `prPanel.team.repos`. Bare names there meant `epidemicsound/<name>`, so they become that. An empty one meant the DRM default list, which goes: you now see your own PRs and your review requests instead.

## Files

`github_service.go` (`FetchPRs`, the search queries), `pr_sources.go` (new, query building and filters), `config_service.go` (`prPanel`), `types.go`, `frontend/src/components/SettingsPanel.tsx`, `frontend/src/components/PRDetailOverlay.tsx`, `frontend/src/hooks/useGitHub.ts`.
