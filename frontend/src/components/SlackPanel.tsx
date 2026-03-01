import { MessageSquare, ChevronRight } from "lucide-react";
import { cn } from "../lib/utils";

interface SlackPanelProps {
  expanded: boolean;
  onToggle: () => void;
}

const mockItems = [
  { channel: "DMs", count: 3, type: "dm" },
  { channel: "Threads", count: 2, type: "thread" },
  { channel: "#product-drm", count: 1, type: "mention" },
];

export default function SlackPanel({ expanded, onToggle }: SlackPanelProps) {
  const total = mockItems.reduce((sum, i) => sum + i.count, 0);

  return (
    <div className="bg-surface overflow-hidden rounded-lg">
      <button
        className="flex w-full items-center gap-2 px-3 py-2.5 text-xs hover:bg-tab-hover transition-colors"
        onClick={onToggle}
      >
        <MessageSquare className="h-3.5 w-3.5 text-accent" />
        <span className="font-medium text-foreground">Slack</span>
        <span className="ml-auto tabular-nums text-muted-foreground">{total} new</span>
        <ChevronRight className={cn("h-3 w-3 text-muted-foreground transition-transform", expanded && "rotate-90")} />
      </button>

      {expanded && (
        <div className="border-t border-border/30">
          {mockItems.map((item) => (
            <div
              key={item.channel}
              className="flex items-center justify-between px-3 py-1.5 text-xs hover:bg-tab-hover transition-colors"
            >
              <span className="text-foreground">{item.channel}</span>
              <span className="text-muted-foreground">{item.count}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
