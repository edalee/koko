import { useCallback, useEffect, useRef, useState } from "react";
import { GetLastMessage } from "../../wailsjs/go/main/ClaudeService";
import { GetSessions, SaveSessions } from "../../wailsjs/go/main/ConfigService";
import {
  CloseSession,
  CreateSessionWithOpts,
  GetClaudeSessionID,
} from "../../wailsjs/go/main/TerminalManager";
import type { SessionHistoryEntry, SessionTab } from "../types";

const MAX_HISTORY = 50;

export function useSessionTabs() {
  const [tabs, setTabs] = useState<SessionTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [history, setHistory] = useState<SessionHistoryEntry[]>([]);
  const historyRef = useRef<SessionHistoryEntry[]>([]);
  const loadedRef = useRef(false);
  const hasLoadedSessions = useRef(false);

  // Load sessions from Go backend on mount
  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;

    GetSessions()
      .then((sessions) => {
        // New format: sessions.sessions contains SessionRecord[]
        const records = sessions.sessions || [];
        let nextPlaceholder = 0;

        const savedTabs = records
          .filter((r: { status: string }) => r.status === "active" || r.status === "disconnected")
          .map(
            (r: {
              slug: string;
              name: string;
              directory: string;
              createdAt: number;
              claudeSessionId?: string;
              lastMsg?: string;
            }) => ({
              id: `saved-${++nextPlaceholder}`,
              slug: r.slug || "",
              name: r.name,
              directory: r.directory,
              createdAt: r.createdAt,
              connected: false,
              claudeSessionId: r.claudeSessionId || "",
              lastMsg: r.lastMsg || "",
            }),
          );

        const closedHistory: SessionHistoryEntry[] = records
          .filter((r: { status: string }) => r.status === "closed")
          .map((r) => ({
            slug: r.slug || "",
            name: r.name,
            directory: r.directory,
            createdAt: r.createdAt,
            closedAt: r.closedAt || 0,
            lastMessage: r.lastMsg || "",
            claudeSessionId: r.claudeSessionId || "",
          }));

        setTabs(savedTabs);
        if (savedTabs.length > 0) {
          setActiveTabId(savedTabs[0].id);
        }
        setHistory(closedHistory);
        historyRef.current = closedHistory;
        hasLoadedSessions.current = true;
      })
      .catch(() => {
        hasLoadedSessions.current = true;
      });
  }, []);

  // Persist sessions on every tab change
  useEffect(() => {
    if (!hasLoadedSessions.current) return;
    const records = [
      ...tabs.map((t) => ({
        slug: t.slug,
        name: t.name,
        directory: t.directory,
        claudeSessionId: t.claudeSessionId || "",
        createdAt: t.createdAt,
        status: t.connected ? "active" : "disconnected",
        lastMsg: t.lastMsg || "",
      })),
      ...historyRef.current.map((h) => ({
        slug: h.slug,
        name: h.name,
        directory: h.directory,
        claudeSessionId: h.claudeSessionId || "",
        createdAt: h.createdAt,
        closedAt: h.closedAt,
        status: "closed",
        lastMsg: h.lastMessage || "",
      })),
    ];
    SaveSessions({
      sessions: records,
      recentDirs: [...new Set(tabs.map((t) => t.directory))].slice(0, 10),
      // biome-ignore lint/suspicious/noExplicitAny: Wails-generated type mismatch
    } as any).catch(() => {});
  }, [tabs]);

  const saveCurrentState = useCallback(
    (newTabs: SessionTab[], newHistory: SessionHistoryEntry[]) => {
      const records = [
        ...newTabs.map((t) => ({
          slug: t.slug,
          name: t.name,
          directory: t.directory,
          claudeSessionId: t.claudeSessionId || "",
          createdAt: t.createdAt,
          status: t.connected ? "active" : "disconnected",
          lastMsg: t.lastMsg || "",
        })),
        ...newHistory.map((h) => ({
          slug: h.slug,
          name: h.name,
          directory: h.directory,
          claudeSessionId: h.claudeSessionId || "",
          createdAt: h.createdAt,
          closedAt: h.closedAt,
          status: "closed",
          lastMsg: h.lastMessage || "",
        })),
      ];
      SaveSessions({
        sessions: records,
        recentDirs: [...new Set(newTabs.map((t) => t.directory))].slice(0, 10),
        // biome-ignore lint/suspicious/noExplicitAny: Wails-generated type mismatch
      } as any).catch(() => {});
    },
    [],
  );

  const createTab = useCallback(async (name: string, directory: string) => {
    const sessionId = await CreateSessionWithOpts({
      name,
      dir: directory,
      cols: 80,
      rows: 24,
      resume: false,
      claudeSessionId: "",
    });
    const newTab: SessionTab = {
      id: sessionId,
      slug: "", // will be populated from GetSessions or after capture
      name,
      directory,
      createdAt: Date.now(),
      connected: true,
    };
    setTabs((prev) => [...prev, newTab]);
    setActiveTabId(sessionId);

    // Capture Claude session UUID and slug after a short delay
    setTimeout(async () => {
      try {
        const claudeId = await GetClaudeSessionID(sessionId);
        if (claudeId) {
          setTabs((prev) =>
            prev.map((t) => (t.id === sessionId ? { ...t, claudeSessionId: claudeId } : t)),
          );
        }
      } catch {
        // ignore
      }
    }, 5000);

    return sessionId;
  }, []);

  const reconnectingRef = useRef<Set<string>>(new Set());
  const reconnectTab = useCallback(async (tab: SessionTab) => {
    if (reconnectingRef.current.has(tab.id)) return "";
    reconnectingRef.current.add(tab.id);
    try {
      const sessionId = await CreateSessionWithOpts({
        name: tab.name,
        dir: tab.directory,
        cols: 80,
        rows: 24,
        resume: true,
        claudeSessionId: tab.claudeSessionId || "",
      });
      setTabs((prev) =>
        prev.map((t) => (t.id === tab.id ? { ...t, id: sessionId, connected: true } : t)),
      );
      setActiveTabId((prev) => (prev === tab.id ? sessionId : prev));

      // Capture new Claude session UUID if we didn't have one
      if (!tab.claudeSessionId) {
        setTimeout(async () => {
          try {
            const claudeId = await GetClaudeSessionID(sessionId);
            if (claudeId) {
              setTabs((prev) =>
                prev.map((t) => (t.id === sessionId ? { ...t, claudeSessionId: claudeId } : t)),
              );
            }
          } catch {
            // ignore
          }
        }, 5000);
      }

      return sessionId;
    } catch (err) {
      console.error("reconnectTab failed:", err);
      return "";
    } finally {
      reconnectingRef.current.delete(tab.id);
    }
  }, []);

  const closeTab = useCallback(
    async (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (tab) {
        let lastMessage = "";
        try {
          lastMessage = await GetLastMessage(tab.directory);
        } catch {
          // ignore
        }

        // Capture Claude session ID before closing
        let claudeId = tab.claudeSessionId || "";
        if (!claudeId && tab.connected) {
          try {
            claudeId = await GetClaudeSessionID(tabId);
          } catch {
            // ignore
          }
        }

        const newEntry: SessionHistoryEntry = {
          slug: tab.slug,
          name: tab.name,
          directory: tab.directory,
          createdAt: tab.createdAt,
          closedAt: Date.now(),
          lastMessage,
          claudeSessionId: claudeId,
        };

        const newHistory = [newEntry, ...historyRef.current].slice(0, MAX_HISTORY);
        historyRef.current = newHistory;
        setHistory(newHistory);

        if (tab.connected) {
          await CloseSession(tabId);
        }

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

      setActiveTabId(tabId);

      if (!tab.connected) {
        reconnectTab(tab);
      }
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
