import {
  Bot,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Circle,
  FileCode2,
  FileMinus,
  FilePlus,
  FileText,
  Plug,
  RefreshCw,
  Sparkles,
  Terminal,
} from "lucide-react";
import { useState } from "react";
import type { main } from "../../wailsjs/go/models";
import type { FileChange } from "../hooks/useFileChanges";
import type { SubagentProcess } from "../hooks/useSubagents";

type SidebarModule = "files" | "context";

interface RightSidebarProps {
  isCollapsed: boolean;
  onToggleCollapse: () => void;
  fileChanges: FileChange[];
  branch: string;
  fileChangesLoading: boolean;
  onRefreshFileChanges: () => void;
  processes: SubagentProcess[];
  agentCount: number;
  mcpServers: main.MCPServer[];
  agents: main.AgentInfo[];
  commands: main.CommandInfo[];
  contextLoading: boolean;
  onRefreshContext: () => void;
  onInjectCommand: (command: string) => void;
  hasActiveSession: boolean;
}

function changeColor(change: FileChange): string {
  if (change.staged) return "text-green-400";
  switch (change.status) {
    case "added":
      return "text-green-400";
    case "deleted":
      return "text-red-400";
    case "renamed":
      return "text-blue-400";
    default:
      return "text-yellow-400";
  }
}

function statusIcon(change: FileChange) {
  const color = changeColor(change);
  switch (change.status) {
    case "added":
      return <FilePlus className={`size-3.5 ${color} shrink-0`} />;
    case "deleted":
      return <FileMinus className={`size-3.5 ${color} shrink-0`} />;
    default:
      return <FileText className={`size-3.5 ${color} shrink-0`} />;
  }
}

function statusLabel(change: FileChange): string {
  const prefix = change.staged ? "S" : "";
  switch (change.status) {
    case "added":
      return `${prefix}A`;
    case "deleted":
      return `${prefix}D`;
    case "renamed":
      return `${prefix}R`;
    default:
      return `${prefix}M`;
  }
}

function fileName(path: string) {
  return path.split("/").pop() ?? path;
}

function fileDir(path: string) {
  const parts = path.split("/");
  if (parts.length <= 1) return "";
  const dirParts = parts.slice(0, -1);
  if (dirParts.length > 3) {
    return `.../${dirParts.slice(-3).join("/")}`;
  }
  return dirParts.join("/");
}

function statusDot(status: string) {
  switch (status) {
    case "connected":
      return "bg-green-400";
    case "auth_needed":
      return "bg-yellow-400";
    default:
      return "bg-red-400";
  }
}

function CollapsibleSection({
  title,
  icon,
  count,
  defaultOpen = true,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  count?: number;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 px-3 py-2 w-full text-left hover:bg-white/[0.04] transition-colors"
      >
        <ChevronDown
          className={`size-3 text-tertiary transition-transform ${open ? "" : "-rotate-90"}`}
        />
        {icon}
        <span className="text-[10px] text-muted-foreground font-medium uppercase tracking-wider">
          {title}
        </span>
        {count !== undefined && count > 0 && (
          <span className="text-[10px] text-tertiary ml-auto">{count}</span>
        )}
      </button>
      {open && children}
    </div>
  );
}

