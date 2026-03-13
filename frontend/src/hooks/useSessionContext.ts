import { useCallback, useEffect, useState } from "react";
import { GetAgents, GetCommands, GetMCPServers } from "../../wailsjs/go/main/ClaudeService";
import type { main } from "../../wailsjs/go/models";

interface UseSessionContextResult {
  mcpServers: main.MCPServer[];
  agents: main.AgentInfo[];
  commands: main.CommandInfo[];
  loading: boolean;
  refresh: () => void;
}

export function useSessionContext(directory: string | null): UseSessionContextResult {
  const [mcpServers, setMcpServers] = useState<main.MCPServer[]>([]);
  const [agents, setAgents] = useState<main.AgentInfo[]>([]);
  const [commands, setCommands] = useState<main.CommandInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchAll = useCallback(async () => {
    if (!directory) {
      setMcpServers([]);
      setAgents([]);
      setCommands([]);
      return;
    }

    setLoading(true);

    const results = await Promise.allSettled([
      GetMCPServers(directory),
      GetAgents(directory),
      GetCommands(directory),
    ]);

    if (results[0].status === "fulfilled") setMcpServers(results[0].value ?? []);
    if (results[1].status === "fulfilled") setAgents(results[1].value ?? []);
    if (results[2].status === "fulfilled") setCommands(results[2].value ?? []);

    setLoading(false);
  }, [directory]);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  return { mcpServers, agents, commands, loading, refresh: fetchAll };
}
