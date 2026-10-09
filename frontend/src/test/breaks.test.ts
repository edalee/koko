import { describe, expect, it } from "vitest";
import {
  ACTIVE_WITHIN,
  type BreakSample,
  type BreakState,
  NEW_CYCLE,
  stepBreak,
} from "../lib/breaks";

const WORK = 90;
const BREAK = 15;

const tick: BreakSample = { elapsed: 1, slept: 0, idle: 0, seen: true, quiet: false };

// run steps the timer once a second for n seconds, with idle time from idleAt.
function run(
  s: BreakState,
  n: number,
  sample: Partial<BreakSample> = {},
  idleAt?: (i: number) => number,
): BreakState {
  let state = s;
  for (let i = 1; i <= n; i++) {
    const idle = idleAt ? idleAt(i) : (sample.idle ?? 0);
    state = stepBreak(state, { ...tick, ...sample, idle }, WORK, BREAK);
  }
  return state;
}

const nearBreak: BreakState = { ...NEW_CYCLE, workSeconds: WORK * 60 - 1 };
const onBreak: BreakState = { ...NEW_CYCLE, phase: "break", workSeconds: WORK * 60 };

describe("stepBreak", () => {
  it("brings the break after the work length of steady work", () => {
    const s = run(NEW_CYCLE, WORK * 60);
    expect(s.phase).toBe("break");
    expect(run(NEW_CYCLE, WORK * 60 - 1).phase).toBe("work");
  });

  it("counts a throttled gap with recent input as work", () => {
    const s = stepBreak(NEW_CYCLE, { ...tick, elapsed: 60, idle: 3 }, WORK, BREAK);
    expect(s.workSeconds).toBe(60);
  });

  it("counts only the time before your last input when you are idle", () => {
    const s = stepBreak(NEW_CYCLE, { ...tick, elapsed: 90, idle: 70 }, WORK, BREAK);
    expect(s.workSeconds).toBe(20);
  });

  it("does not count idle time as work", () => {
    const s = run({ ...NEW_CYCLE, workSeconds: 600, lastIdle: 100 }, 5 * 60, {}, (i) => 100 + i);
    expect(s.workSeconds).toBe(600);
  });

  it("starts a new cycle after idle as long as a break", () => {
    const s = run({ ...NEW_CYCLE, workSeconds: 3000 }, BREAK * 60, {}, (i) => i);
    expect(s.phase).toBe("work");
    expect(s.workSeconds).toBe(0);
  });

  it("keeps the cycle after a shorter idle", () => {
    let s = run({ ...NEW_CYCLE, workSeconds: 3000 }, 10 * 60, {}, (i) => i);
    s = run(s, 5);
    // The first 59 idle seconds still count as work, as a pause to read.
    expect(s.workSeconds).toBe(3000 + 59 + 5);
    expect(s.awaySeconds).toBe(0);
  });

  it("starts a new cycle after sleep as long as a break", () => {
    const s = stepBreak(
      { ...NEW_CYCLE, workSeconds: 3000 },
      { ...tick, elapsed: 20 * 60, slept: 20 * 60, idle: 1 },
      WORK,
      BREAK,
    );
    expect(s.workSeconds).toBe(0);
  });

  it("starts a new cycle after idle then sleep, together as long as a break", () => {
    // Six minutes idle, then ten asleep, idle not counting the sleep.
    let s = run({ ...NEW_CYCLE, workSeconds: 3000 }, 6 * 60, {}, (i) => i);
    expect(s.workSeconds).toBe(3000 + 59);
    s = stepBreak(s, { ...tick, elapsed: 10 * 60, slept: 10 * 60, idle: 6 * 60 + 1 }, WORK, BREAK);
    expect(s.workSeconds).toBe(0);
  });

  it("does not count sleep as work", () => {
    const s = stepBreak(
      { ...NEW_CYCLE, workSeconds: 100 },
      { ...tick, elapsed: 5 * 60, slept: 5 * 60 - 1, idle: 0 },
      WORK,
      BREAK,
    );
    expect(s.workSeconds).toBe(101);
  });

  it("ends a break that is due while you are away", () => {
    const s = run(nearBreak, BREAK * 60 + 1, { seen: false }, (i) => i);
    expect(s.phase).toBe("work");
    expect(s.workSeconds).toBe(0);
  });

  it("counts a break down while you can see it", () => {
    const s = run(onBreak, BREAK * 60);
    expect(s.phase).toBe("work");
    expect(s.workSeconds).toBe(0);
  });

  it("holds a break while Koko is hidden and you are active elsewhere", () => {
    const s = run(onBreak, 30 * 60, { seen: false, idle: 1 });
    expect(s.phase).toBe("break");
    expect(s.breakSeconds).toBe(0);
  });

  it("counts a break down while you are idle with Koko hidden", () => {
    const s = run(onBreak, 120, { seen: false }, (i) => ACTIVE_WITHIN + i);
    expect(s.breakSeconds).toBe(120);
  });

  it("pauses during quiet hours", () => {
    const s = run({ ...NEW_CYCLE, workSeconds: 100 }, 60, { quiet: true });
    expect(s.workSeconds).toBe(100);
  });

  it("counts awake time as work where idle is unknown", () => {
    let s = run(NEW_CYCLE, 30, { idle: -1 });
    expect(s.workSeconds).toBe(30);
    s = stepBreak(s, { ...tick, elapsed: 600, slept: 590, idle: -1 }, WORK, BREAK);
    expect(s.workSeconds).toBe(40);
    s = stepBreak(s, { ...tick, elapsed: BREAK * 60, slept: BREAK * 60, idle: -1 }, WORK, BREAK);
    expect(s.workSeconds).toBe(0);
  });

  it("starts with the idle time when you are already away", () => {
    const s = stepBreak(NEW_CYCLE, { ...tick, idle: BREAK * 60 }, WORK, BREAK);
    expect(s.awaySeconds).toBe(BREAK * 60);
    expect(s.workSeconds).toBe(0);
  });
});
