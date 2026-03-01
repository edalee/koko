import { Plus, X } from "lucide-react";
import { cn } from "../lib/utils";
import type { SessionTab } from "../types";

interface SessionTabBarProps {
  tabs: SessionTab[];
  activeTabId: string | null;
  onSwitch: (tabId: string) => void;
  onClose: (tabId: string) => void;
  onCreate: () => void;
}

export default function SessionTabBar({
  tabs,
  activeTabId,
  onSwitch,
  onClose,
  onCreate,
}: SessionTabBarProps) {
  return (
    <div
      className="flex h-9 shrink-0 items-center gap-1 px-2 overflow-x-auto"
      style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
    >
      {tabs.map((tab) => (
        <button
          key={tab.id}
          className={cn(
            "group relative flex h-8 min-w-32 items-center justify-center px-8 text-[13px] transition-colors",
            tab.id === activeTabId
              ? "text-foreground"
              : "text-muted-foreground hover:bg-tab-hover hover:text-foreground",
          )}
          style={
            tab.id === activeTabId
              ? { background: 'linear-gradient(135deg, oklch(0.40 0.12 350), oklch(0.28 0.06 260))' }
              : undefined
          }
          onClick={() => onSwitch(tab.id)}
        >
          <span className="truncate max-w-32">{tab.title}</span>
          <span
            className={cn(
              "absolute right-1.5 flex h-4 w-4 items-center justify-center transition-opacity",
              tab.id === activeTabId
                ? "opacity-60 hover:opacity-100"
                : "opacity-0 group-hover:opacity-60 hover:!opacity-100",
            )}
            onClick={(e) => {
              e.stopPropagation();
              onClose(tab.id);
            }}
          >
            <X className="h-3 w-3" />
          </span>
        </button>
      ))}
      <button
        className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-tab-hover hover:text-foreground"
        onClick={onCreate}
        title="New session"
      >
        <Plus className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}
