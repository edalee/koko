import { useCallback, useEffect, useRef, useState } from "react";
import { GetLastMessage } from "../../wailsjs/go/main/ClaudeService";
import { GetSessions, SaveSessions } from "../../wailsjs/go/main/ConfigService";
import {
  CloseSession,
  CreateSessionWithOpts,
  GetClaudeSessionID,
  GetSessionSlug,
} from "../../wailsjs/go/main/TerminalManager";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { SessionHistoryEntry, SessionTab } from "../types";

const MAX_HISTORY = 50;

/**
 * Turns a failed reconnect into a line for the reconnect card. Wails rejects
 * with the Go error's text, so ErrConversationBusy arrives as its message.
 */
export function reconnectMessage(err: unknown): string {
  const text = err instanceof Error ? err.message : String(err);
  if (text.includes("already open in another session")) {
    return "This conversation is already open in another tab.";
  }
  if (text.includes("already reconnecting")) {
    return "This tab is already reconnecting. Try again in a moment.";
  }
  return `Could not reconnect: ${text}`;
}

/** A stored conversation to reopen, instead of starting a fresh one. */
export interface ResumeTarget {
  claudeSessionId: string;
  // The slug the conversation had when its tab was closed, if known.
  slug?: string;
}

/** How to reconnect a tab: into a given conversation, or a fresh one. */
export interface ReconnectChoice {
  claudeSessionId?: string;
  fresh?: boolean;
}

