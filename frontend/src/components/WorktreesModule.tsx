import {
  AlertTriangle,
  CheckCircle2,
  Folder,
  GitBranch,
  Loader2,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react";
import { useState } from "react";
import type { main } from "../../wailsjs/go/models";
import { useWorktrees } from "../hooks/useWorktrees";

function shortenPath(path: string): string {
  const home = path.match(/^\/Users\/[^/]+/)?.[0];
  if (home) return path.replace(home, "~");
  return path;
}

function dirBasename(path: string): string {
  const parts = path.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || path;
}

function dirParent(path: string): string {
  const stripped = path.replace(/\/+$/, "");
  const idx = stripped.lastIndexOf("/");
  return idx === -1 ? stripped : stripped.slice(0, idx);
}

function randomSlug(): string {
  return Math.random().toString(36).slice(2, 8);
}

interface WorktreesModuleProps {
  activeDirectory: string | null;
  onOpenSession: (name: string, directory: string) => void;
}

export default function WorktreesModule({ activeDirectory, onOpenSession }: WorktreesModuleProps) {
  const { worktrees, loading, error, refresh, create, remove, prune } =
    useWorktrees(activeDirectory);

  const [showNewForm, setShowNewForm] = useState(false);
  const [newBranch, setNewBranch] = useState("");
  const [newPath, setNewPath] = useState("");
  const [newCreateBranch, setNewCreateBranch] = useState(true);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // Per-row state
  const [removingPath, setRemovingPath] = useState<string | null>(null);
  const [forceConfirmPath, setForceConfirmPath] = useState<string | null>(null);

  function openNewForm() {
    if (!activeDirectory) return;
    const suffix = randomSlug();
    setNewBranch(`wt-${suffix}`);
    setNewPath(`${dirParent(activeDirectory)}/${dirBasename(activeDirectory)}-${suffix}`);
    setNewCreateBranch(true);
    setCreateError(null);
    setShowNewForm(true);
  }

  async function handleCreate() {
    if (!newBranch.trim() || !newPath.trim()) {
      setCreateError("Branch and path are required");
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      await create(newPath.trim(), newBranch.trim(), newCreateBranch);
      setShowNewForm(false);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : String(e));
    } finally {
      setCreating(false);
    }
  }

  async function handleRemove(wt: main.Worktree, force: boolean) {
    setRemovingPath(wt.path);
    try {
      await remove(wt.path, force);
      setForceConfirmPath(null);
    } catch (e) {
      // If it failed without --force and there are uncommitted changes, prompt
      if (!force) {
        setForceConfirmPath(wt.path);
      } else {
        setCreateError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setRemovingPath(null);
    }
  }

  return (
    <div className="h-full flex flex-col">
      <div className="border-b border-border px-4 py-3 flex items-center justify-between">
        <div className="min-w-0">
          <h3 className="text-white text-sm flex items-center gap-1.5">
            <GitBranch className="size-3.5 text-muted-foreground" />
            Worktrees
          </h3>
          <p className="text-[10px] text-muted-foreground truncate mt-0.5">
            {worktrees.length} worktree{worktrees.length !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            type="button"
            onClick={openNewForm}
            disabled={!activeDirectory}
            className="p-1 hover:bg-white/10 rounded transition-colors disabled:opacity-30"
            title="New worktree"
          >
            <Plus className="size-3.5 text-muted-foreground" />
          </button>
          <button
            type="button"
            onClick={refresh}
            className="p-1 hover:bg-white/10 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw
              className={`size-3.5 text-muted-foreground ${loading ? "animate-spin" : ""}`}
            />
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {!activeDirectory && (
          <p className="text-xs text-muted-foreground text-center py-8">No active session</p>
        )}

        {activeDirectory && error && (
          <div className="m-3 p-2 rounded bg-error/10 border border-error/20 text-[11px] text-error">
            {error}
          </div>
        )}

        {activeDirectory && worktrees.length === 0 && !loading && !error && (
          <p className="text-xs text-muted-foreground text-center py-8">
            No worktrees (not a git repository?)
          </p>
        )}

        {/* New worktree form */}
        {showNewForm && (
          <div className="m-3 p-3 rounded-md border border-accent/20 bg-accent/5 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                New Worktree
              </span>
              <button
                type="button"
                onClick={() => setShowNewForm(false)}
                className="p-0.5 hover:bg-white/10 rounded"
              >
                <X className="size-3 text-muted-foreground" />
              </button>
            </div>
            <input
              value={newBranch}
              onChange={(e) => setNewBranch(e.target.value)}
              placeholder="Branch name"
              className="w-full px-2 py-1 text-[12px] font-mono bg-white/[0.04] border border-white/[0.06] rounded text-white placeholder:text-tertiary outline-none focus:border-accent/40"
            />
            <label className="flex items-center gap-2 text-[11px] text-muted-foreground cursor-pointer select-none">
              <input
                type="checkbox"
                checked={newCreateBranch}
                onChange={(e) => setNewCreateBranch(e.target.checked)}
                className="accent-accent"
              />
              Create as new branch
            </label>
            <input
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
              placeholder="Worktree path"
              className="w-full px-2 py-1 text-[11px] font-mono bg-white/[0.04] border border-white/[0.06] rounded text-white/80 placeholder:text-tertiary outline-none focus:border-accent/40"
            />
            {createError && <div className="text-[11px] text-error break-words">{createError}</div>}
            <div className="flex justify-end gap-2 pt-1">
              <button
                type="button"
                onClick={() => setShowNewForm(false)}
                className="px-2 py-1 text-[11px] rounded text-muted-foreground hover:text-white"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleCreate}
                disabled={creating}
                className="px-2 py-1 text-[11px] rounded bg-accent/20 text-accent hover:bg-accent/30 border border-accent/30 disabled:opacity-50 flex items-center gap-1.5"
              >
                {creating && <Loader2 className="size-3 animate-spin" />}
                Create
              </button>
            </div>
          </div>
        )}

        {/* Worktree list */}
        {worktrees.map((wt) => {
          const removable = !wt.isMain && !removingPath;
          const isPrunable = wt.prunable;
          const needsForceConfirm = forceConfirmPath === wt.path;
          return (
            <div
              key={wt.path}
              className="px-3 py-2.5 hover:bg-white/[0.03] border-b border-white/[0.04] group"
            >
              <div className="flex items-start gap-2">
                <div className="mt-0.5 shrink-0">
                  {isPrunable ? (
                    <AlertTriangle className="size-3.5 text-warning" />
                  ) : wt.isMain ? (
                    <CheckCircle2 className="size-3.5 text-accent" />
                  ) : (
                    <Folder className="size-3.5 text-muted-foreground" />
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <span className="text-xs text-white truncate">{dirBasename(wt.path)}</span>
                    {wt.isMain && (
                      <span className="text-[9px] px-1 rounded bg-accent/15 text-accent uppercase">
                        Main
                      </span>
                    )}
                    {wt.isDetached && (
                      <span className="text-[9px] px-1 rounded bg-white/10 text-muted-foreground uppercase">
                        Detached
                      </span>
                    )}
                    {wt.hasUncommittedChanges && (
                      <span
                        className="text-[9px] px-1 rounded bg-warning/15 text-warning"
                        title="Uncommitted changes"
                      >
                        Dirty
                      </span>
                    )}
                  </div>
                  {wt.branch && (
                    <span className="flex items-center gap-1 text-[10px] text-muted-foreground mt-0.5">
                      <GitBranch className="size-2.5" /> {wt.branch}
                    </span>
                  )}
                  <span className="text-[10px] text-tertiary truncate block mt-0.5">
                    {shortenPath(wt.path)}
                  </span>
                  {isPrunable && (
                    <span className="text-[10px] text-warning mt-0.5 block">
                      Directory missing — prune to clean up
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                  {!wt.isMain && !isPrunable && (
                    <button
                      type="button"
                      onClick={() => onOpenSession(dirBasename(wt.path), wt.path)}
                      className="text-[10px] px-2 py-1 rounded text-accent border border-accent/20 hover:bg-accent/10"
                      title="Open new session in this worktree"
                    >
                      Open
                    </button>
                  )}
                  {removable && (
                    <button
                      type="button"
                      onClick={() => handleRemove(wt, false)}
                      className="p-1 rounded hover:bg-white/10 text-muted-foreground hover:text-error"
                      title="Remove worktree"
                    >
                      {removingPath === wt.path ? (
                        <Loader2 className="size-3 animate-spin" />
                      ) : (
                        <Trash2 className="size-3" />
                      )}
                    </button>
                  )}
                </div>
              </div>
              {needsForceConfirm && (
                <div className="mt-2 ml-5 p-2 rounded bg-error/10 border border-error/20 text-[10px] text-error space-y-1.5">
                  <div className="flex items-start gap-1">
                    <AlertTriangle className="size-3 shrink-0 mt-0.5" />
                    <span>Worktree has uncommitted changes or other refs. Force remove?</span>
                  </div>
                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => setForceConfirmPath(null)}
                      className="px-1.5 py-0.5 text-tertiary hover:text-white"
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      onClick={() => handleRemove(wt, true)}
                      className="px-1.5 py-0.5 rounded bg-error/20 text-error border border-error/30 hover:bg-error/30"
                    >
                      Force remove
                    </button>
                  </div>
                </div>
              )}
              {isPrunable && (
                <div className="mt-1 ml-5">
                  <button
                    type="button"
                    onClick={prune}
                    className="text-[10px] px-2 py-0.5 rounded text-warning border border-warning/20 hover:bg-warning/10"
                  >
                    Prune
                  </button>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
