import { Settings } from "lucide-react";
import kokoLogo from "../assets/koko_logo.svg";

export default function Toolbar() {
  return (
    <div
      className="flex items-center justify-between px-4 py-2 bg-toolbar border-b border-border select-none"
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

      {/* Right: Settings */}
      <div style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
        <button
          type="button"
          className="flex items-center gap-1.5 px-2.5 py-1 text-sm text-white/80 hover:bg-white/10 hover:text-white rounded transition-colors"
        >
          <Settings className="size-4" />
          Settings
        </button>
      </div>
    </div>
  );
}
