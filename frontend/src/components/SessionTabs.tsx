import { ChevronDown, ChevronRight, X } from "lucide-react";
import { useState } from "react";
import type { SessionTab } from "../types";

interface SessionTabsProps {
  sessions: SessionTab[];
  activeSessionId: string | null;
  onSessionSelect: (sessionId: string) => void;
  onSessionClose: (sessionId: string) => void;
}

export default function SessionTabs({
  sessions,
  activeSessionId,
  onSessionSelect,
  onSessionClose,
}: SessionTabsProps) {
  const [isExpanded, setIsExpanded] = useState(true);

  if (sessions.length === 0) return null;

  return (
    <div className="border-b border-border">
      <div className="flex items-center gap-2 px-4 py-2 bg-black/20">
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="p-1 hover:bg-white/10 rounded transition-colors"
        >
          {isExpanded ? (
            <ChevronDown className="size-4 text-gray-400" />
          ) : (
            <ChevronRight className="size-4 text-gray-400" />
          )}
        </button>
        <span className="text-xs text-muted-foreground uppercase tracking-wider">
          Open Sessions ({sessions.length})
        </span>
      </div>

      {isExpanded && (
        <div className="flex overflow-x-auto">
          {sessions.map((session) => {
            const isActive = activeSessionId === session.id;
            return (
              <div
                key={session.id}
                className={`relative flex items-center gap-2 px-4 py-3 min-w-0 cursor-pointer transition-all border-r border-border ${
                  isActive
                    ? "bg-gradient-to-r from-accent/30 to-accent-dark/30"
                    : "hover:bg-white/5"
                }`}
                onClick={() => onSessionSelect(session.id)}
              >
                {isActive && (
                  <div className="absolute inset-0 bg-gradient-to-r from-accent/40 to-accent-dark/40 animate-pulse" />
                )}
                <span
                  className={`relative text-sm truncate max-w-[200px] ${
                    isActive ? "text-white font-medium" : "text-gray-300"
                  }`}
                >
                  {session.title}
                </span>
                <button
                  className="relative shrink-0 hover:bg-white/20 rounded p-1 transition-colors"
                  onClick={(e) => {
                    e.stopPropagation();
                    onSessionClose(session.id);
                  }}
                >
                  <X className="size-3 text-gray-300" />
                </button>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
