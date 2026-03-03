import { ChevronLeft, ChevronRight, FolderTree, GitPullRequest, MessageSquare } from "lucide-react";
import { useState } from "react";
import GitHubPanel from "./GitHubPanel";
import SlackPanel from "./SlackPanel";

type SidebarModule = "explorer" | "github" | "slack";

interface RightSidebarProps {
  isCollapsed: boolean;
  onToggleCollapse: () => void;
}

export default function RightSidebar({ isCollapsed, onToggleCollapse }: RightSidebarProps) {
  const [activeModule, setActiveModule] = useState<SidebarModule>("github");

  function handleModuleClick(module: SidebarModule) {
    setActiveModule(module);
    if (isCollapsed) {
      onToggleCollapse();
    }
  }

  return (
    <div className="h-full flex bg-base border-l border-border relative">
      {/* Module Icons */}
      <div className="w-12 shrink-0 flex flex-col items-center pt-3 gap-2">
        <button
          type="button"
          onClick={() => handleModuleClick("explorer")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "explorer"
              ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 text-accent"
              : "text-muted-foreground hover:text-white hover:bg-white/5"
          }`}
          title="Explorer"
        >
          <FolderTree className="size-5" />
        </button>
        <button
          type="button"
          onClick={() => handleModuleClick("github")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "github"
              ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 text-accent"
              : "text-muted-foreground hover:text-white hover:bg-white/5"
          }`}
          title="GitHub"
        >
          <GitPullRequest className="size-5" />
        </button>
        <button
          type="button"
          onClick={() => handleModuleClick("slack")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "slack"
              ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 text-accent"
              : "text-muted-foreground hover:text-white hover:bg-white/5"
          }`}
          title="Slack"
        >
          <MessageSquare className="size-5" />
        </button>
      </div>

      {/* Module Content */}
      {!isCollapsed && (
        <div className="flex-1 flex flex-col overflow-hidden border-l border-border">
          {activeModule === "explorer" && (
            <div className="h-full flex flex-col">
              <div className="border-b border-border px-4 py-3">
                <h3 className="text-white text-sm">Files</h3>
              </div>
              <div className="flex-1 p-4">
                <p className="text-xs text-muted-foreground text-center py-4">
                  No files in session
                </p>
              </div>
            </div>
          )}
          {activeModule === "github" && <GitHubPanel />}
          {activeModule === "slack" && <SlackPanel />}
        </div>
      )}

      {/* Collapse toggle */}
      <button
        type="button"
        onClick={onToggleCollapse}
        className="absolute bottom-3 left-3 p-1.5 hover:bg-white/10 rounded transition-colors"
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
