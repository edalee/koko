import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Activity } from "../../wailsjs/go/main/App";
import { GetConfig } from "../../wailsjs/go/main/ConfigService";
import { useSafeWorking, withDefaults } from "../hooks/useSafeWorking";

const config = {
  safeWorking: {
    quietHoursEnabled: false,
    quietHoursStart: "18:00",
    quietHoursEnd: "08:00",
    breakEnabled: true,
    workMinutes: 2,
    breakMinutes: 1,
  },
};

let activity = { idleSeconds: 0, peakIdleSeconds: 0, sleptSeconds: 0 };

// tick advances the clock one second and lets the sample's promise settle.
async function tick(n = 1) {
  for (let i = 0; i < n; i++) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
  }
}

describe("withDefaults", () => {
  it("treats 0 minutes as the defaults", () => {
    const c = withDefaults({ breakEnabled: true, workMinutes: 0, breakMinutes: 0 });
    expect(c.workMinutes).toBe(90);
    expect(c.breakMinutes).toBe(15);
  });

  it("keeps saved minutes", () => {
    const c = withDefaults({ workMinutes: 45, breakMinutes: 5 });
    expect(c.workMinutes).toBe(45);
    expect(c.breakMinutes).toBe(5);
  });
});

describe("useSafeWorking break timer", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    activity = { idleSeconds: 0, peakIdleSeconds: 0, sleptSeconds: 0 };
    vi.mocked(GetConfig).mockResolvedValue(config as never);
    vi.mocked(Activity).mockImplementation(async () => activity as never);
    vi.spyOn(document, "hasFocus").mockReturnValue(true);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("brings a break after the work time, then ends it", async () => {
    const { result } = renderHook(() => useSafeWorking());
    await tick(2 * 60 + 1);
    expect(result.current.isBreakTime).toBe(true);
    await tick(60);
    expect(result.current.isBreakTime).toBe(false);
  });

  it("measures wall-clock time, so a late timer loses nothing", async () => {
    const { result } = renderHook(() => useSafeWorking());
    await tick(2);
    // The clock jumps 90 seconds with no timer firing, as in a hidden window.
    vi.setSystemTime(Date.now() + 90_000);
    await tick(1);
    expect(result.current.workSecondsLeft).toBeLessThanOrEqual(120 - 92);
  });

  it("does not count sleep as work", async () => {
    const { result } = renderHook(() => useSafeWorking());
    await tick(2);
    activity = { ...activity, sleptSeconds: 30 };
    vi.setSystemTime(Date.now() + 30_000);
    await tick(1);
    // Two seconds of work, then one awake second. The 30 seconds asleep do
    // not count.
    expect(result.current.workSecondsLeft).toBeGreaterThanOrEqual(120 - 4);
  });
});
