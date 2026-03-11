import { ArrowUpCircle, Bell, GitPullRequest, MessageSquare, Settings, X } from "lucide-react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import kokoLogo from "../assets/koko_logo.svg";
import type { OverlayModule } from "../hooks/useOverlay";
import type { UpdateInfo } from "../hooks/useUpdateCheck";
import { cn } from "../lib/utils";
import NotificationBadge from "./NotificationBadge";

interface ToolbarProps {
  activeOverlay: OverlayModule | null;
  onToggleOverlay: (module: OverlayModule) => void;
  githubCount: number;
  slackCount: number;
  notifCount: number;
  update: UpdateInfo | null;
  onDismissUpdate: () => void;
}

export default function Toolbar({
  activeOverlay,
  onToggleOverlay,
  githubCount,
  slackCount,
  notifCount,
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

      {/* Notification icons + Settings */}
      <div
        className="flex items-center gap-1"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <button
          type="button"
          onClick={() => onToggleOverlay("github")}
          className={cn(
            "relative flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors",
            activeOverlay === "github"
              ? "text-accent bg-white/10"
              : "text-white/70 hover:text-white hover:bg-white/5",
          )}
          title="GitHub PRs"
        >
          <GitPullRequest className="size-4" />
          <NotificationBadge count={githubCount} />
        </button>

        <button
          type="button"
          onClick={() => onToggleOverlay("slack")}
          className={cn(
            "relative flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors",
            activeOverlay === "slack"
              ? "text-accent bg-white/10"
              : "text-white/70 hover:text-white hover:bg-white/5",
          )}
          title="Slack Messages"
        >
          <MessageSquare className="size-4" />
          <NotificationBadge count={slackCount} color="var(--color-badge-slack)" />
        </button>

        <button
          type="button"
          onClick={() => onToggleOverlay("notifications")}
          className={cn(
            "relative flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors",
            activeOverlay === "notifications"
              ? "text-accent bg-white/10"
              : "text-white/70 hover:text-white hover:bg-white/5",
          )}
          title="Notifications"
        >
          <Bell className="size-4" />
          <NotificationBadge count={notifCount} color="var(--color-badge-mail)" />
        </button>

        <div className="w-px h-4 bg-white/10 mx-1" />

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
