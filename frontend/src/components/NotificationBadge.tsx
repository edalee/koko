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
    <span
      className="absolute -top-1 -right-1 min-w-[18px] h-[18px] rounded-full flex items-center justify-center text-[10px] font-bold text-white animate-badge-pop"
      style={{ backgroundColor: color }}
    >
      {count > 99 ? "99+" : count}
    </span>
  );
}
