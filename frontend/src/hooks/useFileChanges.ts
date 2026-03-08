import { useCallback, useEffect, useRef, useState } from "react";
import { GetBranchName, GetFileChanges } from "../../wailsjs/go/main/GitService";

export interface FileChange {
  path: string;
  status: "added" | "modified" | "deleted" | "renamed";
  staged: boolean;
}

export function useFileChanges(directory: string | null) {
  const [changes, setChanges] = useState<FileChange[]>([]);
  const [branch, setBranch] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const refresh = useCallback(async () => {
    if (!directory) {
      setChanges([]);
      setBranch("");
      return;
    }
    try {
      setLoading(true);
      const [files, branchName] = await Promise.all([
        GetFileChanges(directory),
        GetBranchName(directory),
      ]);
      setChanges((files as FileChange[]) ?? []);
      setBranch(branchName);
    } catch {
      setChanges([]);
      setBranch("");
    } finally {
      setLoading(false);
    }
  }, [directory]);

  useEffect(() => {
    refresh();
    // Poll every 5 seconds
    intervalRef.current = setInterval(refresh, 5000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [refresh]);

  return { changes, branch, loading, refresh };
}
