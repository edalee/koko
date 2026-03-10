import { Terminal, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { CloseSession, CreateShellSession } from "../../wailsjs/go/main/TerminalManager";
import TerminalPane from "./TerminalPane";

interface QuickTerminalProps {
  open: boolean;
  onClose: () => void;
  directory: string;
}

export default function QuickTerminal({ open, onClose, directory }: QuickTerminalProps) {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [closing, setClosing] = useState(false);
  const prevOpenRef = useRef(false);

  // Create shell session when opened
  useEffect(() => {
    if (open && !prevOpenRef.current && !sessionId) {
      const cols = Math.floor((window.innerWidth * 0.66) / 8);
      const rows = Math.floor((window.innerHeight * 0.35) / 16);
      CreateShellSession(directory || ".", cols, rows).then(setSessionId);
    }
    prevOpenRef.current = open;
  }, [open, sessionId, directory]);

  const handleClose = useCallback(() => {
    setClosing(true);
    setTimeout(() => {
      if (sessionId) {
        CloseSession(sessionId);
        setSessionId(null);
      }
      setClosing(false);
      onClose();
    }, 150);
  }, [sessionId, onClose]);

  const handleSessionExit = useCallback(() => {
    setSessionId(null);
    onClose();
  }, [onClose]);

  if (!open && !closing) return null;

  return (
    <div
      className="absolute bottom-0 left-0 right-0 z-30 flex flex-col border-t border-border"
      style={{
        height: "35%",
        animation: closing
          ? "slide-down 150ms cubic-bezier(0.16, 1, 0.3, 1) forwards"
          : "slide-up 200ms cubic-bezier(0.16, 1, 0.3, 1) forwards",
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-white/[0.04] glass-toolbar border-b border-border shrink-0">
        <div className="flex items-center gap-2">
          <Terminal className="size-3.5 text-accent" />
          <span className="text-xs text-white">Terminal</span>
          {directory && (
            <span className="text-[10px] text-tertiary truncate max-w-48">{directory}</span>
          )}
        </div>
        <div className="flex items-center gap-0.5">
          <button
            type="button"
            onClick={handleClose}
            className="p-1 hover:bg-white/10 rounded transition-colors"
            title="Close terminal (⌘`)"
          >
            <X className="size-3.5 text-muted-foreground hover:text-[#F14D4C]" />
          </button>
        </div>
      </div>

      {/* Terminal */}
      <div className="flex-1 bg-base overflow-hidden">
        {sessionId && (
          <TerminalPane
            sessionId={sessionId}
            active={open && !closing}
            onExit={handleSessionExit}
          />
        )}
      </div>
    </div>
  );
}
