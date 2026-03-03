import { ChevronLeft, ChevronRight, Plus, Search, SquareTerminal, X } from "lucide-react";
import type { SessionTab } from "../types";

interface SessionSidebarProps {
  sessions: SessionTab[];
  activeSessionId: string | null;
  onSessionSelect: (sessionId: string) => void;
  onNewSession: () => void;
  onDeleteSession: (sessionId: string) => void;
  isCollapsed: boolean;
  onToggleCollapse: () => void;
}

export default function SessionSidebar({
  sessions,
  activeSessionId,
  onSessionSelect,
  onNewSession,
  onDeleteSession,
  isCollapsed,
  onToggleCollapse,
}: SessionSidebarProps) {
  return (
    <div className="h-full bg-base flex flex-col border-r border-border relative">
      {!isCollapsed && (
        <>
          {/* Search + New Session */}
          <div className="px-3 pt-3 pb-3 space-y-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
              <input
                placeholder="Search sessions..."
                className="w-full pl-9 pr-3 py-2 text-sm bg-white/5 border border-border rounded-md text-white placeholder:text-muted-foreground/60 outline-none focus:border-accent/50"
              />
            </div>

            <button
              type="button"
              onClick={onNewSession}
              className="w-full flex items-center justify-center gap-2 px-3 py-2.5 rounded-md bg-base text-white hover:bg-surface whitespace-nowrap relative overflow-hidden group transition-colors"
              style={{
                border: "2px solid transparent",
                borderImage: "linear-gradient(to right, #1FF2AB, #24A965) 1",
              }}
            >
              <div className="absolute inset-0 bg-gradient-to-r from-accent/10 to-accent-dark/10 opacity-0 group-hover:opacity-100 transition-opacity" />
              <Plus className="size-4 relative z-10" />
              <span className="text-sm font-medium relative z-10">New Session</span>
            </button>
          </div>

          {/* Session list */}
          <div className="flex-1 overflow-auto px-2 pt-2 pb-10">
            <div className="space-y-2">
              {sessions.map((session) => {
                const isActive = activeSessionId === session.id;
                return (
                  // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: nested interactive elements require div wrapper
                  <div
                    key={session.id}
                    className={`group flex items-start gap-3 p-3 rounded-lg cursor-pointer transition-all ${
                      isActive
                        ? "bg-gradient-to-r from-accent/20 to-accent-dark/20 border border-accent/30"
                        : "hover:bg-white/5 border border-transparent"
                    }`}
                    onClick={() => onSessionSelect(session.id)}
                  >
                    <SquareTerminal
                      className={`size-5 mt-0.5 shrink-0 transition-colors ${
                        isActive ? "text-accent" : "text-muted-foreground group-hover:text-accent"
                      }`}
                    />
                    <div className="flex-1 min-w-0">
                      <h3 className="text-sm text-white truncate mb-1">{session.title}</h3>
                      <p className="text-xs text-muted-foreground truncate">Terminal session</p>
                    </div>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        onDeleteSession(session.id);
                      }}
                      className="p-1 hover:bg-white/10 rounded transition-opacity"
                    >
                      <X className="size-4 text-muted-foreground hover:text-[#F14D4C]" />
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        </>
      )}

      {/* Collapsed state */}
      {isCollapsed && (
        <div className="flex-1 flex flex-col items-center pt-4 gap-3">
          <button
            type="button"
            onClick={onNewSession}
            className="p-2 rounded-lg bg-base hover:bg-surface text-muted-foreground hover:text-accent transition-colors"
            style={{
              border: "2px solid transparent",
              borderImage: "linear-gradient(to right, #1FF2AB, #24A965) 1",
            }}
          >
            <Plus className="size-4" />
          </button>
          {sessions.slice(0, 5).map((session) => (
            <button
              type="button"
              key={session.id}
              onClick={() => onSessionSelect(session.id)}
              className={`p-2 rounded-lg transition-all ${
                activeSessionId === session.id
                  ? "bg-gradient-to-r from-accent/20 to-accent-dark/20"
                  : "hover:bg-white/5"
              }`}
              title={session.title}
            >
              <SquareTerminal
                className={`size-5 transition-colors ${
                  activeSessionId === session.id
                    ? "text-accent"
                    : "text-muted-foreground hover:text-accent"
                }`}
              />
            </button>
          ))}
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
          <ChevronRight className="size-4 text-muted-foreground" />
        ) : (
          <ChevronLeft className="size-4 text-muted-foreground" />
        )}
      </button>
    </div>
  );
}
