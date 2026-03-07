import { GitPullRequest, Mail, MessageSquare, Settings } from "lucide-react";
import kokoLogo from "../assets/koko_logo.svg";
import type { OverlayModule } from "../hooks/useOverlay";
import { cn } from "../lib/utils";
import NotificationBadge from "./NotificationBadge";

interface ToolbarProps {
  activeOverlay: OverlayModule | null;
  onToggleOverlay: (module: OverlayModule) => void;
  githubCount: number;
  slackCount: number;
  mailCount: number;
}

export default function Toolbar({
  activeOverlay,
  onToggleOverlay,
  githubCount,
  slackCount,
  mailCount,
}: ToolbarProps) {
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

      {/* Notification icons + Settings */}
      <div
        className="flex items-center gap-1"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <button
          type="button"
          onClick={() => onToggleOverlay("github")}
          className={cn(
            "relative p-2 rounded-lg transition-colors",
            activeOverlay === "github"
              ? "text-accent bg-white/10"
              : "text-muted-foreground hover:text-white hover:bg-white/5",
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
            "relative p-2 rounded-lg transition-colors",
            activeOverlay === "slack"
              ? "text-white bg-white/10"
              : "text-muted-foreground hover:text-white hover:bg-white/5",
          )}
          title="Slack Messages"
        >
          <MessageSquare className="size-4" />
          <NotificationBadge count={slackCount} color="var(--color-badge-slack)" />
        </button>

        <button
          type="button"
          onClick={() => onToggleOverlay("mail")}
          className={cn(
            "relative p-2 rounded-lg transition-colors",
            activeOverlay === "mail"
              ? "text-white bg-white/10"
              : "text-muted-foreground hover:text-white hover:bg-white/5",
          )}
          title="Mail"
        >
          <Mail className="size-4" />
          <NotificationBadge count={mailCount} color="var(--color-badge-mail)" />
        </button>

        <div className="w-px h-4 bg-white/10 mx-1" />

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
