import { GitPullRequest, MessageSquare, X } from "lucide-react";
import { useState } from "react";
import GitHubPanel from "./GitHubPanel";
import SlackPanel from "./SlackPanel";

interface RightSidebarProps {
  onClose: () => void;
}

type SidebarModule = "github" | "slack";

export default function RightSidebar({ onClose }: RightSidebarProps) {
  const [activeModule, setActiveModule] = useState<SidebarModule>("github");

  return (
    <div className="h-full flex bg-base border-l border-border">
      {/* Module Icons */}
      <div className="w-12 bg-surface border-r border-border flex flex-col items-center py-3 gap-2">
        <button
          onClick={() => setActiveModule("github")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "github"
              ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 text-accent"
              : "text-gray-400 hover:text-white hover:bg-white/5"
          }`}
          title="GitHub"
        >
          <GitPullRequest className="size-5" />
        </button>
        <button
          onClick={() => setActiveModule("slack")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "slack"
              ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 text-accent"
              : "text-gray-400 hover:text-white hover:bg-white/5"
          }`}
          title="Slack"
        >
          <MessageSquare className="size-5" />
        </button>

        <div className="flex-1" />

        <button
          onClick={onClose}
          className="p-2 rounded-lg text-gray-400 hover:text-white hover:bg-white/5 transition-colors"
          title="Close"
        >
          <X className="size-5" />
        </button>
      </div>

      {/* Module Content */}
      <div className="flex-1 min-w-[300px] flex flex-col overflow-hidden">
        {activeModule === "github" && <GitHubPanel />}
        {activeModule === "slack" && <SlackPanel />}
      </div>
    </div>
  );
}
