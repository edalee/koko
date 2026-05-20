import { useEffect, useState } from "react";
import { GetBranchName } from "../../wailsjs/go/main/GitService";

/**
 * Fetches the current git branch for each unique directory in the list.
 * Returns a Map<directory, branch>. Missing entries mean either the
 * directory isn't a git repo or the lookup hasn't resolved yet.
 *
 * Debounced refresh (15s) — branch changes are rare.
 */
export function useSessionBranches(directories: string[]): Map<string, string> {
  const [branches, setBranches] = useState<Map<string, string>>(new Map());

  // Stable key from sorted unique directories
  const uniqueDirs = Array.from(new Set(directories.filter(Boolean))).sort();
  const key = uniqueDirs.join("|");

  useEffect(() => {
    let cancelled = false;

    async function load() {
      const next = new Map<string, string>();
      await Promise.all(
        uniqueDirs.map(async (dir) => {
          try {
            const branch = await GetBranchName(dir);
            if (branch) next.set(dir, branch);
          } catch {
            // Not a git repo — skip
          }
        }),
      );
      if (!cancelled) setBranches(next);
    }

    load();
    const interval = setInterval(load, 15000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return branches;
}
