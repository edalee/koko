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

interface Opts {
  tabs?: SessionTab[];
  history?: SessionHistoryEntry[];
  onCreate?: Mock<OnCreate>;
  onOpenHeld?: Mock<OnOpenHeld>;
}

function renderDialog(o: Opts = {}) {
  const onCreate = o.onCreate ?? vi.fn<OnCreate>().mockResolvedValue(undefined);
  const onOpenHeld = o.onOpenHeld ?? vi.fn<OnOpenHeld>();
  render(
    <SessionDialog
      open
      onClose={vi.fn()}
      onCreate={onCreate}
      onOpenHeld={onOpenHeld}
      history={o.history ?? []}
      activeDirs={[]}
      tabs={o.tabs ?? []}
    />,
  );
  return { onCreate, onOpenHeld };
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
  });
});
