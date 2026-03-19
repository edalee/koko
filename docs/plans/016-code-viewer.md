# Plan 016: Code Viewer & Diff Experience

## Goal
Add a world-class code viewing experience to Koko: click any changed file in the right sidebar to see its diff with syntax highlighting, or view the raw file content. GitHub-style UI, dark theme, split/unified toggle.

## Library Choice
**`@git-diff-view/react`** — GitHub-faithful diff rendering, split + unified views, Shiki syntax highlighting, dark theme via CSS variables, Web Worker support for large diffs. Pure CSS option avoids Tailwind conflicts.

Packages:
- `@git-diff-view/react` — React DiffView component
- `@git-diff-view/shiki` — Shiki-based syntax highlighting (VS Code quality)

## Architecture

### Backend (Go)

New methods on `GitService`:

```
GetFileDiff(dir, path string, staged bool) → FileDiffData
GetFileContent(dir, path string) → FileContentData
```

**`GetFileDiff`** returns:
```go
type FileDiffData struct {
    OldFileName string `json:"oldFileName"`
    OldContent  string `json:"oldContent"`
    NewFileName string `json:"newFileName"`
    NewContent  string `json:"newContent"`
    Hunks       string `json:"hunks"`      // raw unified diff hunks
    Language    string `json:"language"`    // inferred from extension
}
```

Implementation:
- Staged: `git diff --cached -- <path>` for hunks, `git show HEAD:<path>` for old content, read working tree for new
- Unstaged: `git diff -- <path>` for hunks, `git show :<path>` (index) for old content, read working tree for new
- New files: old content empty, full file as new content
- Language: map file extension → language name (`.ts` → `typescript`, `.go` → `go`, etc.)

**`GetFileContent`** returns raw file content + language for viewing unmodified files.

### Frontend

#### Component: `CodeViewer.tsx`
A full-screen overlay (like GitHub/Slack overlays) that opens when clicking a file in the right sidebar.

```
┌─────────────────────────────────────────────────┐
│ ← Back    path/to/file.tsx    Split | Unified   │
│─────────────────────────────────────────────────│
│                                                  │
│  @git-diff-view/react DiffView component         │
│  (GitHub-style diff with syntax highlighting)    │
│                                                  │
│                                                  │
└─────────────────────────────────────────────────┘
```

**Header bar:**
- Back button (or Escape to close)
- File path with status badge (A/M/D, staged indicator)
- Split / Unified toggle
- Line wrap toggle
- File stats: `+12 -3`

**Diff area:**
- `@git-diff-view/react` `DiffView` component
- Dark theme matching Koko's palette (custom CSS variables)
- Shiki syntax highlighting
- Collapsible unchanged sections
- Line numbers

**Raw file view:**
- Toggle between diff and raw file content
- Same syntax highlighting via Shiki

#### Integration with Right Sidebar

Current `RightSidebar.tsx` shows file changes as a list. Add:
- Click handler on each file → opens CodeViewer overlay
- Visual affordance (cursor pointer, hover effect already exists)

#### Hook: `useCodeViewer.ts`
```typescript
const { isOpen, file, openDiff, openFile, close, viewMode, setViewMode } = useCodeViewer();
```

State:
- `isOpen: boolean`
- `file: FileDiffData | null`
- `viewMode: 'split' | 'unified'`
- `loading: boolean`

## Implementation Steps

### Step 1: Backend — diff & file content endpoints
- Add `GetFileDiff()` and `GetFileContent()` to `git_service.go`
- Map file extensions to language names
- Handle edge cases: new files, deleted files, binary files, renamed files

### Step 2: Install packages
```bash
cd frontend && npm install @git-diff-view/react @git-diff-view/shiki
```

### Step 3: CodeViewer component
- Build `CodeViewer.tsx` as an overlay (reuse `OverlayPage` pattern)
- Import `@git-diff-view/react/styles/diff-view-pure.css` (no Tailwind conflicts)
- Configure dark theme CSS variables to match Koko palette
- Split/unified toggle via `DiffModeEnum.Split` / `DiffModeEnum.Unified`
- Shiki highlighter loaded async, cached globally

### Step 4: Wire up right sidebar
- Add `onClick` to file change items in `RightSidebar.tsx`
- Call `openDiff(directory, path, staged)` → fetches diff data → opens overlay

### Step 5: Polish
- Keyboard navigation: Escape to close, `[` / `]` for prev/next file
- Custom CSS variables for Koko dark theme colors
- Loading skeleton while diff loads
- Handle large files gracefully (Web Worker via DiffFile bundle API)
- Binary file detection (show "Binary file" message)

## Dark Theme CSS Variables
```css
[data-component="git-diff-view"][data-theme="dark"] {
  --diff-add-line-bg: rgba(31, 242, 171, 0.08);       /* accent green */
  --diff-del-line-bg: rgba(244, 71, 71, 0.08);        /* red */
  --diff-add-content-highlight-bg: rgba(31, 242, 171, 0.2);
  --diff-del-content-highlight-bg: rgba(244, 71, 71, 0.2);
  --diff-hunk-line-bg: rgba(255, 255, 255, 0.03);
  --diff-add-line-num-bg: rgba(31, 242, 171, 0.12);
  --diff-del-line-num-bg: rgba(244, 71, 71, 0.12);
}
```

## File Extension → Language Map
```go
var langMap = map[string]string{
    ".ts": "typescript", ".tsx": "tsx", ".js": "javascript", ".jsx": "jsx",
    ".go": "go", ".py": "python", ".rs": "rust", ".java": "java",
    ".kt": "kotlin", ".rb": "ruby", ".sh": "bash", ".bash": "bash",
    ".css": "css", ".scss": "scss", ".html": "html", ".json": "json",
    ".yaml": "yaml", ".yml": "yaml", ".toml": "toml", ".md": "markdown",
    ".sql": "sql", ".proto": "protobuf", ".dockerfile": "dockerfile",
    ".xml": "xml", ".svg": "xml", ".graphql": "graphql",
}
```

## Not in Scope (Future)
- Inline comments / code review
- Edit & stage from diff view
- Multi-file diff navigation (PR-style "Files changed" view)
- Blame view
