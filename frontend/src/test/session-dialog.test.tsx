import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { PickDirectory } from "../../wailsjs/go/main/App";
import { ListConversations } from "../../wailsjs/go/main/ClaudeService";
import SessionDialog, { submitLabel } from "../components/SessionDialog";
import type { SessionHistoryEntry, SessionTab } from "../types";

const mockPick = PickDirectory as ReturnType<typeof vi.fn>;
const mockList = ListConversations as ReturnType<typeof vi.fn>;

const conv = (uuid: string, title: string, minutesAgo = 5) => ({
  uuid,
  title,
  preview: `${title} preview`,
  modifiedAt: Date.now() - minutesAgo * 60_000,
  sizeBytes: 100,
});

function tab(over: Partial<SessionTab>): SessionTab {
  return {
    id: "session-1",
    slug: "koko-2",
    name: "koko",
    directory: "/repo",
    createdAt: 0,
    connected: true,
    ...over,
  };
}

type OnCreate = ComponentProps<typeof SessionDialog>["onCreate"];
type OnOpenHeld = ComponentProps<typeof SessionDialog>["onOpenHeld"];
type OnReconnect = ComponentProps<typeof SessionDialog>["onReconnect"];

interface Opts {
  tabs?: SessionTab[];
  history?: SessionHistoryEntry[];
  onCreate?: Mock<OnCreate>;
  onOpenHeld?: Mock<OnOpenHeld>;
  onReconnect?: Mock<OnReconnect>;
  reconnect?: SessionTab;
  initialDirectory?: string;
}

function renderDialog(o: Opts = {}) {
  const onCreate = o.onCreate ?? vi.fn<OnCreate>().mockResolvedValue(undefined);
  const onOpenHeld = o.onOpenHeld ?? vi.fn<OnOpenHeld>();
  const onReconnect = o.onReconnect ?? vi.fn<OnReconnect>().mockResolvedValue(undefined);
  render(
    <SessionDialog
      open
      onClose={vi.fn()}
      onCreate={onCreate}
      onOpenHeld={onOpenHeld}
      onReconnect={onReconnect}
      reconnect={o.reconnect}
      initialDirectory={o.initialDirectory}
      history={o.history ?? []}
      activeDirs={[]}
      tabs={o.tabs ?? []}
    />,
  );
  return { onCreate, onOpenHeld, onReconnect };
}

async function chooseDirectory(dir: string) {
  mockPick.mockResolvedValueOnce(dir);
  fireEvent.click(await screen.findByText("Browse..."));
  await screen.findByText(dir);
}