export default function RightSidebar({
  isCollapsed,
  onToggleCollapse,
  fileChanges,
  branch,
  fileChangesLoading,
  onRefreshFileChanges,
  processes,
  agentCount,
  mcpServers,
  agents,
  commands,
  contextLoading,
  onRefreshContext,
  onInjectCommand,
  hasActiveSession,
}: RightSidebarProps) {
  const [activeModule, setActiveModule] = useState<SidebarModule>("files");

  const subagents = processes.filter((p) => p.type === "subagent");

  function handleModuleClick(module: SidebarModule) {
    setActiveModule(module);
    if (isCollapsed) {
      onToggleCollapse();
    }
  }

  return (
    <div
      className="h-full flex bg-white/[0.03] glass-panel border-l relative overflow-hidden"
      style={{
        borderImage: "linear-gradient(to bottom, rgba(255,255,255,0.15), rgba(255,255,255,0.04)) 1",
      }}
    >
      {/* Module Icons */}
      <div className="w-9 shrink-0 flex flex-col items-center py-3 gap-1.5">
        <button
          type="button"
          onClick={() => handleModuleClick("files")}
          className={`p-1.5 rounded-md transition-colors ${
            activeModule === "files"
              ? "text-accent bg-white/[0.08]"
              : "text-muted-foreground hover:text-white hover:bg-white/5"
          }`}
          title="File Changes"
        >
          <FileCode2 className="size-4" />
        </button>
        <button
          type="button"
          onClick={() => handleModuleClick("context")}
          className={`relative p-1.5 rounded-md transition-colors ${
            activeModule === "context"
              ? "text-accent bg-white/[0.08]"
              : "text-muted-foreground hover:text-white hover:bg-white/5"
          }`}
          title="Session Context"
        >
          <Bot className="size-4" />
          {agentCount > 0 && (
            <span className="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-accent" />
          )}
        </button>
      </div>

      {/* Module Content */}
      {!isCollapsed && (
        <div className="flex-1 flex flex-col overflow-hidden border-l border-border bg-white/[0.02] animate-fade-in">
          {activeModule === "files" && (
            <div className="h-full flex flex-col">
              <div className="border-b border-border px-4 py-3 flex items-center justify-between">
                <div className="min-w-0">
                  <h3 className="text-white text-sm">File Changes</h3>
                  {branch && (
                    <p className="text-[10px] text-muted-foreground truncate mt-0.5">
                      {branch}
                      <span className="text-tertiary ml-1.5">
                        {fileChanges.length} file{fileChanges.length !== 1 ? "s" : ""}
                      </span>
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={onRefreshFileChanges}
                  className="p-1 hover:bg-white/10 rounded transition-colors shrink-0"
                  title="Refresh"
                >
                  <RefreshCw
                    className={`size-3.5 text-muted-foreground ${fileChangesLoading ? "animate-spin" : ""}`}
                  />
                </button>
              </div>
              <div className="flex-1 overflow-auto">
                {fileChanges.length === 0 ? (
                  <p className="text-xs text-muted-foreground text-center py-8">
                    {branch ? "No changes on this branch" : "No active session"}
                  </p>
                ) : (
                  <div className="py-1">
                    {fileChanges.map((change) => (
                      <div
                        key={`${change.path}-${change.staged}`}
                        className="flex items-center gap-2 px-3 py-1.5 hover:bg-white/[0.04] transition-colors group"
                        title={`${change.path}${change.staged ? " (staged)" : ""}`}
                      >
                        {statusIcon(change)}
                        <div className="flex-1 min-w-0">
                          <span className="text-xs text-white truncate block">
                            {fileName(change.path)}
                          </span>
                          {fileDir(change.path) && (
                            <span className="text-[10px] text-tertiary truncate block">
                              {fileDir(change.path)}
                            </span>
                          )}
                        </div>
                        <span
                          className={`text-[10px] font-mono opacity-0 group-hover:opacity-100 transition-opacity ${changeColor(change)}`}
                        >
                          {statusLabel(change)}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
          {activeModule === "context" && (
            <div className="h-full flex flex-col">
              <div className="border-b border-border px-4 py-3 flex items-center justify-between">
                <div className="min-w-0">
                  <h3 className="text-white text-sm">Session Context</h3>
                  {hasActiveSession && (
                    <p className="text-[10px] text-muted-foreground mt-0.5">
                      {mcpServers.length > 0 && (
                        <span>
                          {mcpServers.length} MCP{mcpServers.length !== 1 ? "s" : ""}
                        </span>
                      )}
                      {mcpServers.length > 0 && commands.length > 0 && (
                        <span className="text-tertiary"> · </span>
                      )}
                      {commands.length > 0 && (
                        <span>
                          {commands.length} cmd{commands.length !== 1 ? "s" : ""}
                        </span>
                      )}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={onRefreshContext}
                  className="p-1 hover:bg-white/10 rounded transition-colors shrink-0"
                  title="Refresh"
                >
                  <RefreshCw
                    className={`size-3.5 text-muted-foreground ${contextLoading ? "animate-spin" : ""}`}
                  />
                </button>
              </div>
              <div className="flex-1 overflow-auto">
                {!hasActiveSession ? (
                  <p className="text-xs text-muted-foreground text-center py-8">
                    No active session
                  </p>
                ) : (
                  <div className="py-1">
                    {/* Activity */}
                    <div className="px-3 py-2">
                      {subagents.length > 0 ? (
                        subagents.map((proc) => (
                          <div key={proc.pid} className="flex items-center gap-2 py-0.5">
                            <span className="relative flex size-2">
                              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent opacity-75" />
                              <span className="relative inline-flex rounded-full size-2 bg-accent" />
                            </span>
                            <span className="text-xs text-white">{proc.command}</span>
                            <span className="text-[10px] text-accent font-mono ml-auto">
                              {proc.elapsed}
                            </span>
                          </div>
                        ))
                      ) : (
                        <div className="flex items-center gap-2">
                          <Circle className="size-2 fill-tertiary text-tertiary shrink-0" />
                          <span className="text-xs text-muted-foreground">Idle</span>
                        </div>
                      )}
                    </div>

                    {/* MCP Servers */}
                    {mcpServers.length > 0 && (
                      <CollapsibleSection
                        title="MCP Servers"
                        icon={<Plug className="size-3 text-blue-400" />}
                        count={mcpServers.length}
                      >
                        {mcpServers.map((server) => (
                          <div
                            key={server.name}
                            className="flex items-center gap-2 px-3 py-1.5 pl-7 hover:bg-white/[0.04] transition-colors"
                            title={server.command}
                          >
                            <span
                              className={`size-1.5 rounded-full shrink-0 ${statusDot(server.status)}`}
                            />
                            <span className="text-xs text-white/80 truncate">{server.name}</span>
                          </div>
                        ))}
                      </CollapsibleSection>
                    )}

                    {/* Agents */}
                    {agents.length > 0 && (
                      <CollapsibleSection
                        title="Agents"
                        icon={<Sparkles className="size-3 text-accent" />}
                        count={agents.length}
                      >
                        {agents.map((agent) => (
                          <div
                            key={agent.name}
                            className="flex items-center gap-2 px-3 py-1.5 pl-7 hover:bg-white/[0.04] transition-colors"
                          >
                            <span className="text-xs text-white/80">{agent.name}</span>
                            <span className="text-[10px] text-tertiary ml-auto truncate max-w-[50%] text-right">
                              {agent.model}
                            </span>
                          </div>
                        ))}
                      </CollapsibleSection>
                    )}

                    {/* Commands */}
                    <CollapsibleSection
                      title="Commands"
                      icon={<Terminal className="size-3 text-muted-foreground" />}
                      count={commands.length || undefined}
                    >
                      {commands.length > 0 ? (
                        commands.map((cmd) => (
                          <button
                            key={`${cmd.name}-${cmd.source}`}
                            type="button"
                            onClick={() => onInjectCommand(`/${cmd.name}\n`)}
                            className="flex items-center gap-2 px-3 py-1.5 pl-7 w-full text-left hover:bg-white/[0.06] transition-colors group"
                            title={cmd.description || `/${cmd.name}`}
                          >
                            {cmd.type === "agent" ? (
                              <Bot className="size-3 text-accent shrink-0" />
                            ) : (
                              <Terminal className="size-3 text-muted-foreground shrink-0" />
                            )}
                            <span className="text-xs text-white/80 truncate">/{cmd.name}</span>
                            <span className="text-[9px] text-tertiary/60 ml-auto shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                              {cmd.source}
                            </span>
                          </button>
                        ))
                      ) : (
                        <p className="text-[10px] text-tertiary px-3 py-1.5 pl-7">
                          None in this project
                        </p>
                      )}
                    </CollapsibleSection>

                    {/* Empty state when nothing loaded yet */}
                    {mcpServers.length === 0 &&
                      agents.length === 0 &&
                      commands.length === 0 &&
                      !contextLoading && (
                        <p className="text-xs text-muted-foreground text-center py-6">
                          No context available
                        </p>
                      )}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Collapse toggle */}
      <button
        type="button"
        onClick={onToggleCollapse}
        className="absolute bottom-3 left-1.5 p-1 hover:bg-white/10 rounded transition-colors"
        title={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      >
        {isCollapsed ? (
          <ChevronLeft className="size-4 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 text-muted-foreground" />
        )}
      </button>
    </div>
  );
}
