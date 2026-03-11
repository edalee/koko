import { MessageSquare, Pencil, Sparkles } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetContextInfo } from "../../wailsjs/go/main/App";
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
  const [contextPct, setContextPct] = useState<number | null>(null);
  const [model, setModel] = useState<string | null>(null);

  const sendShiftTab = useCallback(() => {
    Write(sessionId, btoa("\x1b[Z"));
  }, [sessionId]);

  const switchTo = useCallback(
    (target: ClaudeMode) => {
      if (target === mode) return;
      const currentIdx = MODE_ORDER.indexOf(mode);
      const targetIdx = MODE_ORDER.indexOf(target);
      const steps = (targetIdx - currentIdx + MODE_ORDER.length) % MODE_ORDER.length;
      for (let i = 0; i < steps; i++) {
        sendShiftTab();
      }
      setMode(target);
    },
    [mode, sendShiftTab],
  );

  // Poll context info every 5s
  useEffect(() => {
    let active = true;
    async function poll() {
      try {
        const info = await GetContextInfo(sessionId);
        if (active && info) {
          setContextPct(info.usedPercentage);
          if (info.model) setModel(info.model);
        }
      } catch {
        // No data yet — Claude hasn't responded
      }
    }
    poll();
    const id = setInterval(poll, 5000);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, [sessionId]);

  const contextColor =
    contextPct === null
      ? "text-tertiary"
      : contextPct >= 80
        ? "text-red-400"
        : contextPct >= 60
          ? "text-yellow-400"
          : "text-muted-foreground";

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

      <div className="flex items-center gap-2 ml-auto">
        {contextPct !== null && (
          <div className="flex items-center gap-1.5">
            {/* Mini progress bar */}
            <div className="w-12 h-1 rounded-full bg-white/[0.06] overflow-hidden">
              <div
                className={cn(
                  "h-full rounded-full transition-all",
                  contextPct >= 80
                    ? "bg-red-400"
                    : contextPct >= 60
                      ? "bg-yellow-400"
                      : "bg-accent",
                )}
                style={{ width: `${contextPct}%` }}
              />
            </div>
            <span className={cn("text-[10px] tabular-nums", contextColor)}>{contextPct}%</span>
          </div>
        )}
        {model && <span className="text-[10px] text-tertiary">{model}</span>}
      </div>
    </div>
  );
}