/** Payload of the session:claude-id event emitted by TerminalManager. */
interface SessionClaudeIDEvent {
  sessionId: string;
  claudeSessionId: string;
}

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
              worktreePath?: string;
            }) => ({
              id: `saved-${++nextPlaceholder}`,
              slug: r.slug || "",
              name: r.name,
              directory: r.directory,
              createdAt: r.createdAt,
              connected: false,
              claudeSessionId: r.claudeSessionId || "",
              lastMsg: r.lastMsg || "",
              worktreePath: r.worktreePath || undefined,
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
        worktreePath: t.worktreePath || "",
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

  // Read the slug and UUID once, straight after a session is created.
  //
  // The slug is assigned by the backend and has no other route to the
  // frontend. It used to be left as "" for ever, so every persisted slug was
  // empty and koko-1 named nothing.
  //
  // For --continue the backend resolves the UUID synchronously, so the
  // session:claude-id event fires before the caller has the new session id and
  // the event listener matches no tab. This closes that gap too.
  const mergeSessionMeta = useCallback(async (sessionId: string) => {
    const [slug, claudeId] = await Promise.all([
      GetSessionSlug(sessionId).catch(() => ""),
      GetClaudeSessionID(sessionId).catch(() => ""),
    ]);
    if (!slug && !claudeId) return;
    setTabs((prev) =>
      prev.map((t) =>
        t.id === sessionId
          ? {
              ...t,
              slug: slug || t.slug,
              claudeSessionId: claudeId || t.claudeSessionId,
            }
          : t,
      ),
    );
  }, []);

  const createTab = useCallback(
    async (name: string, directory: string, worktreePath?: string, resume?: ResumeTarget) => {
      // Rejects with ErrConversationBusy if another session holds the
      // conversation. The caller shows that, rather than it vanishing.
      const sessionId = await CreateSessionWithOpts({
        name,
        dir: directory,
        cols: 80,
        rows: 24,
        resume: !!resume,
        claudeSessionId: resume?.claudeSessionId ?? "",
        // Reuse the slug a closed session had, so koko-1 keeps naming it.
        slug: resume?.slug ?? "",
        replaces: "", // a new tab takes over from nothing
      });
      const newTab: SessionTab = {
        id: sessionId,
        slug: "", // filled in by mergeSessionMeta below
        name,
        directory,
        createdAt: Date.now(),
        connected: true,
        worktreePath,
        // Known up front when resuming, so the picker marks it held at once.
        claudeSessionId: resume?.claudeSessionId,
      };
      setTabs((prev) => [...prev, newTab]);
      setActiveTabId(sessionId);

      // The slug is ready now. The UUID usually resolves later, on the
      // session:claude-id event.
      void mergeSessionMeta(sessionId);
      return sessionId;
    },
    [mergeSessionMeta],
  );

  // The backend tells us when it captures a Claude session UUID. Claude writes
  // its session file on the first message, so this can arrive minutes after the
  // session starts. Polling on a timer missed it, which left the tab with no
  // UUID and made a later resume fall back to --continue.
  useEffect(() => {
    return EventsOn("session:claude-id", (payload: SessionClaudeIDEvent) => {
      if (!payload?.sessionId || !payload.claudeSessionId) return;
      setTabs((prev) =>
        prev.map((t) =>
          t.id === payload.sessionId ? { ...t, claudeSessionId: payload.claudeSessionId } : t,
        ),
      );
    });
  }, []);

  const reconnectingRef = useRef<Set<string>>(new Set());
  /**
   * Restart a tab's Claude process in the same tab, keeping its slug.
   *
   * With no choice it resumes the tab's own conversation. The picker passes a
   * choice: a specific conversation, or a fresh one. Rejects on failure, after
   * recording the reason on the tab's reconnect card, so the picker can show
   * it too.
   */
  const reconnectTab = useCallback(
    async (tab: SessionTab, choice?: ReconnectChoice) => {
      // Reject rather than return quietly. The picker treated a quiet return
      // as success and closed, so the choice the user made was dropped and
      // the in-flight reconnect won instead.
      if (reconnectingRef.current.has(tab.id)) {
        throw new Error("This tab is already reconnecting.");
      }
      reconnectingRef.current.add(tab.id);
      const fresh = choice?.fresh === true;
      const claudeSessionId = fresh ? "" : choice?.claudeSessionId || tab.claudeSessionId || "";
      try {
        const sessionId = await CreateSessionWithOpts({
          name: tab.name,
          dir: tab.directory,
          cols: 80,
          rows: 24,
          resume: !fresh,
          claudeSessionId,
          // Keep the slug, so koko-1 still names this session after a restart.
          slug: tab.slug || "",
          // Name the session being taken over. Its entry lingers in the
          // backend until something closes it, and the ownership guard would
          // otherwise see this tab's own conversation as already held.
          replaces: tab.id,
        });
        setTabs((prev) =>
          prev.map((t) =>
            t.id === tab.id
              ? {
                  ...t,
                  id: sessionId,
                  connected: true,
                  reconnectError: undefined,
                  // A fresh conversation's id arrives later, on the event.
                  claudeSessionId: claudeSessionId || undefined,
                }
              : t,
          ),
        );
        setActiveTabId((prev) => (prev === tab.id ? sessionId : prev));

        void mergeSessionMeta(sessionId);
        return sessionId;
      } catch (err) {
        // Show why on the tab's reconnect card. Logging alone left the tab
        // looking broken, and every later click failed the same silent way.
        console.error("reconnectTab failed:", err);
        setTabs((prev) =>
          prev.map((t) => (t.id === tab.id ? { ...t, reconnectError: reconnectMessage(err) } : t)),
        );
        throw err;
      } finally {
        reconnectingRef.current.delete(tab.id);
      }
    },
    [mergeSessionMeta],
  );

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

  // Only selects the tab. It used to reconnect a disconnected tab on the
  // spot, silently, into whatever conversation it had or none. Reconnecting
  // is now a choice made in the session dialog (plan 028, step 5).
  const switchTab = useCallback(
    (tabId: string) => {
      if (tabs.some((t) => t.id === tabId)) setActiveTabId(tabId);
    },
    [tabs],
  );

  const renameTab = useCallback((tabId: string, newName: string) => {
    setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, name: newName } : t)));
  }, []);

  const handleSessionExit = useCallback(
    (tabId: string) => {
      setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, connected: false } : t)));
      // Capture last message for the context card on reconnect
      const tab = tabs.find((t) => t.id === tabId);
      if (tab) {
        GetLastMessage(tab.directory)
          .then((msg) => {
            if (msg) {
              setTabs((prev) => prev.map((t) => (t.id === tabId ? { ...t, lastMsg: msg } : t)));
            }
          })
          .catch(() => {});
      }
    },
    [tabs],
  );

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
    reconnectTab,
    renameTab,
    handleSessionExit,
    history,
    clearHistoryEntry,
  };
}