describe("SessionDialog conversation picker", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([]);
  });

  it("lists the directory's conversations once it is chosen", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug"), conv("b", "Plan the picker")]);
    renderDialog();
    await chooseDirectory("/repo");

    expect(await screen.findByText("Fix the resize bug")).toBeInTheDocument();
    expect(screen.getByText("Plan the picker")).toBeInTheDocument();
    expect(mockList).toHaveBeenCalledWith("/repo");
  });

  it("shows no list or heading when the directory has no conversations", async () => {
    renderDialog();
    await chooseDirectory("/repo");
    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(screen.queryByText("Conversations in this directory")).not.toBeInTheDocument();
  });

  it("starts a new conversation by default", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug")]);
    const { onCreate } = renderDialog();
    await chooseDirectory("/repo");
    await screen.findByText("Fix the resize bug");

    fireEvent.click(screen.getByRole("button", { name: "Create Session" }));
    await waitFor(() => expect(onCreate).toHaveBeenCalled());
    expect(onCreate.mock.calls[0][3]).toBeUndefined();
  });

  it("resumes the chosen conversation", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug")]);
    const { onCreate } = renderDialog();
    await chooseDirectory("/repo");
    fireEvent.click(await screen.findByText("Fix the resize bug"));

    fireEvent.click(screen.getByRole("button", { name: "Resume Conversation" }));
    await waitFor(() => expect(onCreate).toHaveBeenCalled());
    expect(onCreate.mock.calls[0][3]).toEqual({ claudeSessionId: "a", slug: undefined });
  });

  // D2: one conversation, one tab. A held conversation switches to its tab.
  it("switches to the tab already holding a conversation", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug")]);
    const { onCreate, onOpenHeld } = renderDialog({
      tabs: [tab({ id: "session-9", slug: "koko-2", claudeSessionId: "a", connected: false })],
    });
    await chooseDirectory("/repo");

    expect(await screen.findByText("reconnect koko-2")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Fix the resize bug"));
    fireEvent.click(screen.getByRole("button", { name: "Reconnect koko-2" }));

    expect(onOpenHeld).toHaveBeenCalledWith("session-9");
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("says open, not reconnect, when the holder is connected", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug")]);
    renderDialog({ tabs: [tab({ claudeSessionId: "a", connected: true })] });
    await chooseDirectory("/repo");
    expect(await screen.findByText("open in koko-2")).toBeInTheDocument();
  });

  // The regression: Recent Sessions used to start a fresh session and drop the
  // stored conversation. It now preselects it, and reuses the old slug.
  it("preselects a Recent Sessions entry's conversation", async () => {
    mockList.mockResolvedValue([conv("x", "Some other one"), conv("old", "The closed one")]);
    const { onCreate } = renderDialog({
      history: [
        {
          slug: "koko-4",
          name: "The closed one",
          directory: "/repo",
          createdAt: 0,
          closedAt: Date.now() - 3_600_000,
          claudeSessionId: "old",
        },
      ],
    });

    fireEvent.click(await screen.findByText("The closed one"));
    const resume = await screen.findByRole("button", { name: "Resume Conversation" });
    fireEvent.click(resume);

    await waitFor(() => expect(onCreate).toHaveBeenCalled());
    expect(onCreate.mock.calls[0][3]).toEqual({ claudeSessionId: "old", slug: "koko-4" });
  });

  it("keeps a Recent Sessions conversation reachable when it is older than the listed ones", async () => {
    mockList.mockResolvedValue([conv("x", "Some other one")]);
    const { onCreate } = renderDialog({
      history: [
        {
          slug: "koko-4",
          name: "Very old session",
          directory: "/repo",
          createdAt: 0,
          closedAt: Date.now() - 30 * 86_400_000,
          claudeSessionId: "ancient",
        },
      ],
    });

    fireEvent.click(await screen.findByText("Very old session"));
    fireEvent.click(await screen.findByRole("button", { name: "Resume Conversation" }));
    await waitFor(() => expect(onCreate).toHaveBeenCalled());
    expect(onCreate.mock.calls[0][3]?.claudeSessionId).toBe("ancient");
  });

  it("clears the chosen conversation when a worktree is turned on", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug")]);
    renderDialog();
    await chooseDirectory("/repo");
    fireEvent.click(await screen.findByText("Fix the resize bug"));
    expect(screen.getByRole("button", { name: "Resume Conversation" })).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText(/Create as a git worktree/));
    expect(screen.getByRole("button", { name: "Create Worktree + Session" })).toBeInTheDocument();
  });

  it("shows why a resume failed and stays open", async () => {
    mockList.mockResolvedValue([conv("a", "Fix the resize bug")]);
    const onCreate = vi
      .fn<OnCreate>()
      .mockRejectedValue("that conversation is already open in another session");
    renderDialog({ onCreate });
    await chooseDirectory("/repo");
    fireEvent.click(await screen.findByText("Fix the resize bug"));
    fireEvent.click(screen.getByRole("button", { name: "Resume Conversation" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This conversation is already open in another tab.",
    );
  });

  // Switching directory while a slow reply is in flight must not show the old
  // directory's conversations under the new one.
  it("ignores a reply for a directory the user has left", async () => {
    let resolveSlow: (v: unknown) => void = () => {};
    mockList
      .mockImplementationOnce(
        () =>
          new Promise((r) => {
            resolveSlow = r;
          }),
      )
      .mockResolvedValueOnce([conv("fast", "Fast directory conversation")]);

    renderDialog();
    await chooseDirectory("/slow");

    mockPick.mockResolvedValueOnce("/fast");
    fireEvent.click(await screen.findByText("/slow"));
    await screen.findByText("Fast directory conversation");

    await act(async () => {
      resolveSlow([conv("stale", "Stale slow conversation")]);
    });
    expect(screen.queryByText("Stale slow conversation")).not.toBeInTheDocument();
  });
});

