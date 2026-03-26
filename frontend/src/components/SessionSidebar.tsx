import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Folder,
  Plus,
  Search,
  SquareTerminal,
  X,
} from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";
import type { SessionState } from "../hooks/useSessionActivity";
import type { SessionTab } from "../types";

function dirBasename(path: string): string {
  const parts = path.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || path;
}

interface SessionSidebarProps {
  sessions: SessionTab[];
  activeSessionId: string | null;
  sessionStates: Map<string, SessionState>;
  onSessionSelect: (sessionId: string) => void;
  onNewSession: () => void;
  onDeleteSession: (sessionId: string) => void;
  onRenameSession: (sessionId: string, newName: string) => void;
  isCollapsed: boolean;
  onToggleCollapse: () => void;
}

interface DirGroup {
  directory: string;
  dirName: string;
  sessions: SessionTab[];
}

export default function SessionSidebar({
  sessions,
  activeSessionId,
  sessionStates,
  onSessionSelect,
  onNewSession,
  onDeleteSession,
  onRenameSession,
  isCollapsed,
  onToggleCollapse,
}: SessionSidebarProps) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [collapsedDirs, setCollapsedDirs] = useState<Set<string>>(new Set());
  const inputRef = useRef<HTMLInputElement>(null);

  const filteredSessions = searchQuery
    ? sessions.filter(
        (s) =>
          s.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          s.directory.toLowerCase().includes(searchQuery.toLowerCase()),
      )
    : sessions;

  // Group sessions by directory
  const groups: DirGroup[] = useMemo(() => {
    const map = new Map<string, SessionTab[]>();
    for (const s of filteredSessions) {
      const existing = map.get(s.directory) || [];
      existing.push(s);
      map.set(s.directory, existing);
    }
    return Array.from(map.entries()).map(([dir, sess]) => ({
      directory: dir,
      dirName: dirBasename(dir),
      sessions: sess,
    }));
  }, [filteredSessions]);

  const toggleDirCollapsed = useCallback((dir: string) => {
    setCollapsedDirs((prev) => {
      const next = new Set(prev);
      if (next.has(dir)) {
        next.delete(dir);
      } else {
        next.add(dir);
      }
      return next;
    });
  }, []);

  const startRename = useCallback((session: SessionTab) => {
    setEditingId(session.id);
    setEditValue(session.name);
    setTimeout(() => inputRef.current?.select(), 0);
  }, []);

  const commitRename = useCallback(() => {
    if (editingId && editValue.trim()) {
      onRenameSession(editingId, editValue.trim());
    }
    setEditingId(null);
  }, [editingId, editValue, onRenameSession]);

  // Flat index for keyboard shortcuts (⌘1-9)
  const flatIndex = useMemo(() => {
    const map = new Map<string, number>();
    let idx = 0;
    for (const s of sessions) {
      map.set(s.id, idx);
      idx++;
    }
    return map;
  }, [sessions]);

  return (
    <div
      className="h-full bg-white/[0.03] glass-panel flex flex-col border-r relative"
      style={{
        borderImage: "linear-gradient(to bottom, rgba(255,255,255,0.15), rgba(255,255,255,0.04)) 1",
      }}
    >
      {!isCollapsed && (
        <>
          {/* Search + New Session */}
          <div className="px-3 pt-3 pb-3 space-y-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
              <input
                placeholder="Search sessions..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-9 pr-3 py-2 text-sm bg-white/[0.04] border border-white/[0.06] rounded-lg text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
              />
            </div>

            <button
              type="button"
              onClick={onNewSession}
              className="w-full flex items-center justify-center gap-2 px-3 py-2.5 rounded text-white whitespace-nowrap relative overflow-hidden group transition-colors border-2 border-transparent"
              style={{
                background:
                  "linear-gradient(rgba(255,255,255,0.06), rgba(255,255,255,0.06)) padding-box, linear-gradient(to right, #1FF2AB, #24A965) border-box",
              }}
            >
              <div className="absolute inset-0 bg-gradient-to-r from-accent/10 to-accent-dark/10 opacity-0 group-hover:opacity-100 transition-opacity" />
              <Plus className="size-4 relative z-10" />
              <span className="text-sm font-medium relative z-10">New Session</span>
              <kbd className="text-[10px] text-white/40 font-mono relative z-10 ml-auto">⌘N</kbd>
            </button>
          </div>

          {/* Grouped session list */}
          <div className="flex-1 overflow-auto px-2 pt-1 pb-10">
            <div className="space-y-1">
              {groups.map((group) => {
                const isMulti = group.sessions.length > 1;
                const isDirCollapsed = isMulti && collapsedDirs.has(group.directory);
                const hasApproval = group.sessions.some(
                  (s) => s.connected && sessionStates.get(s.id) === "approval",
                );
                const activeCount = group.sessions.filter((s) => s.connected).length;

                return (
                  <div key={group.directory}>
                    {/* Directory header — only shown for multi-session directories */}
                    {isMulti && (
                      // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: directory group toggle
                      <div
                        className="flex items-center gap-2 px-2 py-1.5 rounded-lg cursor-pointer hover:bg-white/[0.04] transition-colors"
                        onClick={() => toggleDirCollapsed(group.directory)}
                      >
                        <ChevronDown
                          className={`size-3 text-tertiary transition-transform ${isDirCollapsed ? "-rotate-90" : ""}`}
                        />
                        <Folder className="size-3.5 text-muted-foreground" />
                        <span className="text-xs text-white/70 font-medium truncate flex-1">
                          {group.dirName}
                        </span>
                        {isDirCollapsed && (
                          <span className="text-[10px] text-tertiary tabular-nums">
                            {group.sessions.length}
                          </span>
                        )}
                        {hasApproval && (
                          <span className="size-2 rounded-full bg-amber-400 animate-pulse" />
                        )}
                        {!hasApproval && activeCount > 0 && (
                          <span className="size-2 rounded-full bg-accent/60" />
                        )}
                      </div>
                    )}

                    {/* Sessions — indented under folder header when multi, flat otherwise */}
                    {!isDirCollapsed &&
                      group.sessions.map((session) => {
                        const isActive = activeSessionId === session.id;
                        const isEditing = editingId === session.id;
                        const state = session.connected
                          ? (sessionStates.get(session.id) ?? "active")
                          : "disconnected";
                        const shortcutIdx = flatIndex.get(session.id);
                        const shortcutKey =
                          shortcutIdx !== undefined && shortcutIdx < 9 ? shortcutIdx + 1 : null;

                        return (
                          // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: session item
                          <div
                            key={session.id}
                            className={`group flex items-center gap-2.5 ${isMulti ? "ml-4" : ""} pl-3 pr-2 py-2 rounded-lg cursor-pointer transition-all ${
                              isActive
                                ? "bg-white/[0.08] border border-accent/20 glow-accent"
                                : state === "approval"
                                  ? "hover:bg-white/[0.05] border border-amber-400/30"
                                  : "hover:bg-white/[0.05] border border-transparent"
                            }`}
                            onClick={() => onSessionSelect(session.id)}
                          >
                            {/* Status dot */}
                            <span
                              className={`size-2 rounded-full shrink-0 ${
                                state === "approval"
                                  ? "bg-amber-400 animate-pulse"
                                  : state === "disconnected"
                                    ? "bg-white/20"
                                    : "bg-accent/70"
                              }`}
                            />

                            {/* Name */}
                            <div className="flex-1 min-w-0">
                              {isEditing ? (
                                <input
                                  ref={inputRef}
                                  value={editValue}
                                  onChange={(e) => setEditValue(e.target.value)}
                                  onBlur={commitRename}
                                  onKeyDown={(e) => {
                                    if (e.key === "Enter") commitRename();
                                    if (e.key === "Escape") setEditingId(null);
                                    e.stopPropagation();
                                  }}
                                  onClick={(e) => e.stopPropagation()}
                                  className="text-sm text-white bg-white/[0.08] border border-accent/20 rounded px-1.5 py-0.5 w-full outline-none"
                                />
                              ) : (
                                // biome-ignore lint/a11y/noStaticElementInteractions: double-click rename
                                <span
                                  className={`text-sm truncate block ${
                                    state === "disconnected" ? "text-white/50" : "text-white"
                                  }`}
                                  onDoubleClick={(e) => {
                                    e.stopPropagation();
                                    startRename(session);
                                  }}
                                >
                                  {session.name}
                                </span>
                              )}
                              {!session.connected && (
                                <span className="text-[10px] text-tertiary">Click to resume</span>
                              )}
                            </div>

                            {/* Actions */}
                            <div className="flex items-center gap-1 shrink-0">
                              {shortcutKey && (
                                <kbd className="text-[10px] text-tertiary font-mono px-1 rounded bg-white/[0.04] border border-white/[0.06] leading-relaxed opacity-0 group-hover:opacity-100 transition-opacity">
                                  ⌘{shortcutKey}
                                </kbd>
                              )}
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onDeleteSession(session.id);
                                }}
                                className="p-0.5 hover:bg-white/10 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                                title="Close session"
                              >
                                <X className="size-3.5 text-muted-foreground hover:text-[#F14D4C]" />
                              </button>
                            </div>
                          </div>
                        );
                      })}
                  </div>
                );
              })}

              {filteredSessions.length === 0 && (
                <div className="text-center py-8">
                  <SquareTerminal className="size-8 text-tertiary mx-auto mb-2" />
                  <p className="text-xs text-tertiary">
                    {searchQuery ? "No matching sessions" : "No sessions"}
                  </p>
                </div>
              )}
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
            className="p-2 rounded-md text-muted-foreground hover:text-accent transition-colors border-2 border-transparent"
            style={{
              background:
                "linear-gradient(rgba(255,255,255,0.06), rgba(255,255,255,0.06)) padding-box, linear-gradient(to right, #1FF2AB, #24A965) border-box",
            }}
          >
            <Plus className="size-4" />
          </button>
          {sessions.slice(0, 5).map((session) => {
            const state = session.connected
              ? (sessionStates.get(session.id) ?? "active")
              : "disconnected";
            return (
              <button
                type="button"
                key={session.id}
                onClick={() => onSessionSelect(session.id)}
                className={`relative p-2 rounded-xl transition-all ${
                  activeSessionId === session.id
                    ? "bg-white/[0.08] glow-accent"
                    : "hover:bg-white/[0.05]"
                }`}
                title={state === "approval" ? `${session.name} — needs approval` : session.name}
              >
                <SquareTerminal
                  className={`size-5 transition-colors ${
                    state === "approval"
                      ? "text-amber-400 animate-pulse"
                      : activeSessionId === session.id
                        ? "text-accent"
                        : "text-muted-foreground hover:text-accent"
                  }`}
                />
                {state === "approval" && (
                  <span className="absolute top-1 right-1 size-2 rounded-full bg-amber-400 animate-ping" />
                )}
              </button>
            );
          })}
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
