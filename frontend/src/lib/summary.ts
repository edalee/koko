import type { main } from "../../wailsjs/go/models";

/** The summary fields a session keeps on its tab and its saved record. */
export interface SavedSummary {
  title?: string;
  recap?: string;
  ticket?: string;
}

/**
 * The fields to keep from a fetched summary. An empty summary clears them,
 * because the conversation has nothing to say yet. null means no summary was
 * read, so the caller keeps what it had.
 */
export function savedSummary(s: main.SessionSummary | null | undefined): SavedSummary | null {
  if (!s) return null;
  return {
    title: s.title || undefined,
    recap: s.recap || undefined,
    ticket: s.ticket || undefined,
  };
}

/** The one line a session row shows: the title, else the last prompt. */
export function summaryLine(s: main.SessionSummary | undefined, saved: SavedSummary): string {
  return s?.title || saved.title || s?.lastPrompt || "";
}

/** How long ago a unix-millisecond time was, in a few characters. */
export function timeAgo(ts: number, now = Date.now()): string {
  const seconds = Math.floor((now - ts) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}
