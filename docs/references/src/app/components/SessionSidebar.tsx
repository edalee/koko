import { Plus, X, Search, Sparkles, ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Session } from "../types";

type SessionSidebarProps = {
  sessions: Session[];
  activeSession: Session | null;
  onSessionSelect: (session: Session) => void;
  onNewSession: () => void;
  onDeleteSession: (sessionId: string) => void;
  isCollapsed: boolean;
  onToggleCollapse: () => void;
};

export function SessionSidebar({
  sessions,
  activeSession,
  onSessionSelect,
  onNewSession,
  onDeleteSession,
  isCollapsed,
  onToggleCollapse,
}: SessionSidebarProps) {
  return (
    <div className="h-full bg-[#1e1e1e] flex flex-col border-r border-white/10 relative">
      {!isCollapsed && (
        <>
          <div className="p-3">
            <div className="relative mb-3">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-gray-400" />
              <Input
                placeholder="Search sessions..."
                className="pl-9 bg-white/5 border-white/10 text-white placeholder:text-gray-500"
              />
            </div>
            <Button
              onClick={onNewSession}
              className="w-full bg-[#1e1e1e] text-white hover:bg-[#252526] border-2 border-transparent bg-clip-padding relative overflow-hidden group"
              style={{
                borderImage: 'linear-gradient(to right, #1FF2AB, #24A965) 1',
              }}
            >
              <div className="absolute inset-0 bg-gradient-to-r from-[#1FF2AB]/10 to-[#24A965]/10 opacity-0 group-hover:opacity-100 transition-opacity" />
              <Plus className="size-4 mr-2 relative z-10" />
              <span className="relative z-10">New Session</span>
            </Button>
          </div>

          <div className="flex-1 overflow-auto px-2">
            {sessions.map((session) => (
              <div
                key={session.id}
                className={`group flex items-start gap-3 p-3 mb-2 rounded-lg cursor-pointer transition-all ${
                  activeSession?.id === session.id
                    ? "bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 border border-[#1FF2AB]/30"
                    : "hover:bg-white/5"
                }`}
                onClick={() => onSessionSelect(session)}
              >
                <Sparkles className={`size-5 mt-0.5 shrink-0 transition-colors ${
                  activeSession?.id === session.id
                    ? "text-[#1FF2AB]"
                    : "text-gray-400 group-hover:text-[#1FF2AB]"
                }`} />
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm text-white truncate mb-1">{session.title}</h3>
                  <p className="text-xs text-gray-400 truncate">
                    {session.files.length} files • {session.chatSessions.length} chats
                  </p>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeleteSession(session.id);
                  }}
                  className="p-1 hover:bg-white/10 rounded transition-opacity"
                >
                  <X className="size-4 text-gray-400 hover:text-[#F14D4C]" />
                </button>
              </div>
            ))}
          </div>
        </>
      )}
      
      {isCollapsed && (
        <div className="flex-1 flex flex-col items-center py-4 gap-3">
          <Button
            onClick={onNewSession}
            size="sm"
            className="bg-[#1e1e1e] hover:bg-[#252526] text-gray-400 hover:text-[#1FF2AB] p-2 border-2 border-transparent"
            style={{
              borderImage: 'linear-gradient(to right, #1FF2AB, #24A965) 1',
            }}
          >
            <Plus className="size-4" />
          </Button>
          {sessions.slice(0, 5).map((session) => (
            <button
              key={session.id}
              onClick={() => onSessionSelect(session)}
              className={`p-2 rounded-lg transition-all ${
                activeSession?.id === session.id
                  ? "bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20"
                  : "hover:bg-white/5"
              }`}
              title={session.title}
            >
              <Sparkles className={`size-5 transition-colors ${
                activeSession?.id === session.id
                  ? "text-[#1FF2AB]"
                  : "text-gray-400 hover:text-[#1FF2AB]"
              }`} />
            </button>
          ))}
        </div>
      )}

      <button
        onClick={onToggleCollapse}
        className="absolute bottom-3 left-3 p-1.5 hover:bg-white/10 rounded transition-colors"
        title={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      >
        {isCollapsed ? (
          <ChevronRight className="size-4 text-gray-400" />
        ) : (
          <ChevronLeft className="size-4 text-gray-400" />
        )}
      </button>
    </div>
  );
}