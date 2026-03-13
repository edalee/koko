import { useCallback, useEffect, useRef, useState } from "react";
import { GetChildProcesses } from "../../wailsjs/go/main/ProcessMonitor";
import { GetSessionPID } from "../../wailsjs/go/main/TerminalManager";

export interface SubagentProcess {
  pid: number;
  command: string;
  fullCmd: string;
  type: "subagent" | "mcp" | "infrastructure" | "tool";
  elapsed: string;
  elapsedMs: number;
  children: number;
}

interface UseSubagentsResult {
  processes: SubagentProcess[];
  agentCount: number;
  loading: boolean;
  refresh: () => void;
}

export function useSubagents(sessionId: string | null): UseSubagentsResult {
  const [processes, setProcesses] = useState<SubagentProcess[]>([]);
  const [loading, setLoading] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchProcesses = useCallback(async () => {
    if (!sessionId) {
      setProcesses([]);
      return;
    }

    try {
      const pid = await GetSessionPID(sessionId);
      if (!pid) {
        setProcesses([]);
        return;
      }

      const children = await GetChildProcesses(pid);
      if (!children) {
        setProcesses([]);
        return;
      }

      setProcesses(children as SubagentProcess[]);
    } catch {
      // Session may not exist yet or has exited
      setProcesses([]);
    }
  }, [sessionId]);

  const refresh = useCallback(() => {
    setLoading(true);
    fetchProcesses().finally(() => setLoading(false));
  }, [fetchProcesses]);

  // Poll every 5 seconds
  useEffect(() => {
    if (!sessionId) {
      setProcesses([]);
      return;
    }

    fetchProcesses();
    intervalRef.current = setInterval(fetchProcesses, 5000);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [sessionId, fetchProcesses]);

  const agentCount = processes.filter((p) => p.type === "subagent").length;

  return { processes, agentCount, loading, refresh };
}
