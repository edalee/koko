import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GetSessions } from "../../wailsjs/go/main/ConfigService";
import { CreateSessionWithOpts, GetSessionSlug } from "../../wailsjs/go/main/TerminalManager";
import ConfirmDialog from "../components/ConfirmDialog";
import SessionSidebar from "../components/SessionSidebar";
import { reloadAction, useSessionTabs } from "../hooks/useSessionTabs";
import type { SessionTab } from "../types";

vi.mock("@xterm/xterm", () => {
  class MockTerminal {
    open = vi.fn();
    write = vi.fn();
    writeln = vi.fn();
    onData = vi.fn(() => ({ dispose: vi.fn() }));
    onBinary = vi.fn(() => ({ dispose: vi.fn() }));
    scrollToBottom = vi.fn();
    selectAll = vi.fn();
    clear = vi.fn();
    refresh = vi.fn();
    hasSelection = () => false;
    getSelection = () => "";
    attachCustomKeyEventHandler = vi.fn();
    loadAddon = vi.fn();
    dispose = vi.fn();
    focus = vi.fn();
    cols = 80;
    rows = 24;
  }
  return { Terminal: MockTerminal };
});
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit = vi.fn();
    proposeDimensions = vi.fn(() => ({ cols: 80, rows: 24 }));
    dispose = vi.fn();
  },
}));
vi.mock("@xterm/addon-serialize", () => ({
  SerializeAddon: class {
    serializeAsHTML = vi.fn(() => "");
    dispose = vi.fn();
  },
}));
vi.mock("@xterm/addon-webgl", () => ({
  WebglAddon: class {
    onContextLoss = vi.fn();
    clearTextureAtlas = vi.fn();
    dispose = vi.fn();
  },
}));
vi.mock("@xterm/addon-web-links", () => ({
  WebLinksAddon: class {
    dispose = vi.fn();
  },
}));
globalThis.ResizeObserver = class {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
} as unknown as typeof ResizeObserver;

import TerminalPane from "../components/TerminalPane";

const mockGetSessions = GetSessions as ReturnType<typeof vi.fn>;
const mockCreate = CreateSessionWithOpts as ReturnType<typeof vi.fn>;
const mockSlug = GetSessionSlug as ReturnType<typeof vi.fn>;

function tab(over: Partial<SessionTab> = {}): SessionTab {
  return {
    id: "session-1",
    slug: "koko-1",
    name: "koko",
    directory: "/repo",
    createdAt: 0,
    connected: true,
    claudeSessionId: "conv-1",
    ...over,
  };
}

describe("reloadAction", () => {
  it("reloads an idle tab straight away", () => {
    expect(reloadAction(tab(), "idle")).toBe("reload");
  });

  // UX rule 6: reload never silently discards work in progress. The states
  // are the frontend's activity states. The backend only knew "approval", so
  // a Claude mid-task used to count as idle.
  it("asks first when Claude is active or waiting on approval", () => {
    expect(reloadAction(tab(), "active")).toBe("confirm");
    expect(reloadAction(tab(), "approval")).toBe("confirm");
  });

  // If the state could not be read, asking is the safe side.
  it("asks first when the state is unknown", () => {
    expect(reloadAction(tab(), "unknown")).toBe("confirm");
  });

  // D1: reloading a tab with no stored conversation would have to guess.
  it("opens the picker for an idle tab with no stored conversation", () => {
    expect(reloadAction(tab({ claudeSessionId: undefined }), "idle")).toBe("pick");
  });

  // A busy tab with no stored conversation must be confirmed too. It used to
  // go straight to the picker, and picking a row killed Claude unasked.
  it("confirms a busy tab even when it has no stored conversation", () => {
    expect(reloadAction(tab({ claudeSessionId: undefined }), "active")).toBe("confirm");
  });

  it("does not ask for a disconnected tab, which has nothing running", () => {
    expect(reloadAction(tab({ connected: false }), "unknown")).toBe("reload");
  });
});

