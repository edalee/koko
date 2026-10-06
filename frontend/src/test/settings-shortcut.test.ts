import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useOverlay } from "../hooks/useOverlay";

function press(key: string, opts: KeyboardEventInit = {}) {
  act(() => {
    window.dispatchEvent(new KeyboardEvent("keydown", { key, ...opts }));
  });
}

describe("Settings shortcut", () => {
  it("opens and closes Settings with Cmd+,", () => {
    const { result } = renderHook(() => useOverlay());
    expect(result.current.activeOverlay).toBeNull();

    press(",", { metaKey: true });
    expect(result.current.activeOverlay).toBe("settings");

    press(",", { metaKey: true });
    expect(result.current.activeOverlay).toBeNull();
  });

  it("closes Settings with Escape", () => {
    const { result } = renderHook(() => useOverlay());
    press(",", { metaKey: true });
    press("Escape");
    expect(result.current.activeOverlay).toBeNull();
  });

  it("ignores a plain comma and Cmd+Shift+,", () => {
    const { result } = renderHook(() => useOverlay());
    press(",");
    press(",", { metaKey: true, shiftKey: true });
    expect(result.current.activeOverlay).toBeNull();
  });
});