describe("SessionDialog routing from other entry points (step 5)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([]);
  });

  // The worktrees module used to start a fresh session straight away. It now
  // opens the dialog on that worktree, with a new conversation selected.
  it("opens on a worktree with a new conversation selected", async () => {
    mockList.mockResolvedValue([conv("w", "Worktree conversation")]);
    const { onCreate } = renderDialog({ initialDirectory: "/repo-wt" });

    expect(await screen.findByText("Worktree conversation")).toBeInTheDocument();
    expect(mockList).toHaveBeenCalledWith("/repo-wt");
    fireEvent.click(screen.getByRole("button", { name: "Create Session" }));
    await waitFor(() => expect(onCreate).toHaveBeenCalled());
    expect(onCreate.mock.calls[0][1]).toBe("/repo-wt");
    expect(onCreate.mock.calls[0][3]).toBeUndefined();
  });

  // UX rule 2: a disconnected tab with a stored conversation is restored in
  // one keystroke. Its own conversation is preselected, not shown as held.
  it("preselects a disconnected tab's own conversation", async () => {
    mockList.mockResolvedValue([conv("mine", "My conversation")]);
    const t = tab({ id: "session-3", connected: false, claudeSessionId: "mine" });
    const { onReconnect, onOpenHeld } = renderDialog({ reconnect: t, tabs: [t] });

    expect(await screen.findByText("My conversation")).toBeInTheDocument();
    expect(screen.queryByText(/reconnect koko-2/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Resume Conversation" }));

    await waitFor(() => expect(onReconnect).toHaveBeenCalled());
    expect(onReconnect.mock.calls[0][0].id).toBe("session-3");
    expect(onReconnect.mock.calls[0][1]).toEqual({ claudeSessionId: "mine" });
    expect(onOpenHeld).not.toHaveBeenCalled();
  });

  // D1: a tab saved before conversation ids were captured preselects nothing.
  // Guessing the newest is what --continue did, and it is often a sibling's.
  it("preselects nothing for a tab with no stored conversation", async () => {
    mockList.mockResolvedValue([conv("a", "Some conversation")]);
    const t = tab({ connected: false, claudeSessionId: undefined });
    const { onReconnect } = renderDialog({ reconnect: t, tabs: [t] });

    const open = await screen.findByRole("button", { name: "Choose a conversation" });
    expect(open).toBeDisabled();
    expect(screen.getByText(/This tab has no stored conversation/)).toBeInTheDocument();

    fireEvent.click(await screen.findByText("Some conversation"));
    fireEvent.click(screen.getByRole("button", { name: "Resume Conversation" }));
    await waitFor(() => expect(onReconnect).toHaveBeenCalled());
    expect(onReconnect.mock.calls[0][1]).toEqual({ claudeSessionId: "a" });
  });

  it("keeps Open disabled for D1 while the list is still loading", async () => {
    mockList.mockImplementation(() => new Promise(() => {})); // never resolves
    const t = tab({ connected: false, claudeSessionId: undefined });
    renderDialog({ reconnect: t, tabs: [t] });

    expect(await screen.findByRole("button", { name: "Choose a conversation" })).toBeDisabled();
  });

  it("reconnects the same tab into a fresh conversation", async () => {
    mockList.mockResolvedValue([conv("mine", "My conversation")]);
    const t = tab({ connected: false, claudeSessionId: "mine" });
    const { onReconnect } = renderDialog({ reconnect: t, tabs: [t] });

    fireEvent.click(await screen.findByText("Start a new conversation"));
    fireEvent.click(screen.getByRole("button", { name: "Start New Conversation" }));
    await waitFor(() => expect(onReconnect).toHaveBeenCalled());
    expect(onReconnect.mock.calls[0][1]).toEqual({ fresh: true });
  });

  it("keeps a tab's own conversation reachable when it is older than the listed ones", async () => {
    mockList.mockResolvedValue([conv("x", "Newer conversation")]);
    const t = tab({ name: "Old work", connected: false, claudeSessionId: "ancient" });
    const { onReconnect } = renderDialog({ reconnect: t, tabs: [t] });

    expect(await screen.findByText("Old work")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Resume Conversation" }));
    await waitFor(() => expect(onReconnect).toHaveBeenCalled());
    expect(onReconnect.mock.calls[0][1]).toEqual({ claudeSessionId: "ancient" });
  });

  // D2 still applies while reconnecting: a conversation another tab holds
  // switches to that tab rather than being opened twice.
  it("switches to another tab holding the chosen conversation", async () => {
    mockList.mockResolvedValue([conv("theirs", "Their conversation")]);
    const me = tab({ id: "session-me", slug: "koko-1", connected: false });
    const other = tab({ id: "session-other", slug: "koko-2", claudeSessionId: "theirs" });
    const { onReconnect, onOpenHeld } = renderDialog({ reconnect: me, tabs: [me, other] });

    fireEvent.click(await screen.findByText("Their conversation"));
    fireEvent.click(screen.getByRole("button", { name: "Open in koko-2" }));
    expect(onOpenHeld).toHaveBeenCalledWith("session-other");
    expect(onReconnect).not.toHaveBeenCalled();
  });

  it("shows why a reconnect failed and stays open", async () => {
    mockList.mockResolvedValue([conv("mine", "My conversation")]);
    const t = tab({ connected: false, claudeSessionId: "mine" });
    const onReconnect = vi
      .fn<OnReconnect>()
      .mockRejectedValue("that conversation is already open in another session");
    renderDialog({ reconnect: t, tabs: [t], onReconnect });

    fireEvent.click(await screen.findByRole("button", { name: "Resume Conversation" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This conversation is already open in another tab.",
    );
  });

  it("titles the dialog after the tab and hides new-session fields", async () => {
    const t = tab({ slug: "koko-7", connected: false, claudeSessionId: "mine" });
    renderDialog({ reconnect: t, tabs: [t] });

    expect(await screen.findByText("Reconnect koko-7")).toBeInTheDocument();
    expect(screen.queryByLabelText("Session Name")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Create as a git worktree/)).not.toBeInTheDocument();
    expect(screen.queryByText("Browse...")).not.toBeInTheDocument();
  });
});

describe("submitLabel", () => {
  const holder = { tabId: "t", label: "koko-2", connected: true };
  it("says what the button will do", () => {
    expect(submitLabel({ creatingWorktree: false, selected: "", useWorktree: false })).toBe(
      "Create Session",
    );
    expect(submitLabel({ creatingWorktree: false, selected: "a", useWorktree: false })).toBe(
      "Resume Conversation",
    );
    expect(
      submitLabel({ creatingWorktree: false, holder, selected: "a", useWorktree: false }),
    ).toBe("Open in koko-2");
    expect(
      submitLabel({
        creatingWorktree: false,
        holder: { ...holder, connected: false },
        selected: "a",
        useWorktree: false,
      }),
    ).toBe("Reconnect koko-2");
    expect(submitLabel({ creatingWorktree: false, selected: null, useWorktree: false })).toBe(
      "Choose a conversation",
    );
    expect(
      submitLabel({
        creatingWorktree: false,
        selected: "",
        useWorktree: false,
        reconnecting: true,
      }),
    ).toBe("Start New Conversation");
  });
});
