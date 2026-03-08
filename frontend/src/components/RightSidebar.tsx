import { Bot, ChevronLeft, ChevronRight, FileCode2 } from "lucide-react";
import { useState } from "react";

type SidebarModule = "files" | "agents";

interface RightSidebarProps {
  isCollapsed: boolean;
  onToggleCollapse: () => void;
}

export default function RightSidebar({ isCollapsed, onToggleCollapse }: RightSidebarProps) {
  const [activeModule, setActiveModule] = useState<SidebarModule>("files");

  function handleModuleClick(module: SidebarModule) {
    setActiveModule(module);
    if (isCollapsed) {
      onToggleCollapse();
    }
  }

  return (
    <div className="h-full flex bg-white/[0.03] glass-panel border-l border-border relative overflow-hidden">
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
          onClick={() => handleModuleClick("agents")}
          className={`p-1.5 rounded-md transition-colors ${
            activeModule === "agents"
              ? "text-accent bg-white/[0.08]"
              : "text-muted-foreground hover:text-white hover:bg-white/5"
          }`}
          title="Subagents"
        >
          <Bot className="size-4" />
        </button>
      </div>

      {/* Module Content */}
      {!isCollapsed && (
        <div className="flex-1 flex flex-col overflow-hidden border-l border-border bg-white/[0.02] animate-fade-in">
          {activeModule === "files" && (
            <div className="h-full flex flex-col">
              <div className="border-b border-border px-4 py-3">
                <h3 className="text-white text-sm">File Changes</h3>
              </div>
              <div className="flex-1 p-4">
                <p className="text-xs text-muted-foreground text-center py-4">
                  No file changes in session
                </p>
              </div>
            </div>
          )}
          {activeModule === "agents" && (
            <div className="h-full flex flex-col">
              <div className="border-b border-border px-4 py-3">
                <h3 className="text-white text-sm">Subagents</h3>
              </div>
              <div className="flex-1 p-4">
                <p className="text-xs text-muted-foreground text-center py-4">
                  No active subagents
                </p>
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
