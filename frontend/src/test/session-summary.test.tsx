import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GetSessionSummary } from "../../wailsjs/go/main/ClaudeService";
import { GetSessions, SaveSessions } from "../../wailsjs/go/main/ConfigService";
import type { main } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import ConversationPicker from "../components/ConversationPicker";
import SessionDialog from "../components/SessionDialog";
import SessionSidebar from "../components/SessionSidebar";
import type { SessionState } from "../hooks/useSessionActivity";
import { useSessionSummaries } from "../hooks/useSessionSummaries";
import { useSessionTabs } from "../hooks/useSessionTabs";
import type { SessionHistoryEntry, SessionTab } from "../types";

const mockSummary = GetSessionSummary as ReturnType<typeof vi.fn>;
const mockGetSessions = GetSessions as ReturnType<typeof vi.fn>;
const mockSave = SaveSessions as ReturnType<typeof vi.fn>;

function summary(over: Partial<main.SessionSummary> = {}): main.SessionSummary {
  return {
    title: "",
    recap: "",
    lastPrompt: "",
    recentPrompts: [],
    branch: "",
    ticket: "",
    updatedAt: 0,
    ...over,
  } as main.SessionSummary;
}

function tab(over: Partial<SessionTab> = {}): SessionTab {
  return {
    id: "s1",
    slug: "app-1",
    name: "app",
    directory: "/work/app",
    createdAt: 1,
    connected: true,
    claudeSessionId: "u1",
    ...over,
  };
}

function renderSidebar(sessions: SessionTab[], summaries: Map<string, main.SessionSummary>) {
  const onSelect = vi.fn();
  render(
    <SessionSidebar
      sessions={sessions}
      activeSessionId={null}
      sessionStates={new Map()}
      sessionSummaries={summaries}
      onSessionSelect={onSelect}
      onNewSession={vi.fn()}
      onDeleteSession={vi.fn()}
      onRenameSession={vi.fn()}
      isCollapsed={false}
      onToggleCollapse={vi.fn()}
    />,
  );
  return { onSelect };
}

const full = summary({
  title: "Retry uploads with back-off",
  recap: "Added retries to the upload client and opened a PR.",
  lastPrompt: "Now make the retry back off",
  recentPrompts: [
    { text: "Now make the retry back off", timestamp: Date.now() - 5 * 60_000 },
    { text: "Add a retry to the upload client", timestamp: Date.now() - 2 * 3_600_000 },
  ] as main.SessionPrompt[],
  branch: "feat/ACME-123-retry",
  pr: {
    number: 42,
    repo: "acme/app",
    url: "https://github.com/acme/app/pull/42",
  } as main.SessionPR,
  ticket: "ACME-123",
});

describe("session row second line (plan 036)", () => {
  it("shows the ticket chip and the title", () => {
    renderSidebar([tab()], new Map([["u1", full]]));
    const line = screen.getByTestId("session-summary-line");
    expect(within(line).getByText("ACME-123")).toBeInTheDocument();
    expect(within(line).getByText("Retry uploads with back-off")).toBeInTheDocument();
  });

  it("falls back to the last prompt when there is no title", () => {
    renderSidebar([tab()], new Map([["u1", summary({ lastPrompt: "Fix the flaky test" })]]));
    expect(screen.getByTestId("session-summary-line")).toHaveTextContent("Fix the flaky test");
  });

  it("uses the title the tab kept before the first fetch", () => {
    renderSidebar([tab({ title: "Kept title", ticket: "ACME-7" })], new Map());
    const line = screen.getByTestId("session-summary-line");
    expect(line).toHaveTextContent("ACME-7");
    expect(line).toHaveTextContent("Kept title");
  });

  it("shows no second line with nothing to say", () => {
    renderSidebar([tab()], new Map([["u1", summary()]]));
    expect(screen.queryByTestId("session-summary-line")).toBeNull();
  });

  it("selects the row from the keyboard", () => {
    const { onSelect } = renderSidebar([tab()], new Map());
    fireEvent.keyDown(screen.getByRole("button", { name: /^app/ }), { key: "Enter" });
    expect(onSelect).toHaveBeenCalledWith("s1");
  });
});

