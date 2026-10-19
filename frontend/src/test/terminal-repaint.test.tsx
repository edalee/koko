import { act, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Resize } from "../../wailsjs/go/main/TerminalManager";

const refreshFn = vi.fn();
const fitFn = vi.fn();
const loadAddonFn = vi.fn();

// Every WebGL addon the pane creates, with the handler it gave onContextLoss.
const webglAddons: { dispose: ReturnType<typeof vi.fn>; lose: () => void }[] = [];

// xterm mocks must be classes (used with `new`)
vi.mock("@xterm/xterm", () => {
  class MockTerminal {
    open = vi.fn();
    write = vi.fn();
    writeln = vi.fn();
    onData = vi.fn(() => ({ dispose: vi.fn() }));
    onBinary = vi.fn(() => ({ dispose: vi.fn() }));
    attachCustomKeyEventHandler = vi.fn();
    hasSelection = vi.fn(() => false);
    loadAddon = loadAddonFn;
    refresh = refreshFn;
    dispose = vi.fn();
    focus = vi.fn();
    cols = 80;
    rows = 24;
  }
  return { Terminal: MockTerminal };
});

vi.mock("@xterm/addon-fit", () => {
  class MockFitAddon {
    fit = fitFn;
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
    dispose = vi.fn();
    clearTextureAtlas = vi.fn();
    onContextLoss = vi.fn((handler: () => void) => {
      webglAddons.push({ dispose: this.dispose, lose: handler });
    });
  }
  return { WebglAddon: MockWebglAddon };
});

vi.mock("@xterm/addon-web-links", () => {
  class MockWebLinksAddon {
    dispose = vi.fn();
  }
  return { WebLinksAddon: MockWebLinksAddon };
});

// Suppress ResizeObserver in jsdom
globalThis.ResizeObserver = class {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
} as unknown as typeof ResizeObserver;

import TerminalPane from "../components/TerminalPane";

async function flush() {
  await act(async () => {
    await new Promise((r) => requestAnimationFrame(r));
  });
}

/**
 * A pane can hold its content in the buffer and still show a blank screen,
 * after a lost WebGL context or a sleep. A scroll or a resize redraws it.
 * These tests pin the repaints that make the scroll unnecessary.
 */
describe("TerminalPane repaint", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    webglAddons.length = 0;
  });

  it("repaints every row on tab switch even when the size is unchanged", async () => {
    const { rerender } = render(<TerminalPane sessionId="s1" active={false} />);
    await flush();
    // Mount fits and attaches to the PTY. Only the tab switch is under test.
    refreshFn.mockClear();
    fitFn.mockClear();
    (Resize as ReturnType<typeof vi.fn>).mockClear();

    rerender(<TerminalPane sessionId="s1" active={true} />);
    await flush();

    expect(refreshFn).toHaveBeenCalledWith(0, 23);
    // A repaint is not a resize: no refit and no SIGWINCH.
    expect(fitFn).not.toHaveBeenCalled();
    expect(Resize).not.toHaveBeenCalled();
  });

  it("falls back and repaints on a lost WebGL context, then retries WebGL on activation", async () => {
    const { rerender } = render(<TerminalPane sessionId="s2" active={true} />);
    await flush();
    expect(webglAddons).toHaveLength(1);
    refreshFn.mockClear();

    act(() => webglAddons[0].lose());

    expect(webglAddons[0].dispose).toHaveBeenCalled();
    expect(refreshFn).toHaveBeenCalledWith(0, 23);

    rerender(<TerminalPane sessionId="s2" active={false} />);
    rerender(<TerminalPane sessionId="s2" active={true} />);
    await flush();

    expect(webglAddons).toHaveLength(2);
  });

  it("does not load a second WebGL addon while the first is alive", async () => {
    const { rerender } = render(<TerminalPane sessionId="s3" active={true} />);
    rerender(<TerminalPane sessionId="s3" active={false} />);
    rerender(<TerminalPane sessionId="s3" active={true} />);
    await flush();

    expect(webglAddons).toHaveLength(1);
  });

  it("repaints the active pane when the window is shown again", async () => {
    render(<TerminalPane sessionId="s4" active={true} />);
    await flush();
    refreshFn.mockClear();

    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(refreshFn).toHaveBeenCalledWith(0, 23);
  });

  it("leaves background panes alone when the window is shown again", async () => {
    render(<TerminalPane sessionId="s5" active={false} />);
    await flush();
    refreshFn.mockClear();

    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(refreshFn).not.toHaveBeenCalled();
  });
});
