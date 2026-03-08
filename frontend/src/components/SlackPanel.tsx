import { AtSign, Loader2, MessageSquare, RefreshCw, Settings } from "lucide-react";
import type { SlackMessage } from "../hooks/useSlack";

interface SlackPanelProps {
  messages: SlackMessage[];
  loading: boolean;
  configured: boolean;
  onRefresh: () => void;
  onOpenMessage: (msg: SlackMessage) => void;
  onOpenSettings: () => void;
}

function timeAgo(unixTime: number): string {
  const now = Date.now() / 1000;
  const diff = now - unixTime;
  if (diff < 60) return "now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  return `${Math.floor(diff / 86400)}d`;
}

export default function SlackPanel({
  messages,
  loading,
  configured,
  onRefresh,
  onOpenMessage,
  onOpenSettings,
}: SlackPanelProps) {
  if (!configured) {
    return (
      <div className="p-6 flex flex-col items-center gap-3 text-center">
        <MessageSquare className="size-8 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">Connect Slack to see DMs and @mentions</p>
        <button
          type="button"
          onClick={onOpenSettings}
          className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg border border-white/[0.08] hover:bg-white/[0.06] transition-colors text-white"
        >
          <Settings className="size-3.5" />
          Open Settings
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-4 py-2">
        <span className="text-xs text-muted-foreground">
          {messages.length} message{messages.length !== 1 ? "s" : ""}
        </span>
        <button
          type="button"
          onClick={onRefresh}
          className="p-1 hover:bg-white/10 rounded transition-colors"
          title="Refresh"
        >
          <RefreshCw
            className={`size-3.5 text-muted-foreground ${loading ? "animate-spin" : ""}`}
          />
        </button>
      </div>

      {loading && messages.length === 0 ? (
        <div className="flex-1 flex items-center justify-center">
          <Loader2 className="size-5 animate-spin text-muted-foreground" />
        </div>
      ) : messages.length === 0 ? (
        <div className="flex-1 flex items-center justify-center">
          <p className="text-xs text-muted-foreground">No new messages</p>
        </div>
      ) : (
        <div className="flex-1 overflow-auto px-4 pb-4 space-y-2">
          {messages.map((msg) => (
            <button
              type="button"
              key={`${msg.channelId}-${msg.timestamp}`}
              onClick={() => onOpenMessage(msg)}
              className="w-full text-left p-3 bg-white/[0.06] glass-card rounded-xl border border-border inset-highlight hover:bg-white/[0.09] hover:border-white/[0.12] transition-all cursor-pointer"
            >
              <div className="flex items-start gap-3">
                <div className="size-8 rounded-lg bg-white/[0.08] flex items-center justify-center shrink-0">
                  {msg.type === "dm" ? (
                    <MessageSquare className="size-3.5 text-accent" />
                  ) : (
                    <AtSign className="size-3.5 text-blue-400" />
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm text-white truncate font-medium">{msg.user}</span>
                    <span className="text-[10px] text-tertiary shrink-0">{timeAgo(msg.time)}</span>
                  </div>
                  <p className="text-[10px] text-muted-foreground mb-1">{msg.channel}</p>
                  <p className="text-xs text-muted-foreground line-clamp-2">{msg.text}</p>
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
