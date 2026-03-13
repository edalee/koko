import { useCallback, useEffect, useState } from "react";
import { GetLastMessage } from "../../wailsjs/go/main/ClaudeService";
import { CloseSession, CreateSession } from "../../wailsjs/go/main/TerminalManager";
import { addRecentDir } from "../components/NewSessionDialog";
import type { SessionHistoryEntry, SessionTab } from "../types";

const TABS_STORAGE_KEY = "koko:session-tabs";
const HISTORY_STORAGE_KEY = "koko:session-history";
const MAX_HISTORY = 20;

function loadSavedTabs(): SessionTab[] {
  try {
    const raw = localStorage.getItem(TABS_STORAGE_KEY);
    if (!raw) return [];
    const tabs: SessionTab[] = JSON.parse(raw);
    // Mark all as disconnected (no PTY after restart)
    return tabs.map((t) => ({ ...t, connected: false }));
  } catch {
    return [];
  }
}

function saveTabs(tabs: SessionTab[]) {
  localStorage.setItem(TABS_STORAGE_KEY, JSON.stringify(tabs));
}

function loadHistory(): SessionHistoryEntry[] {
  try {
    return JSON.parse(localStorage.getItem(HISTORY_STORAGE_KEY) || "[]");
  } catch {
    return [];
  }
}

function saveHistory(history: SessionHistoryEntry[]) {
  localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(history.slice(0, MAX_HISTORY)));
}

async function addToHistory(tab: SessionTab) {
  let lastMessage = "";
  try {
    lastMessage = await GetLastMessage(tab.directory);
  } catch {
    // ignore
  }
  const history = loadHistory();
  // Deduplicate by directory — keep the most recent
  const filtered = history.filter((h) => h.directory !== tab.directory);
  filtered.unshift({
    name: tab.name,
    directory: tab.directory,
    createdAt: tab.createdAt,
    closedAt: Date.now(),
    lastMessage,
  });
  saveHistory(filtered);
}

export function useSessionTabs() {
  const [tabs, setTabs] = useState<SessionTab[]>(loadSavedTabs);
  const [activeTabId, setActiveTabId] = useState<string | null>(() => {
    const saved = loadSavedTabs();
    return saved.length > 0 ? saved[0].id : null;
  });

  // Persist tabs to localStorage on every change
  useEffect(() => {
    saveTabs(tabs);
  }, [tabs]);

  const createTab = useCallback(async (name: string, directory: string) => {
    const sessionId = await CreateSession(name, directory, 80, 24, false);
    addRecentDir(directory);
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
    const sessionId = await CreateSession(tab.name, tab.directory, 80, 24, true);
    // Replace the old disconnected tab with the new connected one
    setTabs((prev) =>
      prev.map((t) => (t.id === tab.id ? { ...t, id: sessionId, connected: true } : t)),
    );
    setActiveTabId(sessionId);
    return sessionId;
  }, []);

  const closeTab = useCallback(
    async (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (tab) {
        addToHistory(tab);
        if (tab.connected) {
          await CloseSession(tabId);
        }
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
    [activeTabId, tabs],
  );

  const switchTab = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (!tab) return;

      if (!tab.connected && activeTabId === tabId) {
        // Already viewing this disconnected tab — reconnect it
        reconnectTab(tab);
        return;
      }

      // Select the tab (whether connected or not)
      setActiveTabId(tabId);
    },
    [tabs, activeTabId, reconnectTab],
  );

  const renameTab = useCallback((tabId: string, newName: string) => {
    setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, name: newName } : t)));
  }, []);

  const handleSessionExit = useCallback(
    (tabId: string) => {
      // Mark as disconnected instead of removing
      setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, connected: false } : t)));
      if (activeTabId === tabId) {
        // Stay on the tab — user can see "[Session ended]" and click to resume
      }
    },
    [activeTabId],
  );

  const history = loadHistory();

  const clearHistoryEntry = useCallback((directory: string) => {
    const h = loadHistory().filter((e) => e.directory !== directory);
    saveHistory(h);
  }, []);

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
