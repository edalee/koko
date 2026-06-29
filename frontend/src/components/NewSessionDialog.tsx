import {
  AlertTriangle,
  Clock,
  Folder,
  FolderOpen,
  GitBranch,
  Loader2,
  Plus,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { PickDirectory } from "../../wailsjs/go/main/App";
import { GetSessions } from "../../wailsjs/go/main/ConfigService";
import { CreateWorktree, GetBranchName } from "../../wailsjs/go/main/GitService";
import type { SessionHistoryEntry } from "../types";

function timeAgo(ts: number): string {
  const seconds = Math.floor((Date.now() - ts) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function shortenPath(path: string): string {
  const home = path.match(/^\/Users\/[^/]+/)?.[0];
  if (home) return path.replace(home, "~");
  return path;
}

interface NewSessionDialogProps {
  open: boolean;
  onClose: () => void;
  onCreate: (name: string, directory: string, worktreePath?: string) => void;
  history: SessionHistoryEntry[];
  activeDirs: string[];
}

type AnimState = "closed" | "open" | "closing";

function dirBasename(path: string): string {
  const parts = path.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || path;
}

function dirParent(path: string): string {
  const stripped = path.replace(/\/+$/, "");
  const idx = stripped.lastIndexOf("/");
  return idx === -1 ? stripped : stripped.slice(0, idx);
}

function branchSlug(branch: string): string {
  return branch.replace(/[^a-zA-Z0-9-_]/g, "-").toLowerCase();
}

function randomSlug(): string {
  return Math.random().toString(36).slice(2, 8);
}

export default function NewSessionDialog({
  open,
  onClose,
  onCreate,
  history,
  activeDirs,
}: NewSessionDialogProps) {
  const [state, setState] = useState<AnimState>("closed");
  const [name, setName] = useState("");
  const [directory, setDirectory] = useState("");
  const [recentDirs, setRecentDirs] = useState<string[]>([]);

  // Worktree state
  const [useWorktree, setUseWorktree] = useState(false);
  const [worktreeBranch, setWorktreeBranch] = useState("");
  const [worktreePath, setWorktreePath] = useState("");
  const [createNewBranch, setCreateNewBranch] = useState(true);
  const [currentBranch, setCurrentBranch] = useState<string | null>(null);
  const [creatingWorktree, setCreatingWorktree] = useState(false);
  const [worktreeError, setWorktreeError] = useState<string | null>(null);

  const dirCollides = useMemo(
    () => directory !== "" && activeDirs.includes(directory),
    [directory, activeDirs],
  );

  // Load recent dirs from Go backend
  useEffect(() => {
    if (open) {
      GetSessions()
        .then((s) => setRecentDirs(s.recentDirs || []))
        .catch(() => {});
    }
  }, [open]);

  // When a directory is picked, look up its current branch and seed worktree
  // defaults. If the dir collides with another active session, auto-enable
  // the worktree toggle.
  useEffect(() => {
    if (!directory) {
      setCurrentBranch(null);
      setWorktreeBranch("");
      setWorktreePath("");
      setUseWorktree(false);
      setWorktreeError(null);
      return;
    }
    setUseWorktree(activeDirs.includes(directory));
    GetBranchName(directory)
      .then((branch) => {
        setCurrentBranch(branch);
        const base = branchSlug(branch || "wt");
        const suffix = randomSlug();
        const newBranch = `${base}-${suffix}`;
        setWorktreeBranch(newBranch);
        setWorktreePath(`${dirParent(directory)}/${dirBasename(directory)}-${suffix}`);
      })
      .catch(() => {
        setCurrentBranch(null);
        const suffix = randomSlug();
        setWorktreeBranch(`wt-${suffix}`);
        setWorktreePath(`${dirParent(directory)}/${dirBasename(directory)}-${suffix}`);
      });
  }, [directory, activeDirs]);

  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open && state === "closed") {
      setState("open");
      setName("");
      setDirectory("");
      setTimeout(() => nameRef.current?.focus(), 50);
    } else if (!open && state === "open") {
      setState("closing");
    }
  }, [open, state]);

  function handleAnimationEnd() {
    if (state === "closing") {
      setState("closed");
    }
  }

  const handleBrowse = useCallback(async () => {
    const dir = await PickDirectory();
    if (dir) setDirectory(dir);
  }, []);

  const handleCreate = useCallback(async () => {
    if (!directory) return;
    setWorktreeError(null);

    let finalDir = directory;
    if (useWorktree) {
      if (!worktreeBranch.trim() || !worktreePath.trim()) {
        setWorktreeError("Branch and worktree path are required");
        return;
      }
      setCreatingWorktree(true);
      try {
        const wt = await CreateWorktree(
          directory,
          worktreePath.trim(),
          worktreeBranch.trim(),
          createNewBranch,
        );
        finalDir = wt.path || worktreePath.trim();
      } catch (err) {
        setCreatingWorktree(false);
        setWorktreeError(
          err instanceof Error ? err.message : String(err ?? "Failed to create worktree"),
        );
        return;
      }
      setCreatingWorktree(false);
    }

    const sessionName =
      name.trim() || (useWorktree ? branchSlug(worktreeBranch) : dirBasename(finalDir));
    onCreate(sessionName, finalDir, useWorktree ? finalDir : undefined);
  }, [name, directory, useWorktree, worktreeBranch, worktreePath, createNewBranch, onCreate]);

  useEffect(() => {
    if (state !== "open") return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        onClose();
      } else if (e.key === "Enter" && directory && !creatingWorktree) {
        handleCreate();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state, directory, creatingWorktree, onClose, handleCreate]);

  if (state === "closed") return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop */}
      <div
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm ${
          state === "closing" ? "animate-backdrop-out" : "animate-backdrop-in"
        }`}
        onClick={onClose}
      />

      <div
        className={`relative w-[480px] flex flex-col rounded-xl border shadow-2xl glass-overlay inset-highlight ${
          state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
        }`}
        style={{
          backgroundColor: "rgba(255, 255, 255, 0.08)",
          borderColor: "var(--color-glass-border)",
        }}
        onAnimationEnd={handleAnimationEnd}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/[0.08]">
          <div className="flex items-center gap-2.5">
            <span className="text-accent">
              <Plus className="size-4" />
            </span>
            <h2 className="text-white text-sm font-medium">New Session</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-md text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Content */}
        <div className="p-5 space-y-5">
          {/* Name input */}
          <div className="space-y-2">
            <label
              className="text-xs font-medium text-muted-foreground uppercase tracking-wider"
              htmlFor="session-name"
            >
              Session Name
            </label>
            <input
              ref={nameRef}
              id="session-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={directory ? dirBasename(directory) : "Session name"}
              className="w-full px-3 py-2.5 text-sm bg-white/[0.04] border border-white/[0.06] rounded-lg text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
            />
          </div>

          {/* Directory picker */}
          <div className="space-y-2">
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
              Project Directory
            </span>

            {directory ? (
              <button
                type="button"
                onClick={handleBrowse}
                className="w-full flex items-center gap-3 px-3 py-2.5 text-sm bg-accent/10 border border-accent/30 rounded-md text-white hover:bg-accent/15 transition-colors text-left"
              >
                <FolderOpen className="size-4 text-accent shrink-0" />
                <span className="truncate">{shortenPath(directory)}</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={handleBrowse}
                className="w-full flex items-center justify-center gap-2 px-3 py-3 text-sm bg-white/5 border border-border border-dashed rounded-md text-muted-foreground hover:text-white hover:border-accent/30 hover:bg-white/[0.07] transition-colors"
              >
                <Folder className="size-4" />
                <span>Browse...</span>
              </button>
            )}

            {/* Worktree section — shown once a directory is selected */}
            {directory && (
              <div className="mt-3 rounded-md border border-white/[0.06] bg-white/[0.02] overflow-hidden">
                {dirCollides && (
                  <div className="flex items-start gap-2 px-3 py-2 bg-warning/8 border-b border-warning/15 text-[11px] text-warning">
                    <AlertTriangle className="size-3.5 shrink-0 mt-0.5" />
                    <span>
                      Another session is using this directory. Sessions on the same directory can
                      pollute each other&apos;s files, builds, and git state.
                    </span>
                  </div>
                )}
                <label className="flex items-center gap-2 px-3 py-2 cursor-pointer select-none hover:bg-white/[0.03] transition-colors">
                  <input
                    type="checkbox"
                    checked={useWorktree}
                    onChange={(e) => setUseWorktree(e.target.checked)}
                    className="accent-accent"
                  />
                  <GitBranch className="size-3.5 text-muted-foreground" />
                  <span className="text-[12px] text-white/90">Create as a git worktree</span>
                  {currentBranch && (
                    <span className="ml-auto text-[10px] text-tertiary">from {currentBranch}</span>
                  )}
                </label>

                {useWorktree && (
                  <div className="px-3 pb-3 space-y-2.5 border-t border-white/[0.06] pt-2.5">
                    <div className="space-y-1">
                      <label
                        className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider"
                        htmlFor="worktree-branch"
                      >
                        Branch
                      </label>
                      <input
                        id="worktree-branch"
                        value={worktreeBranch}
                        onChange={(e) => setWorktreeBranch(e.target.value)}
                        className="w-full px-2.5 py-1.5 text-[12px] font-mono bg-white/[0.04] border border-white/[0.06] rounded text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
                      />
                      <label className="flex items-center gap-2 text-[11px] text-muted-foreground cursor-pointer select-none">
                        <input
                          type="checkbox"
                          checked={createNewBranch}
                          onChange={(e) => setCreateNewBranch(e.target.checked)}
                          className="accent-accent"
                        />
                        Create as new branch
                        {!createNewBranch && (
                          <span className="text-[10px] text-tertiary">(checkout existing)</span>
                        )}
                      </label>
                    </div>
                    <div className="space-y-1">
                      <label
                        className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider"
                        htmlFor="worktree-path"
                      >
                        Worktree Path
                      </label>
                      <input
                        id="worktree-path"
                        value={worktreePath}
                        onChange={(e) => setWorktreePath(e.target.value)}
                        className="w-full px-2.5 py-1.5 text-[11px] font-mono bg-white/[0.04] border border-white/[0.06] rounded text-white/80 placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
                      />
                    </div>
                    {worktreeError && (
                      <div className="flex items-start gap-2 px-2 py-1.5 rounded bg-error/10 border border-error/20 text-[11px] text-error">
                        <AlertTriangle className="size-3 shrink-0 mt-0.5" />
                        <span className="break-words">{worktreeError}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Session history / Recent directories */}
            {!directory && (
              <div className="space-y-1 pt-1">
                {history.length > 0 && (
                  <>
                    <span className="text-xs text-muted-foreground/60 px-1 flex items-center gap-1.5">
                      <Clock className="size-3" />
                      Recent Sessions
                    </span>
                    {history
                      .filter((h) => !activeDirs.includes(h.directory))
                      .slice(0, 8)
                      .map((entry, idx) => (
                        <button
                          type="button"
                          key={`${entry.directory}-${entry.closedAt}-${idx}`}
                          onClick={() => {
                            onCreate(entry.name, entry.directory);
                          }}
                          className="w-full flex items-center gap-2.5 px-3 py-2 text-sm rounded-md text-muted-foreground hover:text-white hover:bg-white/5 transition-colors text-left"
                        >
                          <Folder className="size-3.5 shrink-0" />
                          <div className="flex-1 min-w-0">
                            <span className="text-white/80 block truncate">{entry.name}</span>
                            <span className="text-[10px] text-tertiary block truncate">
                              {shortenPath(entry.directory)}
                            </span>
                            {entry.lastMessage && (
                              <span className="text-[10px] text-muted-foreground/50 block truncate mt-0.5 italic">
                                {entry.lastMessage}
                              </span>
                            )}
                          </div>
                          <span className="text-[10px] text-tertiary shrink-0">
                            {timeAgo(entry.closedAt)}
                          </span>
                        </button>
                      ))}
                  </>
                )}
                {history.length === 0 && recentDirs.length > 0 && (
                  <>
                    <span className="text-xs text-muted-foreground/60 px-1">Recent</span>
                    {recentDirs.map((dir) => (
                      <button
                        type="button"
                        key={dir}
                        onClick={() => setDirectory(dir)}
                        className="w-full flex items-center gap-2.5 px-3 py-2 text-sm rounded-md text-muted-foreground hover:text-white hover:bg-white/5 transition-colors text-left"
                      >
                        <Folder className="size-3.5 shrink-0" />
                        <span className="truncate">{shortenPath(dir)}</span>
                      </button>
                    ))}
                  </>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="px-5 py-4 border-t border-white/[0.08] flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm rounded-md text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleCreate}
            disabled={!directory || creatingWorktree}
            className="px-4 py-2 text-sm rounded-md font-medium transition-all disabled:opacity-30 disabled:cursor-not-allowed text-white relative overflow-hidden border-2 border-transparent flex items-center gap-2"
            style={{
              background: directory
                ? "linear-gradient(rgba(255,255,255,0.08), rgba(255,255,255,0.08)) padding-box, linear-gradient(to right, #1FF2AB, #24A965) border-box"
                : undefined,
            }}
          >
            {creatingWorktree && <Loader2 className="size-3.5 animate-spin" />}
            {creatingWorktree
              ? "Creating worktree..."
              : useWorktree
                ? "Create Worktree + Session"
                : "Create Session"}
          </button>
        </div>
      </div>
    </div>
  );
}
