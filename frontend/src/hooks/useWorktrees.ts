import { useCallback, useEffect, useRef, useState } from "react";
import {
  CreateWorktree,
  GetWorktreeForDir,
  ListWorktrees,
  PruneWorktrees,
  RemoveWorktree,
} from "../../wailsjs/go/main/GitService";
import type { main } from "../../wailsjs/go/models";

/**
 * Lists worktrees for the repository containing the given directory and
 * exposes mutators (create, remove, prune). Polls every 10s while
 * mounted.
 */
export function useWorktrees(dir: string | null) {
  const [worktrees, setWorktrees] = useState<main.Worktree[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dirRef = useRef(dir);
  dirRef.current = dir;

  const refresh = useCallback(async () => {
    if (!dirRef.current) {
      setWorktrees([]);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      // Resolve to the canonical worktree, then list siblings.
      // This handles being called from inside a non-main worktree.
      const me = await GetWorktreeForDir(dirRef.current).catch(() => null);
      const repoDir = me?.path || dirRef.current;
      const list = await ListWorktrees(repoDir);
      setWorktrees(list || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setWorktrees([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 10_000);
    return () => clearInterval(interval);
  }, [refresh, dir]);

  const create = useCallback(
    async (path: string, branch: string, createBranch: boolean) => {
      if (!dirRef.current) throw new Error("No active directory");
      const wt = await CreateWorktree(dirRef.current, path, branch, createBranch);
      await refresh();
      return wt;
    },
    [refresh],
  );

  const remove = useCallback(
    async (path: string, force: boolean) => {
      await RemoveWorktree(path, force);
      await refresh();
    },
    [refresh],
  );

  const prune = useCallback(async () => {
    if (!dirRef.current) return;
    await PruneWorktrees(dirRef.current);
    await refresh();
  }, [refresh]);

  return { worktrees, loading, error, refresh, create, remove, prune };
}
