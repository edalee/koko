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
  /**
   * The longest idle time since the last sample, from the backend. A late
   * timer can miss a time away that ended between samples. Defaults to idle.
   */
  peakIdle?: number;
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

// Timers and clocks are not exact, so with no input, idle can grow a little
// less than the awake time between samples. SLACK is the largest shortfall
// allowed, in seconds. The allowance is capped at half the awake time, so if
// any awake time passed, idle that has not grown means fresh input.
const SLACK = 2;

// The wall clock drifts against the monotonic clock by microseconds a second,
// so a sample shows a tiny slept time with no sleep at all. Less than this
// many seconds is not sleep.
const MIN_SLEEP = 1;

/** stepBreak returns the timer state after one sample. */
export function stepBreak(
  s: BreakState,
  x: BreakSample,
  workMinutes: number,
  breakMinutes: number,
): BreakState {
  const elapsed = Math.max(0, x.elapsed);
  const slept = x.slept >= MIN_SLEEP ? Math.min(elapsed, x.slept) : 0;
  const awake = elapsed - slept;
  const known = x.idle >= 0;
  const peak = Math.max(x.idle, x.peakIdle ?? x.idle);

  // The away run continues while idle keeps growing, and is never less than
  // the idle time. Fresh input restarts it at the idle time. If the idle time
  // is unknown (-1), only sleep counts as away.
  let awaySeconds: number;
  // ended is the away run that fresh input just ended, plus this sample's
  // sleep, as when a key press wakes the Mac. It is at least the peak idle
  // time. This sample's awake time does not count, because you may have been
  // working in another app.
  let ended = 0;
  // missed is the idle time inside this sample that fresh input ended, if the
  // run reached ACTIVE_WITHIN. It does not count as work. A shorter pause
  // counts. The part of the run before this sample was already counted, so
  // only its growth since the last sample is missed.
  let missed = 0;
  if (!known) awaySeconds = slept > 0 ? s.awaySeconds + slept : 0;
  else if (x.idle >= s.lastIdle + awake - Math.min(SLACK, awake / 2)) {
    awaySeconds = Math.max(s.awaySeconds + elapsed, x.idle);
  } else {
    ended = Math.max(s.awaySeconds + slept, peak);
    if (peak >= ACTIVE_WITHIN) missed = Math.min(awake, Math.max(0, peak - s.lastIdle));
    awaySeconds = x.idle;
  }
  const lastIdle = known ? x.idle : 0;

  if (awaySeconds >= breakMinutes * 60 || ended >= breakMinutes * 60) {
    return { ...NEW_CYCLE, awaySeconds, lastIdle };
  }
  if (x.quiet) return { ...s, awaySeconds, lastIdle };

  const active = !known || x.idle < ACTIVE_WITHIN;
  if (s.phase === "work") {
    // When you are not active, only the time before your last input counts.
    const work = active ? awake - missed : Math.max(0, awake - x.idle);
    const workSeconds = s.workSeconds + work;
    if (workSeconds >= workMinutes * 60) {
      return { phase: "break", workSeconds, breakSeconds: 0, awaySeconds, lastIdle };
    }
    return { ...s, workSeconds, awaySeconds, lastIdle };
  }

  // A break counts down while Koko has focus, or while you are away. If
  // another app has focus and you are active, the break waits. If the idle
  // time is unknown, Koko cannot tell, so the break always counts down.
  const away = !known || !active || slept > 0 || missed > 0;
  const breakSeconds = s.breakSeconds + (x.seen || away ? elapsed : 0);
  if (breakSeconds >= breakMinutes * 60) {
    return { ...NEW_CYCLE, awaySeconds, lastIdle };
  }
  return { ...s, breakSeconds, awaySeconds, lastIdle };
}
