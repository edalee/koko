import { useCallback, useEffect, useRef, useState } from "react";
import { GetConfig } from "../../wailsjs/go/main/ConfigService";
import { GetMessages } from "../../wailsjs/go/main/SlackService";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

export interface SlackMessage {
  type: "dm" | "mention";
  channel: string;
  channelId: string;
  user: string;
  text: string;
  timestamp: string;
  teamId: string;
  unread: boolean;
  time: number;
}

export function useSlack() {
  const [messages, setMessages] = useState<SlackMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [configured, setConfigured] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const checkConfigured = useCallback(async () => {
    try {
      const config = await GetConfig();
      setConfigured(!!config.slackToken);
      return !!config.slackToken;
    } catch {
      setConfigured(false);
      return false;
    }
  }, []);

  const refresh = useCallback(async () => {
    const hasToken = await checkConfigured();
    if (!hasToken) {
      setMessages([]);
      return;
    }
    try {
      setLoading(true);
      const msgs = await GetMessages();
      setMessages((msgs as SlackMessage[]) ?? []);
    } catch (err) {
      console.error("[Slack]", err);
      setMessages([]);
    } finally {
      setLoading(false);
    }
  }, [checkConfigured]);

  useEffect(() => {
    refresh();
    // Poll every 60 seconds
    intervalRef.current = setInterval(refresh, 60000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [refresh]);

  const openMessage = useCallback((msg: SlackMessage) => {
    // Slack deep link: opens in Slack desktop app
    const link = `slack://channel?team=${msg.teamId}&id=${msg.channelId}&message=${msg.timestamp}`;
    BrowserOpenURL(link);
  }, []);

  const unreadCount = messages.filter((m) => m.unread).length;

  return { messages, loading, configured, unreadCount, refresh, openMessage };
}