describe("session hover card (plan 036)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  function row() {
    return screen.getByRole("button", { name: /^app/ });
  }

  it("opens after a short delay on hover and shows the summary", () => {
    renderSidebar([tab()], new Map([["u1", full]]));
    fireEvent.mouseEnter(row().parentElement as HTMLElement);
    expect(screen.queryByRole("tooltip")).toBeNull();

    act(() => vi.advanceTimersByTime(400));
    const card = screen.getByRole("tooltip");
    expect(card).toHaveTextContent("Added retries to the upload client and opened a PR.");
    expect(card).toHaveTextContent("feat/ACME-123-retry");
    expect(card).toHaveTextContent("Add a retry to the upload client");
    expect(card).toHaveTextContent("5m ago");
    expect(card).toHaveTextContent("2h ago");

    fireEvent.click(within(card).getByRole("button", { name: "acme/app#42" }));
    expect(BrowserOpenURL).toHaveBeenCalledWith("https://github.com/acme/app/pull/42");
  });

  it("opens on keyboard focus, describes the row, and closes on Escape", () => {
    renderSidebar([tab()], new Map([["u1", full]]));
    act(() => row().focus());
    act(() => vi.advanceTimersByTime(400));

    const card = screen.getByRole("tooltip");
    expect(row()).toHaveAttribute("aria-describedby", card.id);

    fireEvent.keyDown(row(), { key: "Escape" });
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("does not open if the pointer leaves first", () => {
    renderSidebar([tab()], new Map([["u1", full]]));
    const wrapper = row().parentElement as HTMLElement;
    fireEvent.mouseEnter(wrapper);
    act(() => vi.advanceTimersByTime(200));
    fireEvent.mouseLeave(wrapper);
    act(() => vi.advanceTimersByTime(1000));
    expect(screen.queryByRole("tooltip")).toBeNull();
  });
});

describe("picker and history show the title and recap (plan 036)", () => {
  it("picker rows show the recap", () => {
    render(
      <ConversationPicker
        conversations={[
          {
            uuid: "u1",
            title: "Retry uploads",
            recap: "Added retries to the upload client.",
            modifiedAt: Date.now(),
            sizeBytes: 1,
          } as main.Conversation,
        ]}
        selected=""
        onSelect={vi.fn()}
        holders={new Map()}
      />,
    );
    expect(screen.getByText("Retry uploads")).toBeInTheDocument();
    expect(screen.getByText("Added retries to the upload client.")).toBeInTheDocument();
  });

  it("history rows show the title, ticket and recap, and old rows only their name", () => {
    const history: SessionHistoryEntry[] = [
      {
        slug: "app-1",
        name: "app",
        directory: "/work/app",
        createdAt: 1,
        closedAt: Date.now(),
        title: "Retry uploads",
        recap: "Added retries.",
        ticket: "ACME-123",
        lastMessage: "an old reply that must not show",
      },
      {
        slug: "lib-1",
        name: "lib",
        directory: "/work/lib",
        createdAt: 1,
        closedAt: Date.now(),
        lastMessage: "Claude's last reply, from before",
      },
    ];
    render(
      <SessionDialog
        open
        onClose={vi.fn()}
        onCreate={vi.fn()}
        onOpenHeld={vi.fn()}
        onReconnect={vi.fn()}
        history={history}
        activeDirs={[]}
        tabs={[]}
        onConversationsDeleted={vi.fn()}
      />,
    );
    expect(screen.getByText("Retry uploads")).toBeInTheDocument();
    expect(screen.getByText("ACME-123")).toBeInTheDocument();
    expect(screen.getByText("Added retries.")).toBeInTheDocument();
    expect(screen.queryByText("an old reply that must not show")).toBeNull();
    expect(screen.getByText("lib")).toBeInTheDocument();
    expect(screen.queryByText("Claude's last reply, from before")).toBeNull();
  });
});

