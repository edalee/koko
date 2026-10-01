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

/** What reloading a tab should do (plan 028 step 6). */
export type ReloadAction = "pick" | "confirm" | "reload";

/**
 * Decide how to reload a tab, given its activity state: "active", "idle" or
 * "approval", as tracked from terminal output.
 *
 * A live tab whose Claude is not idle asks first, because reload stops it
 * mid-task and loses anything typed but not sent. That comes before anything
 * else: a busy tab with no stored conversation must be confirmed too, or
 * picking a row would kill it unasked. After that, a tab with no stored
 * conversation opens the picker, because reloading would otherwise have to
 * guess (D1). Otherwise reload goes straight ahead.
 */
export function reloadAction(tab: SessionTab, sessionState: string): ReloadAction {
  if (tab.connected && sessionState !== "idle") return "confirm";
  if (!tab.claudeSessionId) return "pick";
  return "reload";
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

  // Read the slug once, straight after a session is created.
  //
  // The slug is assigned by the backend and has no other route to the
  // frontend. It used to be left as "" for ever, so every persisted slug was
  // empty and koko-1 named nothing.
  //
  // The conversation id needs no read here. A resume already knows it, and a
  // fresh conversation's id arrives later on the session:claude-id event.
  const mergeSlug = useCallback(async (sessionId: string) => {
    const slug = await GetSessionSlug(sessionId).catch(() => "");
    if (!slug) return;
    setTabs((prev) => prev.map((t) => (t.id === sessionId ? { ...t, slug } : t)));
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
        claudeSessionId: resume?.claudeSessionId ?? "",
        // Reuse the slug a closed session had, so koko-1 keeps naming it.
        slug: resume?.slug ?? "",
        replaces: "", // a new tab takes over from nothing
      });
      const newTab: SessionTab = {
        id: sessionId,
        slug: "", // filled in by mergeSlug below
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
      void mergeSlug(sessionId);
      return sessionId;
    },
    [mergeSlug],
  );

  // The backend tells us when it captures a Claude session UUID. Claude writes
  // its session file on the first message, so this can arrive minutes after the
  // session starts. Polling on a timer missed it, which left the tab with no
  // UUID, so a later reconnect had nothing to resume.
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
          // An id resumes that conversation. Without one the backend starts a
          // fresh conversation: there is no --continue to fall back on.
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

        void mergeSlug(sessionId);
        return sessionId;
      } catch (err) {
        // Show why on the tab's reconnect card. Logging alone left the tab
        // looking broken, and every later click failed the same silent way.
        console.error("reconnectTab failed:", err);
        // Ask the backend whether the replaced session still exists, rather
        // than guess from the error text. It is gone only if the backend got
        // as far as closing it, and then the tab is disconnected. A refusal,
        // or a failure before that point, leaves the tab as it was. Its exit
        // event was ignored above, so the state has to be set here.
        const stillThere = await GetSessionSlug(tab.id).then(
          () => true,
          () => false,
        );
        setTabs((prev) =>
          prev.map((t) =>
            t.id === tab.id
              ? {
                  ...t,
                  reconnectError: reconnectMessage(err),
                  connected: stillThere ? t.connected : false,
                }
              : t,
          ),
        );
        throw err;
      } finally {
        reconnectingRef.current.delete(tab.id);
      }
    },
    [mergeSlug],
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
      // A reconnect closes the session it replaces, which fires this exit.
      // Treating it as a real exit would flash the reconnect card during a
      // reload, and could mark the new session dead if the event arrived
      // after the id swap.
      if (reconnectingRef.current.has(tabId)) return;
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

  // Deleted conversations are gone for good, so closed-session records that
  // point at them drop the id. Choosing such a record then starts fresh in
  // its directory, rather than failing to resume a missing file.
  const forgetConversations = useCallback(
    (ids: string[]) => {
      const gone = new Set(ids);
      if (!historyRef.current.some((e) => e.claudeSessionId && gone.has(e.claudeSessionId))) {
        return;
      }
      const newHistory = historyRef.current.map((e) =>
        e.claudeSessionId && gone.has(e.claudeSessionId) ? { ...e, claudeSessionId: "" } : e,
      );
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
    forgetConversations,
  };
}
