interface NotificationBadgeProps {
  count: number;
  color?: string;
}

export default function NotificationBadge({
  count,
  color = "var(--color-accent)",
}: NotificationBadgeProps) {
  if (count === 0) return null;

  return (
    <>
      {/* Inline count label */}
      <span className="text-xs font-medium tabular-nums" style={{ color }}>
        {count > 99 ? "99+" : count}
      </span>
      {/* Small dot indicator */}
      <span
        className="absolute top-1 right-1 size-1.5 rounded-full animate-badge-pop"
        style={{ backgroundColor: color }}
      />
    </>
  );
}
