import { ChevronDown, ChevronRight, X } from "lucide-react";
import { useState } from "react";
import { Session } from "../types";

type SessionTabsProps = {
  sessions: Session[];
  activeSession: Session | null;
  onSessionSelect: (session: Session) => void;
  onSessionClose: (sessionId: string) => void;
};

export function SessionTabs({
  sessions,
  activeSession,
  onSessionSelect,
  onSessionClose,
}: SessionTabsProps) {
  const [isExpanded, setIsExpanded] = useState(true);

  if (sessions.length === 0) return null;

  return (
    <div className="border-b border-white/10">
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
        <span className="text-xs text-gray-400 uppercase tracking-wider">
          Open Sessions ({sessions.length})
        </span>
      </div>

      {isExpanded && (
        <div className="flex overflow-x-auto">
          {sessions.map((session) => {
            const isActive = activeSession?.id === session.id;
            return (
              <div
                key={session.id}
                className={`relative flex items-center gap-2 px-4 py-3 min-w-0 cursor-pointer transition-all border-r border-white/10 ${
                  isActive
                    ? "bg-gradient-to-r from-[#1FF2AB]/30 to-[#24A965]/30"
                    : "hover:bg-white/5"
                }`}
                onClick={() => onSessionSelect(session)}
              >
                {isActive && (
                  <div className="absolute inset-0 bg-gradient-to-r from-[#1FF2AB]/40 to-[#24A965]/40 animate-pulse" />
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