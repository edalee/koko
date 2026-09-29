import {
  Check,
  CircleAlert,
  Eye,
  EyeOff,
  FlaskConical,
  Loader2,
  Play,
  Plus,
  X,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import {
  Check as CheckConnections,
  GetConfig,
  ReadLog,
  RunNow,
  SaveConfig,
  SetEnabled,
  Status,
} from "../../wailsjs/go/main/WorkerService";
import { cn } from "../lib/utils";

// Mirrors cmd/koko-worker/config.go. The worker fills in defaults for missing
// or empty values, so an empty text box keeps the default.
interface JobConfig {
  enabled: boolean;
  times: string[];
}

interface WorkerConfig {
  enabled: boolean;
  timeZone: string;
  calendarId: string;
  tonoPath: string;
  tonoOwnPRsOnly: boolean;
  tonoTeam: string;
  tonoMaxAgeDays: number;
  slack: { botToken: string; userId: string };
  focus: { windowStart: string; windowEnd: string; minMinutes: number };
  jobs: Record<string, JobConfig>;
}

interface RunRecord {
  status: "ok" | "failed" | "retry" | "offline";
  at: string;
  message?: string;
}

interface WorkerStatus {
  enabled: boolean;
  agentLoaded: boolean;
  nextRun?: string;
  wakeBooked?: string;
  configError?: string;
  jobs: Record<string, { lastRun?: RunRecord }>;
}

interface CheckResult {
  name: string;
  ok: boolean;
  detail: string;
  fix?: string;
}

const DEFAULTS: WorkerConfig = {
  enabled: false,
  timeZone: "Europe/Stockholm",
  calendarId: "primary",
  tonoPath: "",
  tonoOwnPRsOnly: false,
  tonoTeam: "epidemicsound/content-protection",
  tonoMaxAgeDays: 4,
  slack: { botToken: "", userId: "" },
  focus: { windowStart: "09:00", windowEnd: "17:00", minMinutes: 30 },
  jobs: {
    standup: { enabled: true, times: ["07:00"] },
    focus: { enabled: true, times: ["09:15"] },
    tono: { enabled: true, times: ["09:30", "12:00", "14:00"] },
  },
};

const JOBS: { key: string; name: string; about: string; multi: boolean }[] = [
  {
    key: "standup",
    name: "Stand-up",
    about: "Today's meetings, approved PRs ready to merge, and PRs waiting for your review.",
    multi: false,
  },
  {
    key: "focus",
    name: "Focus time",
    about: "Books focus blocks in free gaps of 30 minutes or more.",
    multi: false,
  },
  {
    key: "tono",
    name: "Tono reviews",
    about:
      "Reviews each new commit on your open PRs and on the PRs waiting for your review. Posts nothing to GitHub.",
    multi: true,
  },
];

const STATUS_LABELS: Record<string, string> = {
  ok: "ok",
  failed: "failed",
  retry: "failed, trying again",
  offline: "waiting for internet",
};

const CONNECTION_NAMES: Record<string, string> = {
  slack: "Slack",
  github: "GitHub",
  claude: "Claude Code",
  jira: "Jira",
  calendar: "Google Calendar",
  tono: "Tono CLI",
  wake: "Mac wake",
};

function merge(raw: string): WorkerConfig {
  const parsed = JSON.parse(raw || "{}") as Partial<WorkerConfig>;
  return {
    ...DEFAULTS,
    ...parsed,
    slack: { ...DEFAULTS.slack, ...parsed.slack },
    focus: { ...DEFAULTS.focus, ...parsed.focus },
    jobs: { ...DEFAULTS.jobs, ...parsed.jobs },
  };
}

function formatWhen(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function Toggle({ id, on, onClick }: { id: string; on: boolean; onClick: () => void }) {
  return (
    <button
      id={id}
      type="button"
      onClick={onClick}
      className={cn(
        "w-8 h-[18px] rounded-full transition-colors relative shrink-0",
        on ? "bg-accent" : "bg-white/[0.12]",
      )}
    >
      <span
        className={cn(
          "absolute top-[2px] size-[14px] rounded-full bg-white transition-transform",
          on ? "translate-x-[16px]" : "translate-x-[2px]",
        )}
      />
    </button>
  );
}

const inputClass =
  "px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40";

export default function WorkerSettings() {
  const [cfg, setCfg] = useState<WorkerConfig>(DEFAULTS);
  const [status, setStatus] = useState<WorkerStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);
  const [showToken, setShowToken] = useState(false);
  const [checks, setChecks] = useState<CheckResult[] | null>(null);
  const [checking, setChecking] = useState(false);
  const [running, setRunning] = useState<string | null>(null);
  const [output, setOutput] = useState<{ job: string; text: string } | null>(null);
  const [log, setLog] = useState<string | null>(null);

  const refreshStatus = useCallback(() => {
    Status()
      .then((s) => setStatus(JSON.parse(s)))
      .catch(() => setStatus(null));
  }, []);

  useEffect(() => {
    GetConfig()
      .then((raw) => setCfg(merge(raw)))
      .catch((e) => setError(String(e)));
    refreshStatus();
    const timer = setInterval(refreshStatus, 30_000);
    return () => clearInterval(timer);
  }, [refreshStatus]);

  const save = useCallback(async (next: WorkerConfig) => {
    setCfg(next);
    setError(null);
    try {
      await SaveConfig(JSON.stringify(next));
    } catch (e) {
      setError(String(e));
    }
  }, []);

  const setJob = (key: string, job: JobConfig) =>
    save({ ...cfg, jobs: { ...cfg.jobs, [key]: job } });

  const handleSwitch = async () => {
    const next = !cfg.enabled;
    setSwitching(true);
    setError(null);
    try {
      await SetEnabled(next);
      setCfg({ ...cfg, enabled: next });
      refreshStatus();
    } catch (e) {
      setError(String(e));
    } finally {
      setSwitching(false);
    }
  };

  const handleCheck = async () => {
    setChecking(true);
    try {
      setChecks(JSON.parse(await CheckConnections("")));
    } catch (e) {
      setError(String(e));
    } finally {
      setChecking(false);
    }
  };

  const handleRun = async (job: string, test: boolean) => {
    setRunning(job + (test ? ":test" : ""));
    setOutput(null);
    try {
      const text = await RunNow(job, test);
      setOutput({
        job,
        text: text.trim() || (test ? "(no output)" : "Done. Check your Slack DMs."),
      });
      refreshStatus();
    } catch (e) {
      setOutput({ job, text: String(e) });
    } finally {
      setRunning(null);
    }
  };

  const agentState = !status
    ? "unknown"
    : status.configError
      ? "config error"
      : status.agentLoaded
        ? "running"
        : "stopped";

  return (
    <div className="space-y-4 pt-3 border-t border-white/[0.06]">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm text-white font-medium">Worker</h4>
          <p className="text-xs text-muted-foreground mt-1">
            Stand-up, focus time and tono reviews on weekdays. Runs in the background, even with
            Koko closed.
          </p>
        </div>
        {switching ? (
          <Loader2 className="size-4 animate-spin text-muted-foreground shrink-0" />
        ) : (
          <Toggle id="worker-toggle" on={cfg.enabled} onClick={handleSwitch} />
        )}
      </div>

      <p className="text-[10px] text-tertiary">
        Agent:{" "}
        <span className={cn(agentState === "running" ? "text-success" : "text-white/70")}>
          {agentState}
        </span>
        {status?.nextRun && <> · next run {formatWhen(status.nextRun)}</>}
        {status?.wakeBooked && <> · Mac wakes {formatWhen(status.wakeBooked)}</>}
      </p>
      {status?.configError && <p className="text-[10px] text-error">{status.configError}</p>}
      {error && <p className="text-[10px] text-error">{error}</p>}

      {/* Slack */}
      <div className="space-y-2">
        <p className="text-xs text-white/80">Slack DMs</p>
        <div className="relative">
          <input
            type={showToken ? "text" : "password"}
            value={cfg.slack.botToken}
            onChange={(e) => setCfg({ ...cfg, slack: { ...cfg.slack, botToken: e.target.value } })}
            onBlur={() => save(cfg)}
            placeholder="Bot token, xoxb-..."
            className={cn(inputClass, "w-full py-1.5 pr-9 font-mono placeholder:text-tertiary")}
          />
          <button
            type="button"
            onClick={() => setShowToken(!showToken)}
            className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted-foreground hover:text-white transition-colors"
          >
            {showToken ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
          </button>
        </div>
        <input
          type="text"
          value={cfg.slack.userId}
          onChange={(e) => setCfg({ ...cfg, slack: { ...cfg.slack, userId: e.target.value } })}
          onBlur={() => save(cfg)}
          placeholder="Your Slack member ID, U02F4AZV2"
          className={cn(inputClass, "w-full py-1.5 font-mono placeholder:text-tertiary")}
        />
        <p className="text-[10px] text-tertiary">
          A separate bot from the Slack Bot above. It needs the chat:write scope.
        </p>
      </div>

      {/* Jobs */}
      <div className="space-y-3">
        {JOBS.map(({ key, name, about, multi }) => {
          const job = cfg.jobs[key] ?? DEFAULTS.jobs[key];
          const last = status?.jobs?.[key]?.lastRun;
          return (
            <div
              key={key}
              className="space-y-2 p-2.5 rounded-lg bg-white/[0.02] border border-white/[0.05]"
            >
              <div className="flex items-center justify-between gap-2">
                <label htmlFor={`job-${key}`} className="text-xs text-white/85">
                  {name}
                </label>
                <Toggle
                  id={`job-${key}`}
                  on={job.enabled}
                  onClick={() => setJob(key, { ...job, enabled: !job.enabled })}
                />
              </div>
              <p className="text-[10px] text-tertiary">{about}</p>

              <div className="flex flex-wrap items-center gap-1.5">
                {job.times.map((t, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: each time is edited in place by its position
                  <span key={`${key}-${i}`} className="inline-flex items-center gap-1">
                    <input
                      type="time"
                      value={t}
                      onChange={(e) => {
                        const times = [...job.times];
                        times[i] = e.target.value;
                        setJob(key, { ...job, times });
                      }}
                      className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
                    />
                    {multi && job.times.length > 1 && (
                      <button
                        type="button"
                        onClick={() =>
                          setJob(key, { ...job, times: job.times.filter((_, j) => j !== i) })
                        }
                        className="opacity-50 hover:opacity-100 hover:text-error"
                        title="Remove this time"
                      >
                        <X className="size-3" />
                      </button>
                    )}
                  </span>
                ))}
                {multi && (
                  <button
                    type="button"
                    onClick={() => setJob(key, { ...job, times: [...job.times, "16:00"] })}
                    className="p-1 rounded-md border border-white/[0.08] hover:bg-white/[0.06]"
                    title="Add a time"
                  >
                    <Plus className="size-3" />
                  </button>
                )}
                <span className="text-[10px] text-tertiary">Mon–Fri</span>
              </div>

              {key === "focus" && (
                <div className="flex items-center gap-2 text-[10px] text-tertiary">
                  <span>Between</span>
                  <input
                    type="time"
                    value={cfg.focus.windowStart}
                    onChange={(e) =>
                      save({ ...cfg, focus: { ...cfg.focus, windowStart: e.target.value } })
                    }
                    className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
                  />
                  <span>and</span>
                  <input
                    type="time"
                    value={cfg.focus.windowEnd}
                    onChange={(e) =>
                      save({ ...cfg, focus: { ...cfg.focus, windowEnd: e.target.value } })
                    }
                    className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
                  />
                  <span>, gaps of</span>
                  <input
                    type="number"
                    min={5}
                    step={5}
                    value={cfg.focus.minMinutes}
                    onChange={(e) =>
                      save({
                        ...cfg,
                        focus: {
                          ...cfg.focus,
                          minMinutes: Number.parseInt(e.target.value, 10) || 0,
                        },
                      })
                    }
                    className={cn(inputClass, "w-14 tabular-nums")}
                  />
                  <span>min or more</span>
                </div>
              )}

              {key === "tono" && (
                <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                  <span>PRs opened by</span>
                  <input
                    type="text"
                    value={cfg.tonoTeam}
                    onChange={(e) => setCfg({ ...cfg, tonoTeam: e.target.value })}
                    onBlur={() => save(cfg)}
                    placeholder="org/team-slug"
                    className={cn(
                      inputClass,
                      "flex-1 min-w-40 font-mono placeholder:text-tertiary",
                    )}
                  />
                  <span>in the last</span>
                  <input
                    type="number"
                    min={1}
                    value={cfg.tonoMaxAgeDays}
                    onChange={(e) =>
                      save({ ...cfg, tonoMaxAgeDays: Number.parseInt(e.target.value, 10) || 0 })
                    }
                    className={cn(inputClass, "w-12 tabular-nums")}
                  />
                  <span>days, one review each</span>
                </div>
              )}

              {key === "tono" && (
                <div className="flex items-center justify-between gap-2">
                  <label htmlFor="tono-queue" className="text-[10px] text-tertiary">
                    Also review PRs waiting for your review. Results go in the stand-up.
                  </label>
                  <Toggle
                    id="tono-queue"
                    on={!cfg.tonoOwnPRsOnly}
                    onClick={() => save({ ...cfg, tonoOwnPRsOnly: !cfg.tonoOwnPRsOnly })}
                  />
                </div>
              )}

              {key === "tono" && (
                <input
                  type="text"
                  value={cfg.tonoPath}
                  onChange={(e) => setCfg({ ...cfg, tonoPath: e.target.value })}
                  onBlur={() => save(cfg)}
                  placeholder="Path to the tono CLI (default: tonometer repo)"
                  className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
                />
              )}

              <div className="flex items-center justify-between gap-2">
                <p className="text-[10px] text-tertiary truncate" title={last?.message}>
                  {last ? (
                    <>
                      Last run {formatWhen(last.at)}:{" "}
                      <span
                        className={cn(
                          last.status === "ok" && "text-success",
                          last.status === "failed" && "text-error",
                          (last.status === "retry" || last.status === "offline") && "text-warning",
                        )}
                      >
                        {STATUS_LABELS[last.status] ?? last.status}
                      </span>
                    </>
                  ) : (
                    "Not run yet"
                  )}
                </p>
                <div className="flex items-center gap-1 shrink-0">
                  <button
                    type="button"
                    disabled={running !== null}
                    onClick={() => handleRun(key, true)}
                    className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40"
                    title="Print the result here. Sends and books nothing."
                  >
                    {running === `${key}:test` ? (
                      <Loader2 className="size-3 animate-spin" />
                    ) : (
                      <FlaskConical className="size-3" />
                    )}
                    <span className="text-white">Test</span>
                  </button>
                  <button
                    type="button"
                    disabled={running !== null}
                    onClick={() => handleRun(key, false)}
                    className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40"
                    title="Run it for real now"
                  >
                    {running === key ? (
                      <Loader2 className="size-3 animate-spin" />
                    ) : (
                      <Play className="size-3" />
                    )}
                    <span className="text-white">Run now</span>
                  </button>
                </div>
              </div>

              {output?.job === key && (
                <pre className="max-h-64 overflow-auto p-2 text-[10px] leading-relaxed bg-black/30 rounded-md text-white/75 whitespace-pre-wrap">
                  {output.text}
                </pre>
              )}
            </div>
          );
        })}
      </div>

      {/* Connections */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <p className="text-xs text-white/80">Connections</p>
          <button
            type="button"
            onClick={handleCheck}
            disabled={checking}
            className="flex items-center gap-1.5 px-2.5 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40"
          >
            {checking && <Loader2 className="size-3 animate-spin" />}
            <span className="text-white">{checking ? "Checking…" : "Check all"}</span>
          </button>
        </div>
        {checks?.map((c) => (
          <div key={c.name} className="space-y-0.5">
            <div className="flex items-center gap-1.5 text-[11px]">
              {c.ok ? (
                <Check className="size-3 text-success shrink-0" />
              ) : (
                <CircleAlert className="size-3 text-error shrink-0" />
              )}
              <span className="text-white/85">{CONNECTION_NAMES[c.name] ?? c.name}</span>
              <span className="text-tertiary truncate">{c.detail}</span>
            </div>
            {c.fix && (
              <p className="pl-[18px] text-[10px] text-muted-foreground whitespace-pre-wrap font-mono select-text">
                {c.fix}
              </p>
            )}
          </div>
        ))}
        {!checks && !checking && (
          <p className="text-[10px] text-tertiary">
            Checks Slack, GitHub, Jira, Google Calendar, tono and Mac wake.
          </p>
        )}
      </div>

      {/* Log */}
      <div className="space-y-1">
        <button
          type="button"
          onClick={async () => setLog(log === null ? await ReadLog(40).catch(() => "") : null)}
          className="text-[10px] text-tertiary hover:text-muted-foreground"
        >
          {log === null ? "Show log" : "Hide log"}
        </button>
        {log !== null && (
          <pre className="max-h-48 overflow-auto p-2 text-[10px] bg-black/30 rounded-md text-white/60 whitespace-pre-wrap">
            {log || "The log is empty."}
          </pre>
        )}
      </div>

      <p className="text-[10px] text-tertiary">
        Worker config stored at ~/Library/Application Support/koko/worker.json
      </p>
    </div>
  );
}
