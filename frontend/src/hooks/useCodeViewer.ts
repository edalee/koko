import { useCallback, useState } from "react";
import { GetFileDiff } from "../../wailsjs/go/main/GitService";
import type { main } from "../../wailsjs/go/models";

export type ViewMode = "split" | "unified";

interface CodeViewerState {
  isOpen: boolean;
  file: main.FileDiffData | null;
  loading: boolean;
  viewMode: ViewMode;
  filePath: string;
  staged: boolean;
}

export function useCodeViewer() {
  const [state, setState] = useState<CodeViewerState>({
    isOpen: false,
    file: null,
    loading: false,
    viewMode: "split",
    filePath: "",
    staged: false,
  });

  const openDiff = useCallback(async (dir: string, path: string, staged: boolean) => {
    setState((prev) => ({
      ...prev,
      isOpen: true,
      loading: true,
      filePath: path,
      staged,
    }));

    try {
      const data = await GetFileDiff(dir, path, staged);
      setState((prev) => ({
        ...prev,
        file: data,
        loading: false,
      }));
    } catch {
      setState((prev) => ({
        ...prev,
        loading: false,
      }));
    }
  }, []);

  const close = useCallback(() => {
    setState({
      isOpen: false,
      file: null,
      loading: false,
      viewMode: "split",
      filePath: "",
      staged: false,
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
    openDiff,
    close,
    setViewMode,
  };
}
