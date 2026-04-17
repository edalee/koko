import { useCallback, useState } from "react";
import { FetchPRFileDiff } from "../../wailsjs/go/main/GitHubService";
import { GetFileContent, GetFileDiff } from "../../wailsjs/go/main/GitService";
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
  rawFile: main.FileContentData | null;
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
    rawFile: null,
    loading: false,
    viewMode: "split",
    filePath: "",
    staged: false,
    prContext: null,
    prFiles: [],
  });

  const openFile = useCallback(async (dir: string, path: string) => {
    setState((prev) => ({
      ...prev,
      isOpen: true,
      loading: true,
      file: null,
      rawFile: null,
      filePath: path,
      staged: false,
      prContext: null,
      prFiles: [],
    }));

    try {
      const data = await GetFileContent(dir, path);
      setState((prev) => ({ ...prev, rawFile: data, loading: false }));
    } catch {
      setState((prev) => ({ ...prev, rawFile: null, loading: false }));
    }
  }, []);

  const openDiff = useCallback(async (dir: string, path: string, staged: boolean) => {
    setState((prev) => ({
      ...prev,
      isOpen: true,
      loading: true,
      file: null,
      rawFile: null,
      filePath: path,
      staged,
      prContext: null,
      prFiles: [],
    }));

    try {
      const data = await GetFileDiff(dir, path, staged);
      setState((prev) => ({ ...prev, file: data, loading: false }));
    } catch {
      setState((prev) => ({ ...prev, file: null, loading: false }));
    }
  }, []);

  const openPRDiff = useCallback(
    async (repo: string, number: number, path: string, files?: PRFileItem[]) => {
      setState((prev) => ({
        ...prev,
        isOpen: true,
        loading: true,
        file: null,
        rawFile: null,
        filePath: path,
        staged: false,
        prContext: { repo, number },
        prFiles: files ?? prev.prFiles,
      }));

      try {
        const data = await FetchPRFileDiff(repo, number, path);
        setState((prev) => ({ ...prev, file: data, loading: false }));
      } catch {
        setState((prev) => ({ ...prev, file: null, loading: false }));
      }
    },
    [],
  );

  const close = useCallback(() => {
    setState({
      isOpen: false,
      file: null,
      rawFile: null,
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
    rawFile: state.rawFile,
    loading: state.loading,
    viewMode: state.viewMode,
    filePath: state.filePath,
    staged: state.staged,
    prContext: state.prContext,
    prFiles: state.prFiles,
    openFile,
    openDiff,
    openPRDiff,
    close,
    setViewMode,
  };
}
