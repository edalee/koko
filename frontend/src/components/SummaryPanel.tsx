import { BarChart3 } from "lucide-react";

interface SummaryPanelProps {
  expanded: boolean;
  onToggle: () => void;
  prCount: number;
  slackCount: number;
}

export default function SummaryPanel({
  expanded,
  onToggle,
  prCount,
  slackCount,
}: SummaryPanelProps) {
  return (
    <div className="bg-surface overflow-hidden">
      <button
        className="flex w-full items-center gap-2 px-3 py-2 text-xs hover:bg-tab-hover transition-colors"
        onClick={onToggle}
      >
        <BarChart3 className="h-3.5 w-3.5 text-accent" />
        <span className="font-medium text-foreground">Summary</span>
      </button>

      {expanded && (
        <div className="border-t border-border/30 px-3 py-2 text-xs space-y-1">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Open PRs</span>
            <span className="text-foreground">{prCount}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Slack messages</span>
            <span className="text-foreground">{slackCount}</span>
          </div>
        </div>
      )}
    </div>
  );
}
