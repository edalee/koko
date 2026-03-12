import { GripHorizontal, Terminal, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { CloseSession, CreateShellSession } from "../../wailsjs/go/main/TerminalManager";
import TerminalPane from "./TerminalPane";

interface QuickTerminalProps {
  open: boolean;
  onClose: () => void;
  /** Active session's tab ID — each tab gets its own shell */
  activeTabId: string | null;
  directory: string;
}

const MIN_HEIGHT_PX = 120;
const DEFAULT_HEIGHT_PCT = 35;

export default function QuickTerminal({
  open,
  onClose,
  activeTabId,
  directory,
}: QuickTerminalProps) {
  // Map of tabId → shell session ID
  const [shells, setShells] = useState<Record<string, string>>({});
  const [closing, setClosing] = useState(false);
  const [heightPct, setHeightPct] = useState(DEFAULT_HEIGHT_PCT);
  const dragRef = useRef<{ startY: number; startPct: number } | null>(null);
  const creatingRef = useRef<Set<string>>(new Set());

  const activeShellId = activeTabId ? (shells[activeTabId] ?? null) : null;

  // Create shell for active tab when opened and no shell exists
  useEffect(() => {
    if (!open || !activeTabId || activeShellId || creatingRef.current.has(activeTabId)) return;

    creatingRef.current.add(activeTabId);
    const cols = Math.floor((window.innerWidth * 0.66) / 8);
    const rows = Math.floor((window.innerHeight * (heightPct / 100)) / 16);
    const tabId = activeTabId;

    CreateShellSession(directory || ".", cols, rows).then((sessionId) => {
      setShells((prev) => ({ ...prev, [tabId]: sessionId }));
      creatingRef.current.delete(tabId);
    });
  }, [open, activeTabId, activeShellId, directory, heightPct]);

  // Kill shell for active tab
  const handleKill = useCallback(() => {
    if (!activeTabId || !activeShellId) return;
    setClosing(true);
    const tabId = activeTabId;
    const shellId = activeShellId;
    setTimeout(() => {
      CloseSession(shellId);
      setShells((prev) => {
        const next = { ...prev };
        delete next[tabId];
        return next;
      });
      setClosing(false);
      onClose();
    }, 150);
  }, [activeTabId, activeShellId, onClose]);

  // Handle shell process exiting on its own
  const handleSessionExit = useCallback(
    (tabId: string) => {
      setShells((prev) => {
        const next = { ...prev };
        delete next[tabId];
        return next;
      });
      // If this was the active tab's shell, close the panel
      if (tabId === activeTabId) {
        onClose();
      }
    },
    [activeTabId, onClose],
  );

  // Clean up all shells on unmount
  useEffect(() => {
    return () => {
      for (const shellId of Object.values(shells)) {
        CloseSession(shellId);
      }
    };
    // Only on unmount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Drag to resize height
  const handleDragStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragRef.current = { startY: e.clientY, startPct: heightPct };

      const handleMove = (ev: MouseEvent) => {
        if (!dragRef.current) return;
        const dy = dragRef.current.startY - ev.clientY;
        const dVh = (dy / window.innerHeight) * 100;
        const newPct = Math.max(
          (MIN_HEIGHT_PX / window.innerHeight) * 100,
          Math.min(70, dragRef.current.startPct + dVh),
        );
        setHeightPct(newPct);
      };

      const handleUp = () => {
        dragRef.current = null;
        window.removeEventListener("mousemove", handleMove);
        window.removeEventListener("mouseup", handleUp);
      };

      window.addEventListener("mousemove", handleMove);
      window.addEventListener("mouseup", handleUp);
    },
    [heightPct],
  );

  if (!open && !closing) return null;

  return (
    <div
      className="shrink-0 flex flex-col border-t border-border"
      style={{
        height: `${heightPct}vh`,
        animation: closing
          ? "slide-down 150ms cubic-bezier(0.16, 1, 0.3, 1) forwards"
          : "slide-up 200ms cubic-bezier(0.16, 1, 0.3, 1) forwards",
      }}
    >
      {/* Drag handle to resize */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: drag handle */}
      <div
        className="flex items-center justify-center h-2 cursor-row-resize shrink-0 group hover:bg-accent/10 transition-colors"
        onMouseDown={handleDragStart}
      >
        <div className="w-8 h-0.5 rounded-full bg-white/20 group-hover:bg-accent/50 transition-colors" />
      </div>

      {/* Header */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-white/[0.04] glass-toolbar border-b border-border shrink-0">
        <div className="flex items-center gap-2">
          <GripHorizontal className="size-3 text-tertiary" />
          <Terminal className="size-3.5 text-accent" />
          <span className="text-xs text-white">Terminal</span>
          {directory && (
            <span className="text-[10px] text-tertiary truncate max-w-48">{directory}</span>
          )}
        </div>
        <div className="flex items-center gap-0.5">
          <kbd className="text-[10px] text-tertiary font-mono px-1 mr-1">⌘`</kbd>
          <button
            type="button"
            onClick={handleKill}
            className="p-1 hover:bg-white/10 rounded transition-colors"
            title="Kill terminal"
          >
            <X className="size-3.5 text-muted-foreground hover:text-[#F14D4C]" />
          </button>
        </div>
      </div>

      {/* Terminal panes — one per tab, show/hide to preserve state */}
      <div className="flex-1 bg-base overflow-hidden relative">
        {Object.entries(shells).map(([tabId, shellId]) => (
          <div
            key={shellId}
            className="absolute inset-0"
            style={{ display: tabId === activeTabId ? "block" : "none" }}
          >
            <TerminalPane
              sessionId={shellId}
              active={open && !closing && tabId === activeTabId}
              onExit={() => handleSessionExit(tabId)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
