import {
  Bell,
  Check,
  Circle,
  GitMerge,
  GitPullRequest,
  Loader2,
  RefreshCw,
  Tag,
} from "lucide-react";
import { useState } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { GitHubNotification, NotifFilter } from "../hooks/useNotifications";
import { cn } from "../lib/utils";

type ViewFilter = "all" | "review" | "mention";

const VIEW_FILTERS: { key: ViewFilter; label: string }[] = [
  { key: "all", label: "All" },
  { key: "review", label: "Review" },
  { key: "mention", label: "Mentioned" },
];

function filterByView(notifications: GitHubNotification[], view: ViewFilter): GitHubNotification[] {
  switch (view) {
    case "review":
      return notifications.filter((n) => n.reason === "review_requested");
    case "mention":
      return notifications.filter((n) => n.reason === "mention" || n.reason === "team_mention");
    case "all":
      return notifications;
  }
}

function typeIcon(type: string) {
  switch (type) {
    case "PullRequest":
      return <GitPullRequest className="size-3.5 text-accent" />;
    case "Release":
      return <Tag className="size-3.5 text-purple-400" />;
    case "CheckSuite":
      return <GitMerge className="size-3.5 text-yellow-400" />;
    default:
      return <Bell className="size-3.5 text-blue-400" />;
  }
}

function reasonLabel(reason: string): string {
  switch (reason) {
    case "review_requested":
      return "review";
    case "mention":
      return "mention";
    case "team_mention":
      return "team";
    case "assign":
      return "assigned";
    case "author":
      return "author";
    case "ci_activity":
      return "CI";
    case "subscribed":
      return "";
    default:
      return reason;
  }
}

function reasonColor(reason: string): string {
  switch (reason) {
    case "review_requested":
      return "bg-accent/15 text-accent";
    case "mention":
    case "team_mention":
      return "bg-blue-400/15 text-blue-400";
    case "assign":
      return "bg-purple-400/15 text-purple-400";
    case "author":
      return "bg-white/10 text-white/70";
    case "ci_activity":
      return "bg-yellow-400/15 text-yellow-400";
    default:
      return "";
  }
}

function timeAgo(isoDate: string): string {
  const now = Date.now();
  const then = new Date(isoDate).getTime();
  const diff = (now - then) / 1000;
  if (diff < 60) return "now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  return `${Math.floor(diff / 86400)}d`;
}

interface NotificationsPanelProps {
  notifications: GitHubNotification[];
  loading: boolean;
  refresh: () => void;
  filter: NotifFilter;
  onFilterChange: (f: NotifFilter) => void;
  onMarkRead: (id: string) => void;
}

export default function NotificationsPanel({
  notifications,
  loading,
  refresh,
  filter,
  onFilterChange,
  onMarkRead,
}: NotificationsPanelProps) {
  const [viewFilter, setViewFilter] = useState<ViewFilter>("all");
  const filtered = filterByView(notifications, viewFilter);

  return (
    <div className="flex flex-col h-full">
      {/* Source filter: Participating vs All */}
      <div className="flex items-center gap-1 px-4 pt-3 pb-1">
        <button
          type="button"
          onClick={() => onFilterChange("participating")}
          className={cn(
            "px-2.5 py-1 text-[11px] rounded-md transition-colors",
            filter === "participating"
              ? "bg-white/[0.08] text-white font-medium"
              : "text-muted-foreground hover:text-white hover:bg-white/[0.04]",
          )}
        >
          Participating
        </button>
        <button
          type="button"
          onClick={() => onFilterChange("all")}
          className={cn(
            "px-2.5 py-1 text-[11px] rounded-md transition-colors",
            filter === "all"
              ? "bg-white/[0.08] text-white font-medium"
              : "text-muted-foreground hover:text-white hover:bg-white/[0.04]",
          )}
        >
          All
        </button>

        <button
          type="button"
          onClick={refresh}
          className="p-1 hover:bg-white/10 rounded transition-colors ml-auto"
          title="Refresh"
        >
          <RefreshCw className={cn("size-3.5 text-muted-foreground", loading && "animate-spin")} />
        </button>
      </div>

      {/* Sub-filter: Review / Mentioned / All */}
      <div className="flex items-center gap-1 px-4 pb-2">
        {VIEW_FILTERS.map((f) => (
          <button
            key={f.key}
            type="button"
            onClick={() => setViewFilter(f.key)}
            className={cn(
              "px-2 py-0.5 text-[10px] rounded transition-colors",
              viewFilter === f.key
                ? "bg-white/[0.06] text-white/80"
                : "text-tertiary hover:text-muted-foreground",
            )}
          >
            {f.label}
          </button>
        ))}
        <span className="text-[10px] text-tertiary ml-auto">
          {filtered.filter((n) => n.unread).length} unread
        </span>
      </div>

      {loading && notifications.length === 0 ? (
        <div className="flex-1 flex items-center justify-center">
          <Loader2 className="size-5 animate-spin text-muted-foreground" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex-1 flex items-center justify-center">
          <p className="text-xs text-muted-foreground">No notifications</p>
        </div>
      ) : (
        <div className="flex-1 overflow-auto px-4 pb-4 space-y-2">
          {filtered.map((notif) => (
            // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: notification card opens in browser
            <div
              key={notif.id}
              onClick={() => BrowserOpenURL(notif.url)}
              className="p-3 bg-white/[0.06] glass-card rounded-xl border border-border inset-highlight hover:bg-white/[0.09] hover:border-white/[0.12] transition-all cursor-pointer group"
            >
              <div className="flex items-start gap-3">
                <div className="size-8 rounded-lg bg-white/[0.08] flex items-center justify-center shrink-0">
                  {typeIcon(notif.type)}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <p className="text-sm text-white truncate flex-1">{notif.title}</p>
                    {notif.unread && (
                      <button
                        type="button"
                        title="Mark as read"
                        onClick={(e) => {
                          e.stopPropagation();
                          onMarkRead(notif.id);
                        }}
                        className="size-5 rounded-full flex items-center justify-center shrink-0 transition-colors hover:bg-white/10"
                      >
                        <Circle className="size-2 fill-accent text-accent group-hover:hidden" />
                        <Check className="size-3 text-accent hidden group-hover:block" />
                      </button>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">{notif.repo}</span>
                    {reasonLabel(notif.reason) && (
                      <>
                        <span className="text-xs text-muted-foreground/50">&middot;</span>
                        <span
                          className={`text-[10px] font-medium px-1.5 py-0.5 rounded ${reasonColor(notif.reason)}`}
                        >
                          {reasonLabel(notif.reason)}
                        </span>
                      </>
                    )}
                    <span className="text-[10px] text-tertiary ml-auto shrink-0">
                      {timeAgo(notif.updatedAt)}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
