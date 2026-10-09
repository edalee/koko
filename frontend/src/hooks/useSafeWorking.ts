import { useCallback, useEffect, useRef, useState } from "react";
import { Activity } from "../../wailsjs/go/main/App";
import { GetConfig, SaveConfig } from "../../wailsjs/go/main/ConfigService";
import { type BreakState, NEW_CYCLE, stepBreak } from "../lib/breaks";

export interface SafeWorkingConfig {
  quietHoursEnabled: boolean;
  quietHoursStart: string; // "HH:MM"
  quietHoursEnd: string;
  breakEnabled: boolean;
  workMinutes: number;
  breakMinutes: number;
}

const DEFAULT_CONFIG: SafeWorkingConfig = {
  quietHoursEnabled: false,
  quietHoursStart: "18:00",
  quietHoursEnd: "08:00",
  breakEnabled: false,
  workMinutes: 90,
  breakMinutes: 15,
};

function parseHHMM(time: string): { h: number; m: number } {
  const [h, m] = time.split(":").map(Number);
  return { h: h || 0, m: m || 0 };
}

function isInQuietWindow(start: string, end: string): boolean {
  const now = new Date();
  const nowMins = now.getHours() * 60 + now.getMinutes();
  const { h: sh, m: sm } = parseHHMM(start);
  const { h: eh, m: em } = parseHHMM(end);
  const startMins = sh * 60 + sm;
  const endMins = eh * 60 + em;

  if (startMins <= endMins) {
    // Same-day window (e.g., 09:00–17:00)
    return nowMins >= startMins && nowMins < endMins;
  }
  // Overnight window (e.g., 18:00–08:00)
  return nowMins >= startMins || nowMins < endMins;
}

function quietHoursResumeTime(end: string): Date {
  const { h, m } = parseHHMM(end);
  const now = new Date();
  const resume = new Date(now);
  resume.setHours(h, m, 0, 0);
  // If resume time is in the past, it's tomorrow
  if (resume <= now) {
    resume.setDate(resume.getDate() + 1);
  }
  return resume;
}

// withDefaults fills missing fields from the defaults. The backend saves 0
// for minutes it never had, and a 0-minute break would restart the cycle
// every second, so 0 also takes the default.
export function withDefaults(saved: Partial<SafeWorkingConfig>): SafeWorkingConfig {
  const config = { ...DEFAULT_CONFIG, ...saved };
  if (!(config.workMinutes > 0)) config.workMinutes = DEFAULT_CONFIG.workMinutes;
  if (!(config.breakMinutes > 0)) config.breakMinutes = DEFAULT_CONFIG.breakMinutes;
  return config;
}

export interface UseSafeWorkingResult {
  config: SafeWorkingConfig;
  updateConfig: (config: SafeWorkingConfig) => Promise<void>;
  isQuietHours: boolean;
  quietResumeTime: Date | null;
  isBreakTime: boolean;
  breakSecondsLeft: number;
  workSecondsLeft: number;
  skipBreak: () => void;
  delayQuietHours: () => void;
}

