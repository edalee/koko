import { ReactNode } from "react";
import { Search, Settings } from "lucide-react";

interface TitleBarProps {
  children?: ReactNode;
  sessionCount?: number;
}

export default function TitleBar({ children, sessionCount = 0 }: TitleBarProps) {
  return (
    <div
      className="flex h-11 shrink-0 items-center bg-titlebar border-b border-border/20 select-none"
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      {/* macOS traffic light spacer */}
      <div className="w-18 shrink-0" />

      {/* Branding */}
      <div
        className="flex items-center gap-1.5 pr-4 border-r border-border/20 mr-2"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <span className="text-sm font-semibold tracking-wide text-foreground">KOKO</span>
        {sessionCount > 0 && (
          <span className="text-[10px] text-muted-foreground">{sessionCount}</span>
        )}
      </div>

      {/* Session tabs */}
      <div className="flex-1 min-w-0">
        {children}
      </div>

      {/* Right actions */}
      <div
        className="flex items-center gap-1 px-3"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <button className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-tab-hover hover:text-foreground">
          <Search className="h-3.5 w-3.5" />
        </button>
        <button className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-tab-hover hover:text-foreground">
          <Settings className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}
