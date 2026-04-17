# Plan 025: Raw File Viewer

**Status:** Implemented

## Summary

View unmodified files with syntax highlighting, not just diffs. Uses shiki (already installed as transitive dep) to render files with the `github-dark` theme. Includes line numbers via CSS counters.

## Entry Point

- **Cmd+click** a file in the File Changes sidebar opens it as a raw file view (vs regular click which opens the diff)

## Changes

### Hook (`useCodeViewer.ts`)
- Added `rawFile` state (`FileContentData | null`) alongside existing `file` (diff data)
- Added `openFile(dir, path)` method — calls `GetFileContent`, sets `rawFile`
- All other methods (`openDiff`, `openPRDiff`, `close`) clear `rawFile`

### Component (`CodeViewer.tsx`)
- `isRawMode` — true when `rawFile` is set and `file` is null
- **Shiki highlighting** — `codeToHtml()` with `github-dark` theme, async with cancellation
- **Fallback** — plain `<pre>` if shiki fails
- **Header** — shows language + line count instead of +/- stats; hides split/unified toggle
- **Language mapping** — `LANG_MAP` translates common aliases (golang→go, ts→typescript, etc.)

### Sidebar (`RightSidebar.tsx`)
- Added `onFileView` prop
- File click handler: `metaKey` → `onFileView`, regular → `onFileClick`

### CSS (`globals.css`)
- `.raw-file-viewer` styles: transparent background for shiki `<pre>`, CSS counter-based line numbers on `.line` spans

### Wiring (`App.tsx`)
- Passes `rawFile` + `onFileView` through to CodeViewer and RightSidebar

## No Backend Changes
`GetFileContent` already existed in `git_service.go`.
