// Break timer rules (plan 033). A break is a rest for your eyes, away from
// the keyboard. Work is time you are active at the computer, in any app. Time
// away as long as a break counts as the break.

/** Input within this many seconds means you are active. */
export const ACTIVE_WITHIN = 60;

export interface BreakState {
  phase: "work" | "break";
  workSeconds: number;
  breakSeconds: number;
  /** Time since your last input, sleep included. */
  awaySeconds: number;
  /** The idle time at the last sample, to spot fresh input. */
  lastIdle: number;
}

export interface BreakSample {
  /** Wall-clock seconds since the last sample. */
  elapsed: number;
  /** Seconds of that time the computer slept. */
  slept: number;
  /** Seconds since the last input, or -1 if unknown. */
  idle: number;
  /** Whether you can see Koko: its window has focus. */
  seen: boolean;
  /** Quiet hours pause the timer. */
  quiet: boolean;
}

export const NEW_CYCLE: BreakState = {
  phase: "work",
  workSeconds: 0,
  breakSeconds: 0,
  awaySeconds: 0,
  lastIdle: 0,
};

// Timers and clocks are not exact, so idle can grow a little less than the
// time between samples with no input at all. The slack is at most half that
// time, so idle that stands still always means fresh input.
const SLACK = 2;

/** stepBreak returns the timer state after one sample. */
export function stepBreak(
  s: BreakState,
  x: BreakSample,
  workMinutes: number,
  breakMinutes: number,
): BreakState {
  const elapsed = Math.max(0, x.elapsed);
  const slept = Math.min(elapsed, Math.max(0, x.slept));
  const awake = elapsed - slept;
  const known = x.idle >= 0;

  // The away run continues while idle keeps growing, and is never less than
  // the idle time. Fresh input restarts it at the idle time. With no idle
  // time, only sleep counts as away.
  let awaySeconds: number;
  if (!known) awaySeconds = slept > 0 ? s.awaySeconds + slept : 0;
  else if (x.idle >= s.lastIdle + awake - Math.min(SLACK, awake / 2)) {
    awaySeconds = Math.max(s.awaySeconds + elapsed, x.idle);
  } else awaySeconds = x.idle;
  const lastIdle = known ? x.idle : 0;

  if (awaySeconds >= breakMinutes * 60) {
    return { ...NEW_CYCLE, awaySeconds, lastIdle };
  }
  if (x.quiet) return { ...s, awaySeconds, lastIdle };

  const active = !known || x.idle < ACTIVE_WITHIN;
  if (s.phase === "work") {
    // When you are not active, only the time before your last input counts.
    const work = active ? awake : Math.max(0, awake - x.idle);
    const workSeconds = s.workSeconds + work;
    if (workSeconds >= workMinutes * 60) {
      return { phase: "break", workSeconds, breakSeconds: 0, awaySeconds, lastIdle };
    }
    return { ...s, workSeconds, awaySeconds, lastIdle };
  }

  // A break counts down while you can see it, or while you are away. If Koko
  // is hidden and you are working elsewhere, it waits for you.
  const away = !active || slept > 0;
  const breakSeconds = s.breakSeconds + (x.seen || away ? elapsed : 0);
  if (breakSeconds >= breakMinutes * 60) {
    return { ...NEW_CYCLE, awaySeconds, lastIdle };
  }
  return { ...s, breakSeconds, awaySeconds, lastIdle };
}
