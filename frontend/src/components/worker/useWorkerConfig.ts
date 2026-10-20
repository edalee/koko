import { useCallback, useEffect, useRef, useState } from "react";
import { GetConfig, SaveConfig, SetEnabled, Status } from "../../../wailsjs/go/main/WorkerService";
import {
  canonicalZone,
  DEFAULTS,
  lunchProblem,
  merge,
  type ReviewerConfig,
  type WorkerConfig,
  type WorkerStatus,
} from "./config";

// useWorkerConfig loads worker.json and the worker's status, and holds the
// helpers every section uses to change and save the config.
export function useWorkerConfig() {
  const [cfg, setCfg] = useState<WorkerConfig>(DEFAULTS);
  const [status, setStatus] = useState<WorkerStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  // lunchDraft is the lunch times as typed. cfg.lunch keeps the last valid
  // times, so another save never writes a lunch the worker would reject.
  const [lunchDraft, setLunchDraft] = useState({
    start: DEFAULTS.lunch.start,
    end: DEFAULTS.lunch.end,
  });
  const [lunchError, setLunchError] = useState<string | null>(null);
  // loaded turns true once reading worker.json ends, whether the read worked or failed.
  const [loaded, setLoaded] = useState(false);
  // savedZone is the time zone last read or saved, for a typo to fall back to.
  const savedZone = useRef(DEFAULTS.timeZone);
  // switching is true while the master switch runs. The sections block their
  // edits then, because SetEnabled writes worker.json itself.
  const [switching, setSwitching] = useState(false);

  const refreshStatus = useCallback(() => {
    Status()
      .then((s) => setStatus(JSON.parse(s)))
      .catch(() => setStatus(null));
  }, []);

  useEffect(() => {
    GetConfig()
      .then((raw) => {
        const loaded = merge(raw);
        savedZone.current = loaded.timeZone;
        setCfg(loaded);
      })
      .catch((e) => setError(String(e)))
      .finally(() => setLoaded(true));
    refreshStatus();
    const timer = setInterval(refreshStatus, 30_000);
    return () => clearInterval(timer);
  }, [refreshStatus]);

  useEffect(() => {
    setLunchDraft({ start: cfg.lunch.start, end: cfg.lunch.end });
  }, [cfg.lunch.start, cfg.lunch.end]);

  const save = useCallback(async (next: WorkerConfig) => {
    setCfg(next);
    setError(null);
    savedZone.current = next.timeZone;
    try {
      await SaveConfig(JSON.stringify(next));
    } catch (e) {
      setError(String(e));
    }
  }, []);

  // saveField saves one field of the Calendar and Jira section. An empty time
  // zone becomes the Mac's zone. A name that is not a real zone goes back to
  // the saved zone, because the worker would stop every job on it, and any
  // later save would write it.
  const saveField = (key: "timeZone" | "calendarId" | "jiraSite") => {
    if (key !== "timeZone") return save(cfg);
    const zone = canonicalZone(cfg.timeZone.trim() || DEFAULTS.timeZone);
    if (!zone) {
      setCfg({ ...cfg, timeZone: savedZone.current });
      setError(`"${cfg.timeZone}" is not a time zone. Use a name such as Europe/London.`);
      return;
    }
    return save({ ...cfg, timeZone: zone });
  };

  // saveLunchTimes saves the typed lunch once focus leaves both time boxes,
  // and only if lunch ends after it starts. The worker stops every job on a
  // config error.
  const saveLunchTimes = () => {
    const problem = lunchProblem(lunchDraft.start, lunchDraft.end);
    setLunchError(problem);
    if (problem || (lunchDraft.start === cfg.lunch.start && lunchDraft.end === cfg.lunch.end)) {
      return;
    }
    return save({ ...cfg, lunch: { ...cfg.lunch, ...lunchDraft } });
  };

  // toggleLunch switches lunch on or off, and drops times not yet saved.
  const toggleLunch = () => {
    setLunchDraft({ start: cfg.lunch.start, end: cfg.lunch.end });
    setLunchError(null);
    return save({ ...cfg, lunch: { ...cfg.lunch, enabled: !cfg.lunch.enabled } });
  };

  // setReviewer changes reviewer fields as you type, without saving. ReviewCard
  // saves each text box on blur. saveReviewer saves at once, for switches and numbers.
  const setReviewer = (patch: Partial<ReviewerConfig>) =>
    setCfg({ ...cfg, reviewer: { ...cfg.reviewer, ...patch } });
  const saveReviewer = (patch: Partial<ReviewerConfig>) =>
    save({ ...cfg, reviewer: { ...cfg.reviewer, ...patch } });

  // setEnabled turns the worker on or off. It sets enabled on the newest
  // config, so it never writes back a copy from before the switch.
  const setEnabled = async (next: boolean) => {
    setSwitching(true);
    setError(null);
    try {
      await SetEnabled(next);
      setCfg((c) => ({ ...c, enabled: next }));
      refreshStatus();
    } catch (e) {
      setError(String(e));
    } finally {
      setSwitching(false);
    }
  };

  return {
    cfg,
    switching,
    setEnabled,
    loaded,
    setCfg,
    status,
    refreshStatus,
    error,
    setError,
    save,
    saveField,
    lunchDraft,
    setLunchDraft,
    lunchError,
    saveLunchTimes,
    toggleLunch,
    setReviewer,
    saveReviewer,
  };
}

export type WorkerConfigApi = ReturnType<typeof useWorkerConfig>;
