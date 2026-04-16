import { useCallback, useEffect, useRef, useState } from "react";
import { FetchBranchCI } from "../../wailsjs/go/main/GitHubService";
import { GetRepoSlug } from "../../wailsjs/go/main/GitService";
import type { BranchCI } from "../types";

const REFRESH_INTERVAL = 60_000; // 60 seconds

export function useCI(directory: string | null, branch: string) {
  const [ci, setCI] = useState<BranchCI | null>(null);
  const [loading, setLoading] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastKeyRef = useRef("");

  const refresh = useCallback(async () => {
    if (!directory || !branch) {
      setCI(null);
      return;
    }
    try {
      setLoading(true);
      const slug = await GetRepoSlug(directory);
      if (!slug) {
        setCI(null);
        return;
      }
      const result = await FetchBranchCI(slug, branch);
      setCI(result);
    } catch {
      setCI(null);
    } finally {
      setLoading(false);
    }
  }, [directory, branch]);

  useEffect(() => {
    const key = `${directory}:${branch}`;
    // Reset when branch/directory changes
    if (key !== lastKeyRef.current) {
      lastKeyRef.current = key;
      setCI(null);
    }

    refresh();
    if (intervalRef.current) clearInterval(intervalRef.current);
    intervalRef.current = setInterval(refresh, REFRESH_INTERVAL);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [refresh]);

  return { ci, loading, refresh };
}