export function useSafeWorking(): UseSafeWorkingResult {
  const [config, setConfig] = useState<SafeWorkingConfig>(DEFAULT_CONFIG);
  const [isQuietHours, setIsQuietHours] = useState(false);
  const [quietResumeTime, setQuietResumeTime] = useState<Date | null>(null);
  const [isBreakTime, setIsBreakTime] = useState(false);
  const [breakSecondsLeft, setBreakSecondsLeft] = useState(0);
  const [workSecondsLeft, setWorkSecondsLeft] = useState(0);
  const breakRef = useRef<BreakState>(NEW_CYCLE);
  const isQuietHoursRef = useRef(false);
  const delayUntilRef = useRef(0);

  // Load config on mount
  useEffect(() => {
    GetConfig()
      .then((appConfig) => {
        if (appConfig.safeWorking) setConfig(withDefaults(appConfig.safeWorking));
      })
      .catch(() => {});
  }, []);

  // Save config
  const updateConfig = useCallback(async (newConfig: SafeWorkingConfig) => {
    setConfig(newConfig);
    try {
      const appConfig = await GetConfig();
      // Spread into plain object to avoid Wails class serialization issues
      await SaveConfig({
        ...appConfig,
        safeWorking: { ...newConfig },
      } as typeof appConfig);
    } catch {
      // ignore
    }
  }, []);

  // Quiet hours check — every 30s
  useEffect(() => {
    if (!config.quietHoursEnabled) {
      setIsQuietHours(false);
      setQuietResumeTime(null);
      return;
    }

    function check() {
      if (Date.now() < delayUntilRef.current) return;
      const inQuiet = isInQuietWindow(config.quietHoursStart, config.quietHoursEnd);
      setIsQuietHours(inQuiet);
      setQuietResumeTime(inQuiet ? quietHoursResumeTime(config.quietHoursEnd) : null);
    }

    check();
    const id = setInterval(check, 30_000);
    return () => clearInterval(id);
  }, [config.quietHoursEnabled, config.quietHoursStart, config.quietHoursEnd]);

  // Keep refs in sync with state
  useEffect(() => {
    isQuietHoursRef.current = isQuietHours;
  }, [isQuietHours]);

  // show publishes the break state to React.
  const show = useCallback((b: BreakState, workMinutes: number, breakMinutes: number) => {
    setIsBreakTime(b.phase === "break");
    setWorkSecondsLeft(Math.max(0, Math.round(workMinutes * 60 - b.workSeconds)));
    setBreakSecondsLeft(
      b.phase === "break" ? Math.max(0, Math.round(breakMinutes * 60 - b.breakSeconds)) : 0,
    );
  }, []);

  // Break timer (plan 033). It samples every second, but measures wall-clock
  // time, so a slowed or stopped timer loses no time. Activity() gives the idle
  // time, the longest idle time since the last sample, and the time slept.
  // stepBreak in lib/breaks.ts applies the rules.
  useEffect(() => {
    if (!config.breakEnabled) {
      breakRef.current = NEW_CYCLE;
      show(NEW_CYCLE, config.workMinutes, config.breakMinutes);
      return;
    }

    let last = Date.now();
    let lastSlept: number | null = null;
    let busy = false;
    let stopped = false;
    show(breakRef.current, config.workMinutes, config.breakMinutes);

    const sample = async () => {
      if (busy || stopped) return;
      busy = true;
      try {
        const activity = await Activity().catch(() => null);
        if (stopped) return;
        const now = Date.now();
        const sleptSeconds = activity?.sleptSeconds ?? lastSlept ?? 0;
        const next = stepBreak(
          breakRef.current,
          {
            elapsed: (now - last) / 1000,
            slept: lastSlept === null ? 0 : sleptSeconds - lastSlept,
            idle: activity?.idleSeconds ?? -1,
            peakIdle: activity?.peakIdleSeconds,
            seen: document.visibilityState === "visible" && document.hasFocus(),
            quiet: isQuietHoursRef.current,
          },
          config.workMinutes,
          config.breakMinutes,
        );
        last = now;
        lastSlept = sleptSeconds;
        breakRef.current = next;
        show(next, config.workMinutes, config.breakMinutes);
      } finally {
        busy = false;
      }
    };

    const id = setInterval(sample, 1000);
    return () => {
      stopped = true;
      clearInterval(id);
    };
  }, [config.breakEnabled, config.workMinutes, config.breakMinutes, show]);

  const delayQuietHours = useCallback(() => {
    delayUntilRef.current = Date.now() + 30 * 60 * 1000;
    setIsQuietHours(false);
    setQuietResumeTime(null);
  }, []);

  const skipBreak = useCallback(() => {
    breakRef.current = { ...NEW_CYCLE, lastIdle: breakRef.current.lastIdle };
    show(breakRef.current, config.workMinutes, config.breakMinutes);
  }, [config.workMinutes, config.breakMinutes, show]);

  return {
    config,
    updateConfig,
    isQuietHours,
    quietResumeTime,
    isBreakTime,
    breakSecondsLeft,
    workSecondsLeft,
    skipBreak,
    delayQuietHours,
  };
}
