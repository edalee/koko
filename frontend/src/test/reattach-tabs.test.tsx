import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GetSessions } from "../../wailsjs/go/main/ConfigService";
import { GetSessions as GetLiveSessions } from "../../wailsjs/go/main/TerminalManager";
import { reattachTabs, useSessionTabs } from "../hooks/useSessionTabs";
import type { SessionTab } from "../types";

const mockGetSessions = GetSessions as ReturnType<typeof vi.fn>;
const mockLive = GetLiveSessions as ReturnType<typeof vi.fn>;

const saved = (slug: string, claudeSessionId = ""): SessionTab => ({
  id: `saved-${slug}`,
  slug,
  name: slug,
  directory: "/repo",
  createdAt: 1,
  connected: false,
  claudeSessionId,
});

const live = (id: string, slug: string, claudeSessionId = "", shell = false) => ({
  id,
  slug,
  name: slug,
  dir: "/repo",
  claudeSessionId,
  shell,
});

describe("reattachTabs", () => {
  it("gives a saved tab the id of its live session, by slug", () => {
    const tabs = reattachTabs([saved("api-1", "c1"), saved("api-2")], [live("s7", "api-1", "c1")]);
    expect(tabs.map((t) => [t.slug, t.id, t.connected])).toEqual([
      ["api-1", "s7", true],
      ["api-2", "saved-api-2", false],
    ]);
  });

  it("matches by conversation id when the slug differs", () => {
    const [tab] = reattachTabs([saved("api-1", "c1")], [live("s7", "api-9", "c1")]);
    expect(tab).toMatchObject({ id: "s7", connected: true, slug: "api-1" });
  });

  it("adds a tab for a live session with no saved tab, but not for a shell", () => {
    const tabs = reattachTabs([], [live("s3", "web-1", "c3"), live("shell-4", "", "", true)]);
    expect(tabs.map((t) => [t.id, t.slug, t.claudeSessionId, t.connected])).toEqual([
      ["s3", "web-1", "c3", true],
    ]);
  });

  it("gives one live session to one tab only", () => {
    const tabs = reattachTabs(
      [saved("api-1", "c1"), saved("api-2", "c1")],
      [live("s7", "api-1", "c1")],
    );
    expect(tabs.map((t) => t.connected)).toEqual([true, false]);
  });
});

describe("useSessionTabs after a window reload", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSessions.mockResolvedValue({
      sessions: [
        {
          slug: "api-1",
          name: "api",
          directory: "/repo",
          createdAt: 1,
          status: "active",
          claudeSessionId: "c1",
        },
      ],
      recentDirs: [],
    });
    mockLive.mockResolvedValue([live("s7", "api-1", "c1")]);
  });

  it("loads the tab connected to the session that still runs", async () => {
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.tabs).toHaveLength(1));
    expect(result.current.tabs[0]).toMatchObject({ id: "s7", connected: true });
    expect(result.current.activeTabId).toBe("s7");
  });
});
