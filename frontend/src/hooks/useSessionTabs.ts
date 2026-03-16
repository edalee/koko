import { useCallback, useEffect, useRef, useState } from "react";
import { GetLastMessage } from "../../wailsjs/go/main/ClaudeService";
import { GetSessions, SaveSessions } from "../../wailsjs/go/main/ConfigService";
import { CloseSession, CreateSession } from "../../wailsjs/go/main/TerminalManager";
import type { SessionHistoryEntry, SessionTab } from "../types";

const MAX_HISTORY = 20;

export function useSessionTabs() {
  const [tabs, setTabs] = useState<SessionTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [history, setHistory] = useState<SessionHistoryEntry[]>([]);
  const historyRef = useRef<SessionHistoryEntry[]>([]);
  const loadedRef = useRef(false);

  // Load sessions from Go backend on mount
  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;

    GetSessions()
      .then((sessions) => {
        const savedTabs = (sessions.tabs || []).map(
          (t: { id: string; name: string; directory: string; createdAt: number }) => ({
            ...t,
            connected: false,
          }),
        );
        setTabs(savedTabs);
        if (savedTabs.length > 0) {
          setActiveTabId(savedTabs[0].id);
        }
        const h = sessions.history || [];
        setHistory(h);
        historyRef.current = h;
      })
      .catch(() => {
        // No sessions yet
      });
  }, []);

  // Persist tabs to Go backend on every change (skip initial empty state)
  const initialRender = useRef(true);
  useEffect(() => {
    if (initialRender.current) {
      initialRender.current = false;
      return;
    }
    SaveSessions({
      tabs: tabs.map(({ id, name, directory, createdAt }) => ({
        id,
        name,
        directory,
        createdAt,
      })),
      history: historyRef.current,
      recentDirs: [...new Set(tabs.map((t) => t.directory))].slice(0, 5),
      // biome-ignore lint/suspicious/noExplicitAny: Wails-generated SessionsData class requires convertValues but plain objects work at runtime
    } as any).catch(() => {});
  }, [tabs]);

  const saveCurrentState = useCallback(
    (newTabs: SessionTab[], newHistory: SessionHistoryEntry[]) => {
      SaveSessions({
        tabs: newTabs.map(({ id, name, directory, createdAt }) => ({
          id,
          name,
          directory,
          createdAt,
        })),
        history: newHistory,
        recentDirs: [...new Set(newTabs.map((t) => t.directory))].slice(0, 5),
        // biome-ignore lint/suspicious/noExplicitAny: Wails-generated SessionsData class requires convertValues but plain objects work at runtime
      } as any).catch(() => {});
    },
    [],
  );

  const createTab = useCallback(async (name: string, directory: string) => {
    const sessionId = await CreateSession(name, directory, 80, 24, false);
    const newTab: SessionTab = {
      id: sessionId,
      name,
      directory,
      createdAt: Date.now(),
      connected: true,
    };
    setTabs((prev) => [...prev, newTab]);
    setActiveTabId(sessionId);
    return sessionId;
  }, []);

  const reconnectTab = useCallback(async (tab: SessionTab) => {
    try {
      const sessionId = await CreateSession(tab.name, tab.directory, 80, 24, true);
      setTabs((prev) =>
        prev.map((t) => (t.id === tab.id ? { ...t, id: sessionId, connected: true } : t)),
      );
      setActiveTabId(sessionId);
      return sessionId;
    } catch (err) {
      console.error("reconnectTab failed:", err);
      return "";
    }
  }, []);

  const closeTab = useCallback(
    async (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (tab) {
        // Add to history
        let lastMessage = "";
        try {
          lastMessage = await GetLastMessage(tab.directory);
        } catch {
          // ignore
        }
        const filtered = historyRef.current.filter((h) => h.directory !== tab.directory);
        filtered.unshift({
          name: tab.name,
          directory: tab.directory,
          createdAt: tab.createdAt,
          closedAt: Date.now(),
          lastMessage,
        });
        const newHistory = filtered.slice(0, MAX_HISTORY);
        historyRef.current = newHistory;
        setHistory(newHistory);

        if (tab.connected) {
          await CloseSession(tabId);
        }

        // Save with updated history
        const remaining = tabs.filter((t) => t.id !== tabId);
        saveCurrentState(remaining, newHistory);
      }
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
    [activeTabId, tabs, saveCurrentState],
  );

  const switchTab = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (!tab) return;

      if (!tab.connected) {
        reconnectTab(tab);
        return;
      }

      setActiveTabId(tabId);
    },
    [tabs, reconnectTab],
  );

  const renameTab = useCallback((tabId: string, newName: string) => {
    setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, name: newName } : t)));
  }, []);

  const handleSessionExit = useCallback((tabId: string) => {
    setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, connected: false } : t)));
  }, []);

  const clearHistoryEntry = useCallback(
    (directory: string) => {
      const newHistory = historyRef.current.filter((e) => e.directory !== directory);
      historyRef.current = newHistory;
      setHistory(newHistory);
      saveCurrentState(tabs, newHistory);
    },
    [tabs, saveCurrentState],
  );

  return {
    tabs,
    activeTabId,
    createTab,
    closeTab,
    switchTab,
    renameTab,
    handleSessionExit,
    history,
    clearHistoryEntry,
  };
}
