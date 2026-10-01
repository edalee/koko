import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RemoveWorktrees } from "../../wailsjs/go/main/App";
import { GetSessions, SaveSessions } from "../../wailsjs/go/main/ConfigService";
import HousekeepingSettings from "../components/HousekeepingSettings";
import { useSessionTabs } from "../hooks/useSessionTabs";

const mockGetSessions = GetSessions as ReturnType<typeof vi.fn>;
const mockSave = SaveSessions as ReturnType<typeof vi.fn>;
const mockRemove = RemoveWorktrees as ReturnType<typeof vi.fn>;

type Saved = { slug: string; status: string; worktreePath?: string };
const lastSaved = (): Saved[] => mockSave.mock.calls[mockSave.mock.calls.length - 1][0].sessions;

const record = (slug: string, status: string, worktreePath = "", worktreeCreated = false) => ({
  slug,
  name: slug,
  directory: worktreePath || "/repo",
  createdAt: 1,
  closedAt: status === "closed" ? 2 : 0,
  status,
  worktreePath,
  worktreeCreated,
});

async function loaded(sessions: ReturnType<typeof record>[]) {
  mockGetSessions.mockResolvedValue({ sessions, recentDirs: [] });
  const hook = renderHook(() => useSessionTabs());
  await waitFor(() =>
    expect(hook.result.current.history.length + hook.result.current.tabs.length).toBe(
      sessions.length,
    ),
  );
  mockSave.mockClear();
  return hook;
}

describe("useSessionTabs housekeeping (plan 028 step 7b)", () => {
  beforeEach(() => vi.clearAllMocks());

  it("keeps a closed session's worktree path across a save", async () => {
    const { result } = await loaded([record("koko-1", "closed", "/wt/a")]);
    expect(result.current.history[0].worktreePath).toBe("/wt/a");

    act(() => result.current.forgetWorktrees(["/elsewhere"]));
    // A rename, even of a missing tab, makes a new tabs array, which saves.
    act(() => result.current.renameTab("none", "x"));

    await waitFor(() => expect(mockSave).toHaveBeenCalled());
    expect(lastSaved().find((r) => r.slug === "koko-1")?.worktreePath).toBe("/wt/a");
  });

  it("records a worktree Koko created and the user kept at close", async () => {
    const { result } = await loaded([record("koko-1", "disconnected", "/wt/a", true)]);
    const tab = result.current.tabs[0];
    expect(tab.worktreeCreated).toBe(true);

    await act(async () => {
      await result.current.closeTab(tab.id);
    });

    expect(result.current.history[0].worktreePath).toBe("/wt/a");
    const saved = lastSaved().find((r) => r.slug === "koko-1");
    expect(saved?.status).toBe("closed");
    expect(saved?.worktreePath).toBe("/wt/a");
  });

  it("records no worktree the user removed at close", async () => {
    const { result } = await loaded([record("koko-1", "disconnected", "/wt/a", true)]);

    await act(async () => {
      await result.current.closeTab(result.current.tabs[0].id, true);
    });

    expect(result.current.history[0].worktreePath).toBeUndefined();
  });

  // A worktree opened from the Worktrees module may be one the user made by
  // hand. It is not Koko's to remove in bulk.
  it("records no worktree Koko did not create", async () => {
    const { result } = await loaded([record("koko-1", "disconnected", "/wt/a", false)]);

    await act(async () => {
      await result.current.closeTab(result.current.tabs[0].id);
    });

    expect(result.current.history[0].worktreePath).toBeUndefined();
  });

  it("keeps the created flag on tabs it saves", async () => {
    const { result } = await loaded([
      record("koko-1", "disconnected", "/wt/a", true),
      record("koko-2", "disconnected"),
    ]);

    await act(async () => {
      await result.current.closeTab(result.current.tabs[1].id);
    });

    const saved = lastSaved().find((r) => r.slug === "koko-1") as Saved & {
      worktreeCreated?: boolean;
    };
    expect(saved.worktreePath).toBe("/wt/a");
    expect(saved.worktreeCreated).toBe(true);
  });

  it("clears every closed session and leaves open tabs alone", async () => {
    const { result } = await loaded([
      record("koko-1", "closed"),
      record("koko-2", "closed"),
      record("koko-3", "disconnected"),
    ]);

    act(() => result.current.clearHistory());

    expect(result.current.history).toHaveLength(0);
    expect(result.current.tabs).toHaveLength(1);
    expect(lastSaved().map((r) => r.slug)).toEqual(["koko-3"]);
  });

  it("forgets removed worktrees on closed sessions", async () => {
    const { result } = await loaded([
      record("koko-1", "closed", "/wt/a"),
      record("koko-2", "closed", "/wt/b"),
    ]);

    act(() => result.current.forgetWorktrees(["/wt/a"]));

    expect(result.current.history.map((h) => h.worktreePath)).toEqual([undefined, "/wt/b"]);
  });
});

describe("HousekeepingSettings", () => {
  beforeEach(() => vi.clearAllMocks());

  function renderPanel(over: Partial<Parameters<typeof HousekeepingSettings>[0]> = {}) {
    const props = {
      historyCount: 2,
      onClearHistory: vi.fn(),
      worktrees: ["/wt/a", "/wt/b"],
      onWorktreesRemoved: vi.fn(),
      ...over,
    };
    render(<HousekeepingSettings {...props} />);
    return props;
  }

  it("clears history only after confirming", async () => {
    const { onClearHistory } = renderPanel({ worktrees: [] });

    fireEvent.click(screen.getByRole("button", { name: "Clear history" }));
    expect(onClearHistory).not.toHaveBeenCalled();
    expect(
      screen.getByText("Koko forgets its closed sessions. Claude's conversations stay."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Clear" }));

    expect(onClearHistory).toHaveBeenCalled();
    expect(mockRemove).not.toHaveBeenCalled();
  });

  it("removes worktrees, forgets the removed ones and names the kept ones", async () => {
    mockRemove.mockResolvedValue({
      removed: ["/wt/a"],
      gone: ["/wt/c"],
      skipped: [{ path: "/wt/b", reason: "has uncommitted or ignored files" }],
    });
    const { onWorktreesRemoved } = renderPanel({ worktrees: ["/wt/a", "/wt/b", "/wt/c"] });

    fireEvent.click(screen.getByRole("button", { name: "Remove worktrees" }));
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));

    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith(["/wt/a", "/wt/b", "/wt/c"]));
    expect(await screen.findByText("Removed 1 worktree.")).toBeInTheDocument();
    expect(screen.getByText(/Already gone: \/wt\/c/)).toBeInTheDocument();
    expect(screen.getByText("Kept /wt/b: has uncommitted or ignored files.")).toBeInTheDocument();
    expect(onWorktreesRemoved).toHaveBeenCalledWith(["/wt/a", "/wt/c"]);
  });

  // Kept worktrees are listed from closed sessions, so clearing history
  // stops Koko listing them. The confirmation says so.
  it("warns that clearing history forgets kept worktrees", () => {
    renderPanel();

    fireEvent.click(screen.getByRole("button", { name: "Clear history" }));

    expect(screen.getByText(/stops listing 2 kept worktrees/)).toBeInTheDocument();
  });

  it("disables both actions when there is nothing to do", () => {
    renderPanel({ historyCount: 0, worktrees: [] });

    expect(screen.getByRole("button", { name: "Clear history" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove worktrees" })).toBeDisabled();
  });
});
