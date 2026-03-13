import { useEffect, useRef, useState } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const IDLE_THRESHOLD_MS = 4000;

/**
 * Tracks which sessions are idle (no PTY output for N seconds).
 * When Claude is working, the spinner generates continuous output.
 * Silence while the process is alive means Claude is waiting for user input.
 */
export function useSessionActivity(sessionIds: string[]): Set<string> {
  const [idleSessions, setIdleSessions] = useState<Set<string>>(new Set());
  const lastActivityRef = useRef<Map<string, number>>(new Map());
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    const cleanups: (() => void)[] = [];

    for (const id of sessionIds) {
      lastActivityRef.current.set(id, Date.now());
      const cancel = EventsOn(`pty:data:${id}`, () => {
        lastActivityRef.current.set(id, Date.now());
      });
      cleanups.push(cancel);
    }

    // Check for idle sessions every second
    timerRef.current = setInterval(() => {
      const now = Date.now();
      const newIdle = new Set<string>();
      for (const id of sessionIds) {
        const last = lastActivityRef.current.get(id) ?? now;
        if (now - last >= IDLE_THRESHOLD_MS) {
          newIdle.add(id);
        }
      }
      setIdleSessions((prev) => {
        // Only update if changed to avoid re-renders
        if (prev.size !== newIdle.size || [...newIdle].some((id) => !prev.has(id))) {
          return newIdle;
        }
        return prev;
      });
    }, 1000);

    return () => {
      for (const cancel of cleanups) cancel();
      if (timerRef.current) clearInterval(timerRef.current);
      // Clean up removed sessions
      for (const id of lastActivityRef.current.keys()) {
        if (!sessionIds.includes(id)) lastActivityRef.current.delete(id);
      }
    };
  }, [sessionIds]);

  return idleSessions;
}
