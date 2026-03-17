import { act, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Resize } from "../../wailsjs/go/main/TerminalManager";

// Track fit() and proposeDimensions() calls
const fitFn = vi.fn();
const proposeDimensionsFn = vi.fn();

let mockTermCols = 80;
let mockTermRows = 24;

// xterm mocks must be classes (used with `new`)
vi.mock("@xterm/xterm", () => {
  class MockTerminal {
    open = vi.fn();
    write = vi.fn();
    writeln = vi.fn();
    onData = vi.fn(() => ({ dispose: vi.fn() }));
    onBinary = vi.fn(() => ({ dispose: vi.fn() }));
    onWriteParsed = vi.fn(() => ({ dispose: vi.fn() }));
    onScroll = vi.fn(() => ({ dispose: vi.fn() }));
    scrollToBottom = vi.fn();
    buffer = { active: { viewportY: 0, baseY: 0 } };
    loadAddon = vi.fn();
    dispose = vi.fn();
    focus = vi.fn();
    get cols() {
      return mockTermCols;
    }
    get rows() {
      return mockTermRows;
    }
  }
  return { Terminal: MockTerminal };
});

vi.mock("@xterm/addon-fit", () => {
  class MockFitAddon {
    fit = fitFn;
    proposeDimensions = proposeDimensionsFn;
    dispose = vi.fn();
  }
  return { FitAddon: MockFitAddon };
});

vi.mock("@xterm/addon-webgl", () => {
  class MockWebglAddon {
    onContextLoss = vi.fn();
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

// Suppress ResizeObserver in jsdom
globalThis.ResizeObserver = class {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
} as unknown as typeof ResizeObserver;

import TerminalPane from "../components/TerminalPane";

/**
 * Tests that TerminalPane only calls fit() and Resize() when terminal
 * dimensions actually change. This prevents unnecessary PTY SIGWINCH
 * signals that disrupt Claude's scroll position.
 */
describe("TerminalPane resize guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTermCols = 80;
    mockTermRows = 24;
    proposeDimensionsFn.mockReturnValue({ cols: 80, rows: 24 });
  });

  it("does not call fit() on tab switch when dimensions are unchanged", async () => {
    const { rerender } = render(<TerminalPane sessionId="session-1" active={false} />);

    fitFn.mockClear();
    (Resize as ReturnType<typeof vi.fn>).mockClear();

    // Proposed dimensions match current (80x24) — no resize needed
    proposeDimensionsFn.mockReturnValue({ cols: 80, rows: 24 });

    rerender(<TerminalPane sessionId="session-1" active={true} />);

    await act(async () => {
      await new Promise((r) => requestAnimationFrame(r));
    });

    // fit() should NOT be called — dimensions haven't changed
    expect(fitFn).not.toHaveBeenCalled();
    expect(Resize).not.toHaveBeenCalled();
  });

  it("relies on ResizeObserver (not tab switch) to trigger fit()", async () => {
    // With the scroll-pinning rewrite, fit() is no longer called on
    // active prop change — ResizeObserver handles all resize detection.
    const { rerender } = render(<TerminalPane sessionId="session-1" active={false} />);

    fitFn.mockClear();
    (Resize as ReturnType<typeof vi.fn>).mockClear();

    proposeDimensionsFn.mockReturnValue({ cols: 120, rows: 30 });

    rerender(<TerminalPane sessionId="session-1" active={true} />);

    await act(async () => {
      await new Promise((r) => requestAnimationFrame(r));
    });

    // fit() should NOT be called on tab switch — ResizeObserver handles it
    expect(fitFn).not.toHaveBeenCalled();
  });

  it("ResizeObserver skips fit when dimensions are unchanged", async () => {
    // Capture the ResizeObserver callback
    let observerCallback: ResizeObserverCallback | null = null;
    globalThis.ResizeObserver = class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
      constructor(cb: ResizeObserverCallback) {
        observerCallback = cb;
      }
    } as unknown as typeof ResizeObserver;

    render(<TerminalPane sessionId="session-2" active={true} />);

    fitFn.mockClear();
    (Resize as ReturnType<typeof vi.fn>).mockClear();

    // Proposed dimensions match current — no actual resize
    proposeDimensionsFn.mockReturnValue({ cols: 80, rows: 24 });

    // Trigger ResizeObserver
    if (observerCallback) {
      (observerCallback as ResizeObserverCallback)([], {} as ResizeObserver);
    }

    // Wait for debounce (50ms)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 60));
    });

    expect(fitFn).not.toHaveBeenCalled();
    expect(Resize).not.toHaveBeenCalled();
  });
});
