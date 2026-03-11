import { MessageSquare, Pencil, Sparkles } from "lucide-react";
import { useCallback, useState } from "react";
import { Write } from "../../wailsjs/go/main/TerminalManager";
import { cn } from "../lib/utils";

type ClaudeMode = "ask" | "auto" | "plan";

const MODES: { key: ClaudeMode; label: string; icon: typeof MessageSquare }[] = [
  { key: "ask", label: "Ask", icon: MessageSquare },
  { key: "auto", label: "Auto-edit", icon: Pencil },
  { key: "plan", label: "Plan", icon: Sparkles },
];

const MODE_ORDER: ClaudeMode[] = ["ask", "auto", "plan"];

interface ClaudeModeSwitcherProps {
  sessionId: string;
}

export default function ClaudeModeSwitcher({ sessionId }: ClaudeModeSwitcherProps) {
  const [mode, setMode] = useState<ClaudeMode>("ask");

  const sendShiftTab = useCallback(() => {
    // Shift+Tab escape sequence
    Write(sessionId, btoa("\x1b[Z"));
  }, [sessionId]);

  const switchTo = useCallback(
    (target: ClaudeMode) => {
      if (target === mode) return;

      const currentIdx = MODE_ORDER.indexOf(mode);
      const targetIdx = MODE_ORDER.indexOf(target);
      // Calculate how many Shift+Tabs needed (cycling forward)
      const steps = (targetIdx - currentIdx + MODE_ORDER.length) % MODE_ORDER.length;

      for (let i = 0; i < steps; i++) {
        sendShiftTab();
      }
      setMode(target);
    },
    [mode, sendShiftTab],
  );

  return (
    <div className="flex items-center gap-1 px-2 py-1.5 bg-base border-t border-white/[0.06]">
      {MODES.map(({ key, label, icon: Icon }) => (
        <button
          key={key}
          type="button"
          onClick={() => switchTo(key)}
          className={cn(
            "flex items-center gap-1.5 px-2.5 py-1 text-[11px] rounded-md transition-colors",
            mode === key
              ? "bg-white/[0.08] text-white font-medium"
              : "text-muted-foreground hover:text-white hover:bg-white/[0.04]",
          )}
        >
          <Icon className="size-3" />
          {label}
        </button>
      ))}
      <span className="text-[10px] text-tertiary ml-auto">⇧⇥ to cycle</span>
    </div>
  );
}
