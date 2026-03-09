import { useEffect } from "react";

interface KeyboardShortcutActions {
  onNewSession: () => void;
  onSwitchSession: (index: number) => void;
  onCloseSession: () => void;
  onToggleTerminal: () => void;
}

export function useKeyboardShortcuts({
  onNewSession,
  onSwitchSession,
  onCloseSession,
  onToggleTerminal,
}: KeyboardShortcutActions) {
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      // Only handle Cmd (Mac) / Ctrl shortcuts
      if (!e.metaKey && !e.ctrlKey) return;

      // Cmd+N — New session
      if (e.key === "n" && !e.shiftKey) {
        e.preventDefault();
        onNewSession();
        return;
      }

      // Cmd+W — Close active session
      if (e.key === "w" && !e.shiftKey) {
        e.preventDefault();
        onCloseSession();
        return;
      }

      // Cmd+` — Toggle quick terminal
      if (e.key === "`" && !e.shiftKey) {
        e.preventDefault();
        onToggleTerminal();
        return;
      }

      // Cmd+1 through Cmd+9 — Switch to session by index
      const num = Number.parseInt(e.key, 10);
      if (num >= 1 && num <= 9) {
        e.preventDefault();
        onSwitchSession(num - 1);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onNewSession, onSwitchSession, onCloseSession, onToggleTerminal]);
}
