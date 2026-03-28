import { ArrowUpCircle, Settings, X } from "lucide-react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import kokoLogo from "../assets/koko_logo.svg";
import type { OverlayModule } from "../hooks/useOverlay";
import type { UpdateInfo } from "../hooks/useUpdateCheck";
import { cn } from "../lib/utils";

interface ToolbarProps {
  activeOverlay: OverlayModule | null;
  onToggleOverlay: (module: OverlayModule) => void;
  update: UpdateInfo | null;
  onDismissUpdate: () => void;
}

export default function Toolbar({
  activeOverlay,
  onToggleOverlay,
  update,
  onDismissUpdate,
}: ToolbarProps) {
  return (
    <div
      className="flex items-center justify-between px-4 py-2 bg-toolbar glass-toolbar border-b border-border select-none"
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      {/* macOS traffic light spacer */}
      <div className="w-18 shrink-0" />

      {/* Left: logo */}
      <div className="flex items-center gap-2">
        <img src={kokoLogo} alt="Koko" className="h-4 w-auto" />
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Update notification */}
      {update && (
        <div
          className="flex items-center gap-2 px-2.5 py-1 mr-2 rounded-lg bg-accent/10 border border-accent/20"
          style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
        >
          <ArrowUpCircle className="size-3.5 text-accent" />
          <button
            type="button"
            onClick={() => BrowserOpenURL(update.url)}
            className="text-[11px] text-accent hover:underline"
          >
            v{update.version} available
          </button>
          <button
            type="button"
            onClick={onDismissUpdate}
            className="p-0.5 text-accent/50 hover:text-accent transition-colors"
          >
            <X className="size-3" />
          </button>
        </div>
      )}

      {/* Settings */}
      <div
        className="flex items-center gap-1"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <button
          type="button"
          onClick={() => onToggleOverlay("settings")}
          className={cn(
            "relative flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors",
            activeOverlay === "settings"
              ? "text-accent bg-white/10"
              : "text-white/70 hover:text-white hover:bg-white/5",
          )}
          title="Settings"
        >
          <Settings className="size-4" />
        </button>
      </div>
    </div>
  );
}