describe("ConfirmDialog", () => {
  function renderConfirm(destructive = true) {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <ConfirmDialog
        open
        title="Reload koko-1?"
        message="Claude isn't idle."
        confirmLabel="Reload"
        destructive={destructive}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    return { onConfirm, onCancel };
  }

  it("confirms with the button", async () => {
    const { onConfirm } = renderConfirm();
    fireEvent.click(await screen.findByRole("button", { name: "Reload" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  // The dialog takes focus from the terminal, which otherwise received the
  // same keypress. A destructive one focuses Cancel, so a stray Enter cancels.
  it("focuses Cancel when destructive, and the action otherwise", async () => {
    renderConfirm(true);
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Cancel" })),
    );
  });

  it("focuses the action when not destructive", async () => {
    renderConfirm(false);
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Reload" })),
    );
  });

  // Enter from a window listener used to confirm whatever had focus,
  // including Cancel. It is now left to the focused button.
  it("does not confirm on a window Enter", async () => {
    const { onConfirm } = renderConfirm();
    await screen.findByRole("button", { name: "Cancel" });
    fireEvent.keyDown(window, { key: "Enter" });
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("cancels with the button and with Escape", async () => {
    const { onCancel, onConfirm } = renderConfirm();
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(2);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});

describe("Reload Session in the terminal menu", () => {
  async function openMenu(onReload?: () => void) {
    const { container } = render(
      <TerminalPane sessionId="session-1" active={true} onReload={onReload} />,
    );
    const pane = container.querySelector("div.p-1") as HTMLElement;
    await act(async () => {
      fireEvent.contextMenu(pane);
    });
  }

  it("offers Reload Session and calls it", async () => {
    const onReload = vi.fn();
    await openMenu(onReload);
    fireEvent.mouseDown(screen.getByText("Reload Session"));
    expect(onReload).toHaveBeenCalled();
  });

  // Quick Terminal shells pass no handler: they have no conversation to keep.
  it("hides it when no handler is given", async () => {
    await openMenu();
    expect(screen.queryByText("Reload Session")).not.toBeInTheDocument();
  });
});

describe("Reload in the session sidebar", () => {
  function renderSidebar(sessions: SessionTab[], onReloadSession = vi.fn()) {
    render(
      <SessionSidebar
        sessions={sessions}
        activeSessionId={null}
        sessionStates={new Map()}
        onSessionSelect={vi.fn()}
        onNewSession={vi.fn()}
        onDeleteSession={vi.fn()}
        onRenameSession={vi.fn()}
        onReloadSession={onReloadSession}
        isCollapsed={false}
        onToggleCollapse={vi.fn()}
      />,
    );
    return onReloadSession;
  }

  it("reloads a live session from its row", () => {
    const onReload = renderSidebar([tab()]);
    fireEvent.click(screen.getByLabelText("Reload koko-1"));
    expect(onReload).toHaveBeenCalledWith("session-1");
  });

  // A disconnected tab is reconnected by clicking it, so no reload button.
  it("shows no reload button on a disconnected session", () => {
    renderSidebar([tab({ connected: false })]);
    expect(screen.queryByLabelText("Reload koko-1")).not.toBeInTheDocument();
  });
});

describe("useSessionTabs during a reload", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSessions.mockResolvedValue({
      sessions: [
        {
          slug: "koko-1",
          name: "koko",
          directory: "/repo",
          createdAt: 1,
          status: "disconnected",
          claudeSessionId: "conv-1",
        },
      ],
      recentDirs: [],
    });
  });

  async function loaded() {
    const hook = renderHook(() => useSessionTabs());
    await waitFor(() => expect(hook.result.current.tabs).toHaveLength(1));
    return hook;
  }

  // Reload closes the session it replaces, which fires its exit. That exit
  // must not mark the tab disconnected, or the reconnect card would flash.
  it("ignores the exit of the session being replaced", async () => {
    const { result } = await loaded();
    // Make the tab live first, as it is when reloaded.
    mockCreate.mockResolvedValueOnce("session-live");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]);
    });
    expect(result.current.tabs[0].connected).toBe(true);

    let release: (v: string) => void = () => {};
    mockCreate.mockImplementationOnce(
      () =>
        new Promise<string>((r) => {
          release = r;
        }),
    );
    let pending: Promise<string> | undefined;
    act(() => {
      pending = result.current.reconnectTab(result.current.tabs[0]);
    });

    // The backend kills the old process mid-reload, and its exit arrives.
    act(() => result.current.handleSessionExit("session-live"));
    // Still live: no reconnect card flashes up while the new one starts.
    expect(result.current.tabs[0].connected).toBe(true);

    await act(async () => {
      release("session-new");
      await pending;
    });
    expect(result.current.tabs[0].id).toBe("session-new");
    expect(result.current.tabs[0].connected).toBe(true);
  });

  // Outside a reconnect, an exit still marks the tab disconnected.
  it("still honours a real exit", async () => {
    const { result } = await loaded();
    mockCreate.mockResolvedValueOnce("session-new");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]);
    });
    act(() => result.current.handleSessionExit("session-new"));
    expect(result.current.tabs[0].connected).toBe(false);
  });

  // A failure other than a refusal comes after the backend closed the old
  // session, so the tab is disconnected. Its exit was ignored, so the hook
  // has to set it.
  it("marks the tab disconnected when a reload fails after closing", async () => {
    const { result } = await loaded();
    mockCreate.mockResolvedValueOnce("session-live");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]);
    });
    expect(result.current.tabs[0].connected).toBe(true);

    // The backend closed the old session before the new one failed, so it no
    // longer knows it.
    mockCreate.mockRejectedValueOnce("failed to start PTY: fork failed");
    mockSlug.mockRejectedValueOnce("session not found");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]).catch(() => {});
    });
    expect(result.current.tabs[0].connected).toBe(false);
    expect(result.current.tabs[0].reconnectError).toMatch(/Could not reconnect/);
  });

  // A failure before the backend closed anything, such as a bridge error,
  // used to be read from the error text and marked the live tab dead. The
  // backend is now asked, and the session is still there.
  it("keeps a live tab connected when a reload fails before closing", async () => {
    const { result } = await loaded();
    mockCreate.mockResolvedValueOnce("session-live");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]);
    });

    mockCreate.mockRejectedValueOnce("bridge error: call timed out");
    mockSlug.mockResolvedValueOnce("koko-1");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]).catch(() => {});
    });
    expect(result.current.tabs[0].connected).toBe(true);
  });

  // A refusal happens before anything is closed, so a live tab stays live.
  it("keeps a live tab connected when the reload is refused", async () => {
    const { result } = await loaded();
    mockCreate.mockResolvedValueOnce("session-live");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]);
    });

    mockCreate.mockRejectedValueOnce("that conversation is already open in another session");
    await act(async () => {
      await result.current.reconnectTab(result.current.tabs[0]).catch(() => {});
    });
    expect(result.current.tabs[0].connected).toBe(true);
  });
});
