import { useCallback, useEffect, useRef, useState } from "react";
import { GetSessionSummary } from "../../wailsjs/go/main/ClaudeService";
import type { main } from "../../wailsjs/go/models";
import type { SessionTab } from "../types";
import type { SessionState } from "./useSessionActivity";

const POLL_MS = 15000;

interface Target {
  uuid: string;
  directory: string;
}

/**
 * Summaries of the tabs' conversations, keyed by Claude session UUID (plan
 * 036). The key is the UUID, not the tab id, because a reconnect gives the tab
 * a new id.
 *
 * It fetches when a UUID first appears, every 15 seconds while Koko is
 * visible, and when a tab goes from active to idle. The backend caches each
 * file, so an unchanged transcript costs only a stat.
 */
export function useSessionSummaries(
  tabs: SessionTab[],
  states: Map<string, SessionState>,
): Map<string, main.SessionSummary> {
  const [summaries, setSummaries] = useState<Map<string, main.SessionSummary>>(new Map());

  const targets: Target[] = tabs
    .filter((t) => t.claudeSessionId)
    .map((t) => ({ uuid: t.claudeSessionId as string, directory: t.directory }));
  const key = targets.map((t) => `${t.uuid}|${t.directory}`).join("\n");
  const targetsRef = useRef(targets);
  targetsRef.current = targets;

  const fetchOne = useCallback(async (t: Target) => {
    let s: main.SessionSummary;
    try {
      s = await GetSessionSummary(t.directory, t.uuid);
    } catch {
      return;
    }
    // Only a real change makes a new map, so an unchanged poll renders nothing.
    setSummaries((prev) => {
      const old = prev.get(t.uuid);
      if (old && JSON.stringify(old) === JSON.stringify(s)) return prev;
      const next = new Map(prev);
      next.set(t.uuid, s);
      return next;
    });
  }, []);

  const fetchAll = useCallback(() => {
    for (const t of targetsRef.current) void fetchOne(t);
  }, [fetchOne]);

  // A new or changed set of conversations: fetch now, and drop closed ones.
  useEffect(() => {
    const live = new Set(key.split("\n").map((k) => k.split("|")[0]));
    setSummaries((prev) => {
      if ([...prev.keys()].every((uuid) => live.has(uuid))) return prev;
      return new Map([...prev].filter(([uuid]) => live.has(uuid)));
    });
    if (key) fetchAll();
  }, [key, fetchAll]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (document.visibilityState === "visible") fetchAll();
    }, POLL_MS);
    const onVisible = () => {
      if (document.visibilityState === "visible") fetchAll();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      clearInterval(interval);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [fetchAll]);

  // A tab that has just gone idle has most likely finished a turn, so its
  // title and recap may have changed.
  const prevStates = useRef<Map<string, SessionState>>(new Map());
  const tabsRef = useRef(tabs);
  tabsRef.current = tabs;
  useEffect(() => {
    for (const [id, state] of states) {
      if (state !== "idle" || prevStates.current.get(id) !== "active") continue;
      const tab = tabsRef.current.find((t) => t.id === id);
      if (tab?.claudeSessionId) {
        void fetchOne({ uuid: tab.claudeSessionId, directory: tab.directory });
      }
    }
    prevStates.current = new Map(states);
  }, [states, fetchOne]);

  return summaries;
}
