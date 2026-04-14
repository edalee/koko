import { ArrowUpCircle, Maximize2, Minus, Settings, X } from "lucide-react";
import {
  BrowserOpenURL,
  Quit,
  ScreenGetAll,
  WindowGetPosition,
  WindowGetSize,
  WindowMinimise,
  WindowSetPosition,
  WindowSetSize,
} from "../../wailsjs/runtime/runtime";
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

const drag = { "--wails-draggable": "drag" } as React.CSSProperties;
const noDrag = { "--wails-draggable": "no-drag" } as React.CSSProperties;

// Manual maximize: instant position+size instead of macOS two-step animation
let savedBounds: { x: number; y: number; w: number; h: number } | null = null;

async function toggleMaximise() {
  if (savedBounds) {
    // Restore
    const { x, y, w, h } = savedBounds;
    savedBounds = null;
    WindowSetPosition(x, y);
    WindowSetSize(w, h);
  } else {
    // Save current bounds, then fill screen
    const [pos, size, screens] = await Promise.all([
      WindowGetPosition(),
      WindowGetSize(),
      ScreenGetAll(),
    ]);
    savedBounds = { x: pos.x, y: pos.y, w: size.w, h: size.h };
    const screen = screens.find((s) => s.isCurrent) || screens[0];
    WindowSetPosition(0, 0);
    WindowSetSize(screen.width, screen.height);
  }
}

export default function Toolbar({
  activeOverlay,
  onToggleOverlay,
  update,
  onDismissUpdate,
}: ToolbarProps) {
  return (
    <div
      className="relative flex items-center px-4 py-2 bg-toolbar glass-toolbar border-b border-border select-none"
      style={drag}
    >
      {/* Window controls — absolutely positioned, hover to reveal */}
      <div
        className="absolute left-0 top-0 bottom-0 z-10 flex items-center pl-4 pr-6 group/wc"
        style={noDrag}
      >
        <div className="flex items-center gap-2 opacity-0 group-hover/wc:opacity-100 transition-opacity duration-100">
          <button
            type="button"
            onClick={Quit}
            className="size-3.5 rounded-full bg-[#FF5F57] flex items-center justify-center hover:brightness-110"
            title="Close"
          >
            <X className="size-2.5 text-black/80 stroke-[3]" />
          </button>
          <button
            type="button"
            onClick={WindowMinimise}
            className="size-3.5 rounded-full bg-[#FEBC2E] flex items-center justify-center hover:brightness-110"
            title="Minimise"
          >
            <Minus className="size-2.5 text-black/80 stroke-[3]" />
          </button>
          <button
            type="button"
            onClick={toggleMaximise}
            className="size-3.5 rounded-full bg-[#28C840] flex items-center justify-center hover:brightness-110"
            title="Maximise"
          >
            <Maximize2 className="size-2 text-black/80 stroke-[3]" />
          </button>
        </div>
      </div>

      {/* Logo — centered */}
      <div className="flex-1 flex justify-center">
        <img src={kokoLogo} alt="Koko" className="h-4 w-auto" />
      </div>

      {/* Right side */}
      <div className="flex items-center gap-2 shrink-0" style={noDrag}>
        {/* Update notification */}
        {update && (
          <div className="flex items-center gap-2 px-2.5 py-1 mr-1 rounded-lg bg-accent/10 border border-accent/20">
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
