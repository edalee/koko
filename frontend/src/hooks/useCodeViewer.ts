import { useCallback, useState } from "react";
import { FetchPRFileDiff } from "../../wailsjs/go/main/GitHubService";
import { GetFileDiff } from "../../wailsjs/go/main/GitService";
import type { main } from "../../wailsjs/go/models";

export type ViewMode = "split" | "unified";

export interface PRFileItem {
  path: string;
  additions: number;
  deletions: number;
}

interface CodeViewerState {
  isOpen: boolean;
  file: main.FileDiffData | null;
  loading: boolean;
  viewMode: ViewMode;
  filePath: string;
  staged: boolean;
  prContext: { repo: string; number: number } | null;
  prFiles: PRFileItem[];
}

export function useCodeViewer() {
  const [state, setState] = useState<CodeViewerState>({
    isOpen: false,
    file: null,
    loading: false,
    viewMode: "split",
    filePath: "",
    staged: false,
    prContext: null,
    prFiles: [],
  });

  const openDiff = useCallback(async (dir: string, path: string, staged: boolean) => {
    setState((prev) => ({
      ...prev,
      isOpen: true,
      loading: true,
      filePath: path,
      staged,
      prContext: null,
      prFiles: [],
    }));

    try {
      const data = await GetFileDiff(dir, path, staged);
      setState((prev) => ({ ...prev, file: data, loading: false }));
    } catch {
      setState((prev) => ({ ...prev, loading: false }));
    }
  }, []);

  const openPRDiff = useCallback(
    async (repo: string, number: number, path: string, files?: PRFileItem[]) => {
      setState((prev) => ({
        ...prev,
        isOpen: true,
        loading: true,
        filePath: path,
        staged: false,
        prContext: { repo, number },
        prFiles: files ?? prev.prFiles,
      }));

      try {
        const data = await FetchPRFileDiff(repo, number, path);
        setState((prev) => ({ ...prev, file: data, loading: false }));
      } catch {
        setState((prev) => ({ ...prev, loading: false }));
      }
    },
    [],
  );

  const close = useCallback(() => {
    setState({
      isOpen: false,
      file: null,
      loading: false,
      viewMode: "split",
      filePath: "",
      staged: false,
      prContext: null,
      prFiles: [],
    });
  }, []);

  const setViewMode = useCallback((mode: ViewMode) => {
    setState((prev) => ({ ...prev, viewMode: mode }));
  }, []);

  return {
    isOpen: state.isOpen,
    file: state.file,
    loading: state.loading,
    viewMode: state.viewMode,
    filePath: state.filePath,
    staged: state.staged,
    prContext: state.prContext,
    prFiles: state.prFiles,
    openDiff,
    openPRDiff,
    close,
    setViewMode,
  };
}
