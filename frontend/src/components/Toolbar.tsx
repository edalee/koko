import { Code2, FileText, Search, Settings, PanelRight } from "lucide-react";

interface ToolbarProps {
  onToggleSidebar: () => void;
  isSidebarOpen: boolean;
}

export default function Toolbar({ onToggleSidebar, isSidebarOpen }: ToolbarProps) {
  return (
    <div
      className="flex items-center justify-between px-4 py-2 bg-toolbar border-b border-border select-none"
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      {/* macOS traffic light spacer */}
      <div className="w-18 shrink-0" />

      {/* Left: branding + nav */}
      <div
        className="flex items-center gap-4"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <div className="flex items-center gap-2">
          <Code2 className="size-5 text-accent" />
          <span className="text-sm text-white">KOKO</span>
        </div>
        <div className="flex items-center gap-1">
          <button className="flex items-center gap-1.5 px-2.5 py-1 text-sm text-white/80 hover:bg-white/10 hover:text-white rounded transition-colors">
            <FileText className="size-4" />
            File
          </button>
          <button className="flex items-center gap-1.5 px-2.5 py-1 text-sm text-white/80 hover:bg-white/10 hover:text-white rounded transition-colors">
            <Search className="size-4" />
            Search
          </button>
          <button className="flex items-center gap-1.5 px-2.5 py-1 text-sm text-white/80 hover:bg-white/10 hover:text-white rounded transition-colors">
            <Settings className="size-4" />
            Settings
          </button>
        </div>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Right: sidebar toggle */}
      <div style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
        <button
          onClick={onToggleSidebar}
          className={`p-1.5 rounded transition-colors ${
            isSidebarOpen
              ? "bg-white/20 text-accent"
              : "text-white/80 hover:bg-white/10 hover:text-white"
          }`}
          title="Toggle sidebar"
        >
          <PanelRight className="size-4" />
        </button>
      </div>
    </div>
  );
}