describe("saving the summary (plan 036)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSessions.mockResolvedValue({
      sessions: [
        {
          slug: "app-1",
          name: "app",
          directory: "/work/app",
          createdAt: 1,
          status: "disconnected",
          claudeSessionId: "u1",
        },
      ],
      recentDirs: [],
    });
  });

  it("keeps the tab's own title, recap and ticket in the closed record", async () => {
    mockSummary.mockResolvedValue(full);
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.tabs).toHaveLength(1));

    await act(() => result.current.closeTab(result.current.tabs[0].id));

    expect(mockSummary).toHaveBeenCalledWith("/work/app", "u1");
    const saved = mockSave.mock.calls[mockSave.mock.calls.length - 1][0].sessions;
    expect(saved[0]).toMatchObject({
      status: "closed",
      title: "Retry uploads with back-off",
      recap: "Added retries to the upload client and opened a PR.",
      ticket: "ACME-123",
    });
  });

  it("keeps the saved summary at close when the transcript is gone", async () => {
    mockGetSessions.mockResolvedValue({
      sessions: [
        {
          slug: "app-1",
          name: "app",
          directory: "/work/app",
          createdAt: 1,
          status: "disconnected",
          claudeSessionId: "u1",
          title: "Saved title",
          ticket: "ACME-5",
        },
      ],
      recentDirs: [],
    });
    mockSummary.mockResolvedValue(summary({ notFound: true }));
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.tabs).toHaveLength(1));

    await act(() => result.current.closeTab(result.current.tabs[0].id));

    const saved = mockSave.mock.calls[mockSave.mock.calls.length - 1][0].sessions;
    expect(saved[0]).toMatchObject({ status: "closed", title: "Saved title", ticket: "ACME-5" });
  });

  it("updates a tab from a summary only when something changed", async () => {
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.tabs).toHaveLength(1));

    act(() => result.current.rememberSummaries(new Map([["u1", full]])));
    expect(result.current.tabs[0]).toMatchObject({ title: full.title, ticket: "ACME-123" });

    const before = result.current.tabs;
    act(() => result.current.rememberSummaries(new Map([["u1", full]])));
    expect(result.current.tabs).toBe(before);

    // A missing transcript keeps what was saved.
    act(() => result.current.rememberSummaries(new Map([["u1", summary({ notFound: true })]])));
    expect(result.current.tabs[0]).toMatchObject({ title: full.title, ticket: "ACME-123" });

    // An empty summary for the same conversation clears what was kept.
    act(() => result.current.rememberSummaries(new Map([["u1", summary()]])));
    expect(result.current.tabs[0]).toMatchObject({
      title: undefined,
      recap: undefined,
      ticket: undefined,
    });
  });

  it("clears the summary when a reconnect opens another conversation", async () => {
    const { result } = renderHook(() => useSessionTabs());
    await waitFor(() => expect(result.current.tabs).toHaveLength(1));
    act(() => result.current.rememberSummaries(new Map([["u1", full]])));

    // The same conversation keeps its summary.
    await act(() => result.current.reconnectTab(result.current.tabs[0]));
    expect(result.current.tabs[0]).toMatchObject({ title: full.title, ticket: "ACME-123" });

    // A fresh one does not.
    await act(() => result.current.reconnectTab(result.current.tabs[0], { fresh: true }));
    expect(result.current.tabs[0]).toMatchObject({
      claudeSessionId: undefined,
      title: undefined,
      recap: undefined,
      ticket: undefined,
    });
  });
});

describe("useSessionSummaries (plan 036)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    mockSummary.mockResolvedValue(full);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("fetches each conversation at once, then every 15 seconds", async () => {
    const tabs = [tab(), tab({ id: "s2", claudeSessionId: undefined })];
    const states = new Map<string, SessionState>();
    const { result } = renderHook(() => useSessionSummaries(tabs, states));
    await act(async () => {});
    // Only the tab with a conversation is fetched, by its own UUID.
    expect(mockSummary).toHaveBeenCalledTimes(1);
    expect(mockSummary).toHaveBeenCalledWith("/work/app", "u1");
    expect(result.current.get("u1")?.title).toBe(full.title);

    await act(async () => vi.advanceTimersByTime(15000));
    expect(mockSummary).toHaveBeenCalledTimes(2);
  });

  it("fetches when a tab goes from active to idle", async () => {
    const tabs = [tab()];
    const { rerender } = renderHook(({ states }) => useSessionSummaries(tabs, states), {
      initialProps: { states: new Map<string, SessionState>([["s1", "active"]]) },
    });
    await act(async () => {});
    mockSummary.mockClear();

    rerender({ states: new Map<string, SessionState>([["s1", "idle"]]) });
    await act(async () => {});
    expect(mockSummary).toHaveBeenCalledTimes(1);
  });
});
