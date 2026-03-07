import { useCallback, useState } from "react";
import { CloseSession, CreateSession } from "../../wailsjs/go/main/TerminalManager";
import { addRecentDir } from "../components/NewSessionDialog";
import type { SessionTab } from "../types";

export function useSessionTabs() {
  const [tabs, setTabs] = useState<SessionTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);

  const createTab = useCallback(async (name: string, directory: string) => {
    const sessionId = await CreateSession(name, directory, 80, 24);
    addRecentDir(directory);
    const newTab: SessionTab = {
      id: sessionId,
      name,
      directory,
      createdAt: Date.now(),
    };
    setTabs((prev) => [...prev, newTab]);
    setActiveTabId(sessionId);
    return sessionId;
  }, []);

  const closeTab = useCallback(
    async (tabId: string) => {
      await CloseSession(tabId);
      setTabs((prev) => {
        const remaining = prev.filter((t) => t.id !== tabId);
        if (activeTabId === tabId) {
          const idx = prev.findIndex((t) => t.id === tabId);
          const nextTab = remaining[Math.min(idx, remaining.length - 1)];
          setActiveTabId(nextTab?.id ?? null);
        }
        return remaining;
      });
    },
    [activeTabId],
  );

  const switchTab = useCallback((tabId: string) => {
    setActiveTabId(tabId);
  }, []);

  const renameTab = useCallback((tabId: string, newName: string) => {
    setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, name: newName } : t)));
  }, []);

  const handleSessionExit = useCallback(
    (tabId: string) => {
      setTabs((prev) => {
        const remaining = prev.filter((t) => t.id !== tabId);
        if (activeTabId === tabId) {
          const idx = prev.findIndex((t) => t.id === tabId);
          const nextTab = remaining[Math.min(idx, remaining.length - 1)];
          setActiveTabId(nextTab?.id ?? null);
        }
        return remaining;
      });
    },
    [activeTabId],
  );

  return {
    tabs,
    activeTabId,
    createTab,
    closeTab,
    switchTab,
    renameTab,
    handleSessionExit,
  };
}
