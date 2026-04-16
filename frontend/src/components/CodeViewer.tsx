import { DiffModeEnum, DiffView } from "@git-diff-view/react";
import "@git-diff-view/react/styles/diff-view-pure.css";
import { ArrowLeft, Columns2, FileMinus, FilePlus, FileText, Loader2, Rows2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import type { main } from "../../wailsjs/go/models";
import type { PRFileItem, ViewMode } from "../hooks/useCodeViewer";
import type { FileChange } from "../hooks/useFileChanges";

type AnimState = "closed" | "open" | "closing";

interface CodeViewerProps {
  open: boolean;
  file: main.FileDiffData | null;
  loading: boolean;
  viewMode: ViewMode;
  filePath: string;
  staged: boolean;
  fileChanges: FileChange[];
  onClose: () => void;
  onSetViewMode: (mode: ViewMode) => void;
  onFileSelect: (path: string, staged: boolean) => void;
  prFiles?: PRFileItem[];
  onPRFileSelect?: (path: string) => void;
}

function fileNameFromPath(path: string) {
  return path.split("/").pop() ?? path;
}

function fileDir(path: string) {
  const parts = path.split("/");
  if (parts.length <= 1) return "";
  return parts.slice(0, -1).join("/");
}

function statusColor(change: FileChange): string {
  if (change.staged) return "text-success";
  switch (change.status) {
    case "added":
      return "text-success";
    case "modified":
      return "text-warning";
    case "deleted":
      return "text-error";
    default:
      return "text-accent";
  }
}

function StatusIcon({ change }: { change: FileChange }) {
  const cls = `size-3.5 ${statusColor(change)}`;
  switch (change.status) {
    case "added":
      return <FilePlus className={cls} />;
    case "deleted":
      return <FileMinus className={cls} />;
    default:
      return <FileText className={cls} />;
  }
}

export default function CodeViewer({
  open,
  file,
  loading,
  viewMode,
  filePath,
  staged,
  fileChanges,
  onClose,
  onSetViewMode,
  onFileSelect,
  prFiles,
  onPRFileSelect,
}: CodeViewerProps) {
  const [state, setState] = useState<AnimState>("closed");

  useEffect(() => {
    if (open && state === "closed") {
      setState("open");
    } else if (!open && state === "open") {
      setState("closing");
    }
  }, [open, state]);

  const navigateFile = useCallback(
    (direction: -1 | 1) => {
      if (prFiles && prFiles.length > 0) {
        const idx = prFiles.findIndex((f) => f.path === filePath);
        const next = idx + direction;
        if (next >= 0 && next < prFiles.length) {
          onPRFileSelect?.(prFiles[next].path);
        }
      } else if (fileChanges.length > 0) {
        const idx = fileChanges.findIndex((f) => f.path === filePath && f.staged === staged);
        const next = idx + direction;
        if (next >= 0 && next < fileChanges.length) {
          onFileSelect(fileChanges[next].path, fileChanges[next].staged);
        }
      }
    },
    [prFiles, fileChanges, filePath, staged, onPRFileSelect, onFileSelect],
  );

  useEffect(() => {
    if (state !== "open") return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
      if (e.key === "[" || e.key === "ArrowUp") {
        e.preventDefault();
        navigateFile(-1);
      }
      if (e.key === "]" || e.key === "ArrowDown") {
        e.preventDefault();
        navigateFile(1);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state, onClose, navigateFile]);

  function handleAnimationEnd() {
    if (state === "closing") setState("closed");
  }

  if (state === "closed") return null;

  const diffData = file
    ? {
        oldFile: {
          fileName: file.oldFileName ?? undefined,
          fileLang: file.language || undefined,
          content: file.oldContent ?? "",
        },
        newFile: {
          fileName: file.newFileName ?? undefined,
          fileLang: file.language || undefined,
          content: file.newContent ?? "",
        },
        hunks: file.hunks ? [file.hunks] : [],
      }
    : undefined;

  return (
    <div
      className={`fixed inset-0 z-[60] flex flex-col bg-base ${
        state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
      }`}
      onAnimationEnd={handleAnimationEnd}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-4 py-2.5 border-b border-white/[0.08] shrink-0 bg-white/[0.02]"
        style={{ "--wails-draggable": "drag" } as React.CSSProperties}
      >
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-14 shrink-0" />
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-md text-muted-foreground hover:text-white hover:bg-white/10 transition-colors shrink-0"
            style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
            title="Back (Escape)"
          >
            <ArrowLeft className="size-4" />
          </button>

          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-foreground text-sm font-medium truncate">
                {fileNameFromPath(filePath)}
              </span>
              {staged && (
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-success/10 text-success font-medium shrink-0">
                  Staged
                </span>
              )}
            </div>
            {fileDir(filePath) && (
              <span className="text-[10px] text-tertiary truncate block">{fileDir(filePath)}</span>
            )}
          </div>
        </div>

        <div
          className="flex items-center gap-2 shrink-0"
          style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
        >
          {((prFiles && prFiles.length > 1) || fileChanges.length > 1) && (
            <span className="text-[10px] text-tertiary mr-2">
              <kbd className="px-1 py-0.5 rounded bg-white/[0.06] border border-white/[0.08] font-mono">
                [
              </kbd>{" "}
              <kbd className="px-1 py-0.5 rounded bg-white/[0.06] border border-white/[0.08] font-mono">
                ]
              </kbd>{" "}
              navigate
            </span>
          )}
          {file && (file.additions > 0 || file.deletions > 0) && (
            <div className="flex items-center gap-1.5 text-xs font-mono mr-2">
              {file.additions > 0 && <span className="text-success">+{file.additions}</span>}
              {file.deletions > 0 && <span className="text-error">-{file.deletions}</span>}
            </div>
          )}

          <div className="flex items-center rounded-md border border-white/[0.08] overflow-hidden">
            <button
              type="button"
              onClick={() => onSetViewMode("split")}
              className={`p-1.5 transition-colors ${
                viewMode === "split"
                  ? "bg-white/[0.1] text-white"
                  : "text-muted-foreground hover:text-white hover:bg-white/[0.04]"
              }`}
              title="Split view"
            >
              <Columns2 className="size-3.5" />
            </button>
            <button
              type="button"
              onClick={() => onSetViewMode("unified")}
              className={`p-1.5 transition-colors ${
                viewMode === "unified"
                  ? "bg-white/[0.1] text-white"
                  : "text-muted-foreground hover:text-white hover:bg-white/[0.04]"
              }`}
              title="Unified view"
            >
              <Rows2 className="size-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Main content: diff + file list sidebar */}
      <div className="flex-1 flex min-h-0">
        {/* Diff content */}
        <div className="flex-1 overflow-auto code-viewer-wrapper">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <Loader2 className="size-5 text-muted-foreground animate-spin" />
            </div>
          ) : file && diffData ? (
            <DiffView
              data={diffData}
              diffViewMode={viewMode === "split" ? DiffModeEnum.Split : DiffModeEnum.Unified}
              diffViewTheme="dark"
              diffViewHighlight
              diffViewWrap
              diffViewFontSize={13}
            />
          ) : (
            <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
              No diff data available
            </div>
          )}
        </div>

        {/* File list sidebar */}
        {prFiles && prFiles.length > 0 ? (
          <div className="w-56 shrink-0 border-l border-white/[0.08] bg-white/[0.02] overflow-y-auto">
            <div className="px-3 py-2 text-[10px] text-tertiary uppercase tracking-wider">
              PR Files ({prFiles.length})
            </div>
            {prFiles.map((pf) => {
              const isActive = pf.path === filePath;
              return (
                <button
                  key={pf.path}
                  type="button"
                  onClick={() => onPRFileSelect?.(pf.path)}
                  className={`flex items-center gap-2 w-full px-3 py-1.5 text-left transition-colors ${
                    isActive
                      ? "bg-white/[0.08] text-white"
                      : "text-muted-foreground hover:text-white hover:bg-white/[0.04]"
                  }`}
                  title={pf.path}
                >
                  <FileText className="size-3.5 shrink-0" />
                  <span className="text-[11px] truncate flex-1">{fileNameFromPath(pf.path)}</span>
                  <span className="text-[9px] text-success shrink-0">+{pf.additions}</span>
                  <span className="text-[9px] text-error shrink-0">-{pf.deletions}</span>
                </button>
              );
            })}
          </div>
        ) : fileChanges.length > 0 ? (
          <div className="w-56 shrink-0 border-l border-white/[0.08] bg-white/[0.02] overflow-y-auto">
            <div className="px-3 py-2 text-[10px] text-tertiary uppercase tracking-wider">
              Changed Files ({fileChanges.length})
            </div>
            {fileChanges.map((change) => {
              const isActive = change.path === filePath && change.staged === staged;
              return (
                <button
                  key={`${change.path}-${change.staged}`}
                  type="button"
                  onClick={() => onFileSelect(change.path, change.staged)}
                  className={`flex items-center gap-2 w-full px-3 py-1.5 text-left transition-colors ${
                    isActive
                      ? "bg-white/[0.08] text-white"
                      : "text-muted-foreground hover:text-white hover:bg-white/[0.04]"
                  }`}
                  title={`${change.path}${change.staged ? " (staged)" : ""}`}
                >
                  <StatusIcon change={change} />
                  <span className="text-[11px] truncate">{fileNameFromPath(change.path)}</span>
                  {change.staged && (
                    <span className="text-[9px] px-1 rounded bg-success/10 text-success ml-auto shrink-0">
                      S
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        ) : null}
      </div>
    </div>
  );
}
