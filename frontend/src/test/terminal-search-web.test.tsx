import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

let mockSelection = "";
let mockHasSelection = false;

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
    hasSelection = () => mockHasSelection;
    getSelection = () => mockSelection;
    attachCustomKeyEventHandler = vi.fn();
    loadAddon = vi.fn();
    dispose = vi.fn();
    focus = vi.fn();
    cols = 80;
    rows = 24;
  }
  return { Terminal: MockTerminal };
});

vi.mock("@xterm/addon-fit", () => {
  class MockFitAddon {
    fit = vi.fn();
    proposeDimensions = vi.fn(() => ({ cols: 80, rows: 24 }));
    dispose = vi.fn();
  }
  return { FitAddon: MockFitAddon };
});

vi.mock("@xterm/addon-serialize", () => {
  class MockSerializeAddon {
    serializeAsHTML = vi.fn(() => "");
    dispose = vi.fn();
  }
  return { SerializeAddon: MockSerializeAddon };
});

vi.mock("@xterm/addon-webgl", () => {
  class MockWebglAddon {
    onContextLoss = vi.fn();
    clearTextureAtlas = vi.fn();
    dispose = vi.fn();
  }
  return { WebglAddon: MockWebglAddon };
});

vi.mock("@xterm/addon-web-links", () => {
  class MockWebLinksAddon {
    dispose = vi.fn();
  }
  return { WebLinksAddon: MockWebLinksAddon };
});

globalThis.ResizeObserver = class {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
} as unknown as typeof ResizeObserver;

import TerminalPane from "../components/TerminalPane";

/** Open the terminal context menu and return the "Search Web" item. */
async function openMenu() {
  const { container } = render(<TerminalPane sessionId="session-1" active={true} />);
  // The contextmenu listener lives on the inner xterm container, and the event
  // bubbles up — so it has to be fired on that element, not the outer pane.
  const pane = container.querySelector("div.p-1") as HTMLElement;
  await act(async () => {
    fireEvent.contextMenu(pane);
  });
  return screen.getByText("Search Web");
}

describe("terminal context menu — Search Web", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSelection = "";
    mockHasSelection = false;
  });

  it("is disabled when nothing is selected", async () => {
    const item = await openMenu();
    expect(item).toBeDisabled();
  });

  it("opens the selection in the default browser as a search query", async () => {
    mockHasSelection = true;
    mockSelection = "ENOTFOUND registry.npmjs.org";

    const item = await openMenu();
    expect(item).not.toBeDisabled();

    await act(async () => {
      fireEvent.mouseDown(item);
    });

    expect(BrowserOpenURL).toHaveBeenCalledWith(
      "https://www.google.com/search?q=ENOTFOUND%20registry.npmjs.org",
    );
  });

  it("collapses whitespace and newlines from a multi-line selection", async () => {
    mockHasSelection = true;
    mockSelection = "  panic: runtime error\n    goroutine 1  [running]:  \n";

    const item = await openMenu();
    await act(async () => {
      fireEvent.mouseDown(item);
    });

    const url = (BrowserOpenURL as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
    expect(decodeURIComponent(url.split("q=")[1])).toBe(
      "panic: runtime error goroutine 1 [running]:",
    );
  });

  it("caps a very long selection so the URL stays sane", async () => {
    mockHasSelection = true;
    mockSelection = "x".repeat(5000);

    const item = await openMenu();
    await act(async () => {
      fireEvent.mouseDown(item);
    });

    const url = (BrowserOpenURL as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
    expect(decodeURIComponent(url.split("q=")[1])).toHaveLength(400);
  });

  it("caps without splitting a surrogate pair", async () => {
    mockHasSelection = true;
    // Emoji straddles the 400 cap — a raw .slice() would leave a lone
    // surrogate and encodeURIComponent would throw URIError.
    mockSelection = `${"x".repeat(399)}🎉${"y".repeat(100)}`;

    const item = await openMenu();
    await act(async () => {
      fireEvent.mouseDown(item);
    });

    const url = (BrowserOpenURL as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
    const query = decodeURIComponent(url.split("q=")[1]);
    expect(query).toBe(`${"x".repeat(399)}🎉`);
  });

  it("does nothing when the selection is only whitespace", async () => {
    mockHasSelection = true;
    mockSelection = "   \n  \n";

    const item = await openMenu();
    await act(async () => {
      fireEvent.mouseDown(item);
    });

    expect(BrowserOpenURL).not.toHaveBeenCalled();
  });
});
