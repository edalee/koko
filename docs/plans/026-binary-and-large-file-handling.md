# Plan 026: Binary File Detection & Large File Handling

**Status:** Implemented

## Summary

Two related Code Viewer improvements:
1. **Binary file detection** — detect binary files and show a clean message instead of garbled output
2. **Large file handling** — detect large files (10k+ lines) and gate loading behind a user confirmation

## Binary File Detection

### Backend (`git_service.go`)

Add `isBinaryData(data []byte) bool` helper — checks first 8KB for null bytes (same heuristic as git).

**`GetFileDiff`**: After reading the working tree file, check if binary. If binary:
- Set `IsBinary: true` on `FileDiffData`
- Clear content fields (no point sending binary data over IPC)
- Still count additions/deletions from the diff hunks if available

**`GetFileContent`**: After reading the file, check if binary. If binary:
- Set `IsBinary: true` on `FileContentData`
- Clear content field

### Backend (`github_service.go`)

**`FetchPRFileDiff`**: Check if the extracted hunks contain the `"Binary files"` marker that git produces for binary diffs. Also check fetched content for null bytes.

### Types (`types.go`)

Add to `FileDiffData`:
```go
IsBinary bool `json:"isBinary"`
```

Add to `FileContentData`:
```go
IsBinary bool `json:"isBinary"`
```

### Frontend (`CodeViewer.tsx`)

When `file?.isBinary` or `rawFile?.isBinary`:
- Show centered message: file icon + "Binary file — cannot display diff/content"
- Still show header (file name, path, +/- stats if available)
- Skip DiffView / shiki rendering entirely

## Large File Handling

### Backend

Add `LineCount int` field to `FileDiffData`. Count newlines in `NewContent` after fetching.

### Frontend (`CodeViewer.tsx`)

Threshold: 10,000 lines (combined old + new content).

When `file.lineCount > 10000` and file is not yet "force loaded":
- Show centered warning: "Large file (X lines) — rendering may be slow"
- "Load anyway" button
- `forceLoad` state tracks user override

When force loaded or under threshold, render DiffView normally.

## Files to Modify

| File | Change |
|------|--------|
| `types.go` | Add `IsBinary` to FileDiffData + FileContentData, `LineCount` to FileDiffData |
| `git_service.go` | Add `isBinaryData()` helper, binary check in GetFileDiff + GetFileContent |
| `github_service.go` | Binary check in FetchPRFileDiff |
| `frontend/src/components/CodeViewer.tsx` | Binary file message, large file warning + gated loading |

## Verification

1. Open a binary file (image, compiled binary) → see "Binary file" message
2. Open a diff containing a binary file → see message, no garbled output
3. Open a 10k+ line diff → see warning, click "Load anyway" to render
4. Normal files (<10k lines, non-binary) → unchanged behavior
