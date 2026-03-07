const mockItems = [
  { channel: "DMs", count: 3, user: "Alice" },
  { channel: "Threads", count: 2, user: "Bob" },
  { channel: "#product-drm", count: 1, user: "Charlie" },
];

export default function SlackPanel() {
  return (
    <div className="p-4 space-y-2">
      {mockItems.map((item) => (
        <div
          key={item.channel}
          className="p-3 bg-white/5 rounded-lg border border-border hover:bg-white/10 transition-colors cursor-pointer"
        >
          <div className="flex items-start gap-3">
            <div className="size-8 rounded bg-gradient-to-r from-accent to-accent-dark flex items-center justify-center text-toolbar text-xs font-bold shrink-0">
              {item.user[0]}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm text-white mb-1">{item.channel}</p>
              <p className="text-xs text-muted-foreground">{item.count} unread</p>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function useSlackCount() {
  return mockItems.reduce((sum, item) => sum + item.count, 0);
}
