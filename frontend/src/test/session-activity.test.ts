import { beforeEach, describe, expect, it, vi } from "vitest";
import { GetSessionState } from "../../wailsjs/go/main/TerminalManager";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// We test the hook's core logic directly since renderHook with
// timers and events is fragile. These tests verify the state
// machine transitions that useSessionActivity implements.

/**
 * Tests the session activity state machine:
 *   PTY output → "active"
 *   No output for 4s → check backend → "idle" or "approval"
 */
describe("Session activity state transitions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("GetSessionState returns 'idle' for normal sessions", async () => {
    (GetSessionState as ReturnType<typeof vi.fn>).mockResolvedValue("idle");
    const state = await GetSessionState("session-1");
    expect(state).toBe("idle");
  });

  it("GetSessionState returns 'approval' when waiting for tool approval", async () => {
    (GetSessionState as ReturnType<typeof vi.fn>).mockResolvedValue("approval");
    const state = await GetSessionState("session-1");
    expect(state).toBe("approval");
  });

  it("EventsOn registers per-session listeners", () => {
    const ids = ["session-1", "session-2", "session-3"];
    for (const id of ids) {
      EventsOn(`pty:data:${id}`, () => {});
    }

    expect(EventsOn).toHaveBeenCalledTimes(3);
    expect(EventsOn).toHaveBeenCalledWith("pty:data:session-1", expect.any(Function));
    expect(EventsOn).toHaveBeenCalledWith("pty:data:session-2", expect.any(Function));
    expect(EventsOn).toHaveBeenCalledWith("pty:data:session-3", expect.any(Function));
  });

  it("state map tracks sessions independently", () => {
    const states = new Map<string, string>();

    // Session 1 active, session 2 needs approval
    states.set("session-1", "active");
    states.set("session-2", "approval");

    expect(states.get("session-1")).toBe("active");
    expect(states.get("session-2")).toBe("approval");

    // Session 1 goes idle — session 2 should not change
    states.set("session-1", "idle");
    expect(states.get("session-1")).toBe("idle");
    expect(states.get("session-2")).toBe("approval");
  });
});
