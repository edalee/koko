import { useState, useCallback } from "react";
import {
  CreateSession,
  CloseSession,
} from "../../wailsjs/go/main/TerminalManager";
import type { SessionTab } from "../types";

export function useSessionTabs() {
  const [tabs, setTabs] = useState<SessionTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);

  const createTab = useCallback(async () => {
    const sessionId = await CreateSession(80, 24);
    const newTab: SessionTab = {
      id: sessionId,
      title: `Session ${sessionId.replace("session-", "")}`,
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
    handleSessionExit,
  };
}
