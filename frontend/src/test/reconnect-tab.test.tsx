import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GetSessions } from "../../wailsjs/go/main/ConfigService";
import { CreateSessionWithOpts } from "../../wailsjs/go/main/TerminalManager";
import { useSessionTabs } from "../hooks/useSessionTabs";

const mockGetSessions = GetSessions as ReturnType<typeof vi.fn>;
const mockCreate = CreateSessionWithOpts as ReturnType<typeof vi.fn>;

// One saved tab, disconnected, with a stored conversation.
function savedTab(claudeSessionId = "conv-1") {
  return {
    sessions: [
      {
        slug: "koko-1",
        name: "koko",
        directory: "/repo",
        createdAt: 1,
        status: "disconnected",
        claudeSessionId,
      },
    ],
    recentDirs: [],
  };
}

async function loaded(claudeSessionId?: string) {
  mockGetSessions.mockResolvedValue(savedTab(claudeSessionId));
  const hook = renderHook(() => useSessionTabs());
  await waitFor(() => expect(hook.result.current.tabs).toHaveLength(1));
  return hook;
}

describe("useSessionTabs reconnecting (plan 028 step 5)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreate.mockResolvedValue("session-new");
  });

  // Selecting a disconnected tab used to reconnect it silently. Reconnecting
  // is now a choice made in the session dialog.
  it("switchTab only selects a disconnected tab", async () => {
    const { result } = await loaded();
    const id = result.current.tabs[0].id;

    act(() => result.current.switchTab(id));

    expect(result.current.activeTabId).toBe(id);
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("reconnects into the tab's own conversation by default", async () => {
    const { result } = await loaded("conv-1");
    const tab = result.current.tabs[0];

    await act(async () => {
      await result.current.reconnectTab(tab);
    });

    expect(mockCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        resume: true,
        claudeSessionId: "conv-1",
        slug: "koko-1",
        replaces: tab.id,
      }),
    );
  });

  it("reconnects into a chosen conversation", async () => {
    const { result } = await loaded("conv-1");
    const tab = result.current.tabs[0];

    await act(async () => {
      await result.current.reconnectTab(tab, { claudeSessionId: "conv-2" });
    });

    expect(mockCreate).toHaveBeenCalledWith(
      expect.objectContaining({ resume: true, claudeSessionId: "conv-2" }),
    );
    expect(result.current.tabs[0].claudeSessionId).toBe("conv-2");
  });

  // A fresh reconnect must not resume with an empty id, which the backend
  // turns into --continue: the newest conversation, often a sibling's.
  it("reconnects into a fresh conversation without resuming", async () => {
    const { result } = await loaded("conv-1");
    const tab = result.current.tabs[0];

    await act(async () => {
      await result.current.reconnectTab(tab, { fresh: true });
    });

    expect(mockCreate).toHaveBeenCalledWith(
      expect.objectContaining({ resume: false, claudeSessionId: "", slug: "koko-1" }),
    );
    expect(result.current.tabs[0].claudeSessionId).toBeUndefined();
    expect(result.current.tabs[0].connected).toBe(true);
  });

  // The dialog shows the reason, so the failure has to reach it.
  it("rejects on failure and records the reason on the tab", async () => {
    const { result } = await loaded("conv-1");
    const tab = result.current.tabs[0];
    mockCreate.mockRejectedValueOnce("that conversation is already open in another session");

    let caught: unknown;
    await act(async () => {
      await result.current.reconnectTab(tab).catch((e) => {
        caught = e;
      });
    });

    expect(caught).toBeDefined();
    expect(result.current.tabs[0].reconnectError).toBe(
      "This conversation is already open in another tab.",
    );
  });
});
