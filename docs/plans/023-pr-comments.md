# Plan 023: PR Comments — Read & Reply

**Status:** Implemented

## Summary

Added ability to view and reply to PR comments directly in the PR detail overlay. Supports both general discussion comments (issue comments) and inline code review comments (threaded by file).

## Changes

### Go Backend
- **`types.go`** — Added `PRComment`, `PRCommentThread`, `PRCommentsData` types
- **`github_service.go`** — Added three methods:
  - `FetchPRComments(repo, number)` — Fetches both review comments (threaded, grouped by file) and issue comments via GitHub REST API
  - `ReplyToReviewComment(repo, number, commentID, body)` — Replies to a review comment thread
  - `AddIssueComment(repo, number, body)` — Adds a general discussion comment

### Frontend
- **`types/index.ts`** — Added `PRComment`, `PRCommentThread`, `PRCommentsData` interfaces
- **`PRDetailOverlay.tsx`** — Two new collapsible sections between Reviews and File Changes:
  - **Discussion** — Flat list of issue comments with "Add comment" button
  - **Code Comments** — Review threads grouped by file path, with diff hunk context, threaded replies, and reply action
  - Both sections support replying with Cmd+Enter shortcut
  - Bot comments show a "Bot" badge
  - Sections hidden when no comments exist
