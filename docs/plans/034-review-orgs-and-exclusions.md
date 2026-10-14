# Plan 034: Review only some orgs, and skip some repos

## Problem

The review worker reviews every PR its scopes find. Your own PRs come from every org and personal account you push to. You cannot keep the worker to work orgs, or keep it away from a repo where an automated review is not wanted.

## Goal

Two settings narrow both scopes, My PRs and Team PRs:

- **Orgs:** the owners whose PRs the worker reviews. Empty means every owner.
- **Excluded repos:** `owner/repo` names the worker never reviews. An exclusion wins over an org.

## Design

- `ReviewerConfig` gets `orgs` and `excludeRepos`, both lists of strings in `worker.json`.
- `fillDefaults` trims the entries and drops empty ones.
- `ReviewerConfig.problem()` checks the shape only: an org has no `/`, an excluded repo is `owner/repo`. A bad entry stops the review job, not the stand-up or focus time. A team repo that the filters skip is not an error.
- `ReviewerConfig.skipRepo(repo)` says why a repo is skipped, or returns "". It compares names in any case.
- `runReview` calls it first for each PR, before any GitHub call. A PR named with `--pr` is reviewed whatever the filters say, like the team scope.
- Settings > Worker shows both lists above My PRs and Team PRs. `RepoList` gains an org mode for the orgs list.

A personal account is an owner too. With orgs set, your personal repos are skipped unless your login is in the list. The help text says so.

## Not in scope

- Wildcards in repo names.
- Narrowing the GitHub searches by org. The filter runs on the results, which are small.

## Tests

- `skipRepo`: empty orgs, mixed case, an exclusion that beats an org.
- `fillDefaults` trims and drops empty entries. `problem()` rejects a bad shape.
- `merge()` turns `null` lists into `[]`.
- `RepoList` in org mode rejects `owner/repo`.
