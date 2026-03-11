import { useCallback, useEffect, useRef, useState } from "react";
import { GetConfig, SaveConfig } from "../../wailsjs/go/main/ConfigService";

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

export interface UseSafeWorkingResult {
  config: SafeWorkingConfig;
  updateConfig: (config: SafeWorkingConfig) => Promise<void>;
  isQuietHours: boolean;
  quietResumeTime: Date | null;
  isBreakTime: boolean;
  breakSecondsLeft: number;
  workSecondsLeft: number;
  skipBreak: () => void;
}

export function useSafeWorking(hasActiveSession: boolean): UseSafeWorkingResult {
  const [config, setConfig] = useState<SafeWorkingConfig>(DEFAULT_CONFIG);
  const [isQuietHours, setIsQuietHours] = useState(false);
  const [quietResumeTime, setQuietResumeTime] = useState<Date | null>(null);
  const [isBreakTime, setIsBreakTime] = useState(false);
  const [breakSecondsLeft, setBreakSecondsLeft] = useState(0);
  const [workSecondsLeft, setWorkSecondsLeft] = useState(0);
  const workElapsedRef = useRef(0);
  const breakElapsedRef = useRef(0);

  // Load config on mount
  useEffect(() => {
    GetConfig()
      .then((appConfig) => {
        if (appConfig.safeWorking) {
          setConfig(appConfig.safeWorking);
        }
      })
      .catch(() => {});
  }, []);

  // Save config
  const updateConfig = useCallback(async (newConfig: SafeWorkingConfig) => {
    setConfig(newConfig);
    try {
      const appConfig = await GetConfig();
      appConfig.safeWorking = newConfig;
      await SaveConfig(appConfig);
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
      const inQuiet = isInQuietWindow(config.quietHoursStart, config.quietHoursEnd);
      setIsQuietHours(inQuiet);
      setQuietResumeTime(inQuiet ? quietHoursResumeTime(config.quietHoursEnd) : null);
    }

    check();
    const id = setInterval(check, 30_000);
    return () => clearInterval(id);
  }, [config.quietHoursEnabled, config.quietHoursStart, config.quietHoursEnd]);

  // Break timer — ticks every second
  useEffect(() => {
    if (!config.breakEnabled || !hasActiveSession) {
      // Reset when disabled or no session
      if (!config.breakEnabled) {
        workElapsedRef.current = 0;
        breakElapsedRef.current = 0;
        setIsBreakTime(false);
        setBreakSecondsLeft(0);
        setWorkSecondsLeft(0);
      }
      return;
    }

    const workDuration = config.workMinutes * 60;
    const breakDuration = config.breakMinutes * 60;

    const id = setInterval(() => {
      if (isQuietHours) return; // Pause during quiet hours

      if (!isBreakTime) {
        // Working phase
        workElapsedRef.current += 1;
        const remaining = workDuration - workElapsedRef.current;
        setWorkSecondsLeft(Math.max(0, remaining));

        if (workElapsedRef.current >= workDuration) {
          // Time for a break
          setIsBreakTime(true);
          breakElapsedRef.current = 0;
          setBreakSecondsLeft(breakDuration);
        }
      } else {
        // Break phase
        breakElapsedRef.current += 1;
        const remaining = breakDuration - breakElapsedRef.current;
        setBreakSecondsLeft(Math.max(0, remaining));

        if (breakElapsedRef.current >= breakDuration) {
          // Break over
          setIsBreakTime(false);
          workElapsedRef.current = 0;
          setWorkSecondsLeft(workDuration);
        }
      }
    }, 1000);

    // Initialize display
    if (!isBreakTime) {
      setWorkSecondsLeft(config.workMinutes * 60 - workElapsedRef.current);
    }

    return () => clearInterval(id);
  }, [
    config.breakEnabled,
    config.workMinutes,
    config.breakMinutes,
    hasActiveSession,
    isQuietHours,
    isBreakTime,
  ]);

  const skipBreak = useCallback(() => {
    setIsBreakTime(false);
    workElapsedRef.current = 0;
    breakElapsedRef.current = 0;
    setWorkSecondsLeft(config.workMinutes * 60);
    setBreakSecondsLeft(0);
  }, [config.workMinutes]);

  return {
    config,
    updateConfig,
    isQuietHours,
    quietResumeTime,
    isBreakTime,
    breakSecondsLeft,
    workSecondsLeft,
    skipBreak,
  };
}
