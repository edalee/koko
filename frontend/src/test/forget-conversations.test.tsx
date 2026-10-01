import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GetSessions, SaveSessions } from "../../wailsjs/go/main/ConfigService";
import { useSessionTabs } from "../hooks/useSessionTabs";

const mockGetSessions = GetSessions as ReturnType<typeof vi.fn>;
const mockSave = SaveSessions as ReturnType<typeof vi.fn>;

const closed = (slug: string, claudeSessionId: string) => ({
  slug,
  name: slug,
  directory: "/repo",
  createdAt: 1,
  closedAt: 2,
  status: "closed",
  claudeSessionId,
});

describe("useSessionTabs forgetConversations (plan 028 step 7a)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSessions.mockResolvedValue({
      sessions: [closed("koko-1", "gone"), closed("koko-2", "kept")],
      recentDirs: [],
    });
  });

  // Recent Sessions resumes a record's conversation. A deleted one would fail
  // to resume, so the record forgets it and starts fresh instead.
  it("drops deleted ids from closed-session records and saves", async () => {
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.history).toHaveLength(2));
    mockSave.mockClear();

    act(() => result.current.forgetConversations(["gone"]));

    const ids = result.current.history.map((h) => [h.slug, h.claudeSessionId]);
    expect(ids).toEqual([
      ["koko-1", ""],
      ["koko-2", "kept"],
    ]);
    const saved = mockSave.mock.calls[mockSave.mock.calls.length - 1][0].sessions;
    expect(saved.find((r: { slug: string }) => r.slug === "koko-1").claudeSessionId).toBe("");
  });

  it("saves nothing when no record points at a deleted id", async () => {
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.history).toHaveLength(2));
    mockSave.mockClear();

    act(() => result.current.forgetConversations(["unrelated"]));

    expect(mockSave).not.toHaveBeenCalled();
  });
});
