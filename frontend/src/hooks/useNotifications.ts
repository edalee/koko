import { useCallback, useEffect, useRef, useState } from "react";
import {
  FetchNotifications,
  MarkAllNotificationsRead,
  MarkNotificationRead,
} from "../../wailsjs/go/main/GitHubService";

export interface GitHubNotification {
  id: string;
  title: string;
  type: string;
  reason: string;
  repo: string;
  url: string;
  unread: boolean;
  updatedAt: string;
}

export type NotifFilter = "participating" | "all";

interface UseNotificationsResult {
  notifications: GitHubNotification[];
  unreadCount: number;
  loading: boolean;
  filter: NotifFilter;
  setFilter: (f: NotifFilter) => void;
  refresh: () => void;
  markRead: (id: string) => void;
  markAllRead: () => void;
}

export function useNotifications(): UseNotificationsResult {
  const [notifications, setNotifications] = useState<GitHubNotification[]>([]);
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState<NotifFilter>("participating");
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(
    async (showLoading = false) => {
      if (showLoading) setLoading(true);
      try {
        const data = await FetchNotifications(filter);
        if (data) {
          setNotifications(data as GitHubNotification[]);
        }
      } catch {
        // gh CLI may not be available
      } finally {
        if (showLoading) setLoading(false);
      }
    },
    [filter],
  );

  const refresh = useCallback(() => {
    fetchData(true);
  }, [fetchData]);

  useEffect(() => {
    fetchData(true);
    intervalRef.current = setInterval(() => fetchData(false), 60_000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [fetchData]);

  const markRead = useCallback(
    async (id: string) => {
      // Optimistic update
      setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, unread: false } : n)));
      try {
        await MarkNotificationRead(id);
        // Refresh from server to get updated list
        fetchData(false);
      } catch {
        // Revert on failure
        fetchData(false);
      }
    },
    [fetchData],
  );

  const unreadCount = notifications.filter((n) => n.unread).length;

  const markAllRead = useCallback(async () => {
    setNotifications((prev) => prev.map((n) => ({ ...n, unread: false })));
    try {
      await MarkAllNotificationsRead();
      fetchData(false);
    } catch {
      fetchData(false);
    }
  }, [fetchData]);

  return { notifications, unreadCount, loading, filter, setFilter, refresh, markRead, markAllRead };
}
