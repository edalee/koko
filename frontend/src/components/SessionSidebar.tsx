import { Plus, Trash2, Search, Terminal } from "lucide-react";
import type { SessionTab } from "../types";

interface SessionSidebarProps {
  sessions: SessionTab[];
  activeSessionId: string | null;
  onSessionSelect: (sessionId: string) => void;
  onNewSession: () => void;
  onDeleteSession: (sessionId: string) => void;
}

export default function SessionSidebar({
  sessions,
  activeSessionId,
  onSessionSelect,
  onNewSession,
  onDeleteSession,
}: SessionSidebarProps) {
  return (
    <div className="h-full bg-base flex flex-col border-r border-border">
      <div className="p-4 border-b border-border">
        <button
          onClick={onNewSession}
          className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-md bg-gradient-to-r from-accent to-accent-dark hover:from-accent/90 hover:to-accent-dark/90 text-toolbar font-medium text-sm transition-colors"
        >
          <Plus className="size-4" />
          New Session
        </button>
      </div>

      <div className="p-3">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-gray-400" />
          <input
            placeholder="Search sessions..."
            className="w-full pl-9 pr-3 py-1.5 text-sm bg-white/5 border border-border rounded-md text-white placeholder:text-gray-500 outline-none focus:border-accent/50"
          />
        </div>
      </div>

      <div className="flex-1 overflow-auto px-2">
        {sessions.map((session) => (
          <div
            key={session.id}
            className={`group flex items-start gap-3 p-3 mb-2 rounded-lg cursor-pointer transition-all ${
              activeSessionId === session.id
                ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 border border-accent/30"
                : "hover:bg-white/5 border border-transparent"
            }`}
            onClick={() => onSessionSelect(session.id)}
          >
            <Terminal className="size-5 mt-0.5 text-accent shrink-0" />
            <div className="flex-1 min-w-0">
              <h3 className="text-sm text-white truncate mb-1">{session.title}</h3>
              <p className="text-xs text-gray-400 truncate">Terminal session</p>
            </div>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDeleteSession(session.id);
              }}
              className="opacity-0 group-hover:opacity-100 transition-opacity p-1 hover:bg-white/10 rounded"
            >
              <Trash2 className="size-4 text-gray-400 hover:text-red-400" />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
