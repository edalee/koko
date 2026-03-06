import { useCallback, useEffect, useRef, useState } from "react";
import { FetchPRs } from "../../wailsjs/go/main/GitHubService";
import type { GitHubPR } from "../types";

const REFRESH_INTERVAL = 60_000; // 60 seconds

export function useGitHub() {
  const [prs, setPRs] = useState<GitHubPR[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await FetchPRs();
      setPRs(result ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch PRs");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    intervalRef.current = setInterval(refresh, REFRESH_INTERVAL);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [refresh]);

  return { prs, loading, error, refresh };
}
