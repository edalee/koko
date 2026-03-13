import { useEffect, useRef, useState } from "react";
import { GetSessionState } from "../../wailsjs/go/main/TerminalManager";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const IDLE_THRESHOLD_MS = 4000;

export type SessionState = "active" | "idle" | "approval";

/**
 * Tracks session activity state by monitoring PTY output.
 * - "active": PTY producing output (Claude is working)
 * - "idle": No output, session finished or waiting for new prompt
 * - "approval": No output, Claude is waiting for tool approval
 */
export function useSessionActivity(sessionIds: string[]): Map<string, SessionState> {
  const [states, setStates] = useState<Map<string, SessionState>>(new Map());
  const lastActivityRef = useRef<Map<string, number>>(new Map());
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    const cleanups: (() => void)[] = [];

    for (const id of sessionIds) {
      lastActivityRef.current.set(id, Date.now());
      const cancel = EventsOn(`pty:data:${id}`, () => {
        lastActivityRef.current.set(id, Date.now());
        // Immediately mark as active when output arrives
        setStates((prev) => {
          if (prev.get(id) !== "active") {
            const next = new Map(prev);
            next.set(id, "active");
            return next;
          }
          return prev;
        });
      });
      cleanups.push(cancel);
    }

    // Check for idle sessions every second
    timerRef.current = setInterval(async () => {
      const now = Date.now();
      const updates: [string, SessionState][] = [];

      for (const id of sessionIds) {
        const last = lastActivityRef.current.get(id) ?? now;
        if (now - last >= IDLE_THRESHOLD_MS) {
          // Session is idle — check if it's waiting for approval
          try {
            const state = await GetSessionState(id);
            updates.push([id, state === "approval" ? "approval" : "idle"]);
          } catch {
            updates.push([id, "idle"]);
          }
        }
      }

      if (updates.length > 0) {
        setStates((prev) => {
          let changed = false;
          const next = new Map(prev);
          for (const [id, state] of updates) {
            if (prev.get(id) !== state) {
              next.set(id, state);
              changed = true;
            }
          }
          return changed ? next : prev;
        });
      }
    }, 1000);

    return () => {
      for (const cancel of cleanups) cancel();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [sessionIds]);

  return states;
}
