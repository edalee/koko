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

interface ScopeConfig {
  enabled: boolean;
  maxAgeDays: number;
}

interface ReviewerConfig {
  source: "managed" | "local";
  repo: string;
  branch: string;
  autoUpdate: boolean;
  script: string;
  path: string;
  command: string;
  logs: string;
  markerPrefix: string;
  mine: ScopeConfig;
  team: ScopeConfig & { team: string; repos: string[] };
}

interface WorkerConfig {
  enabled: boolean;
  timeZone: string;
  calendarId: string;
  reviewer: ReviewerConfig;
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
  version?: string;
  reviewer?: {
    source: string;
    cli: string;
    repo?: string;
    branch?: string;
    commit?: string;
    commitBranch?: string;
    updated?: string;
  };
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
  // No reviewer is set up, so the review job starts off.
  reviewer: {
    source: "managed",
    repo: "",
    branch: "main",
    autoUpdate: true,
    script: "",
    path: "",
    command: "",
    logs: "",
    markerPrefix: "",
    mine: { enabled: true, maxAgeDays: 14 },
    team: { enabled: false, maxAgeDays: 4, team: "", repos: [] },
  },
  slack: { botToken: "", userId: "" },
  focus: { windowStart: "09:00", windowEnd: "17:00", minMinutes: 30 },
  jobs: {
    standup: { enabled: true, times: ["07:00"] },
    focus: { enabled: true, times: ["09:15"] },
    review: { enabled: false, times: ["09:30", "12:00", "14:00"] },
  },
};

const JOBS: { key: string; name: string; about: string; multi: boolean }[] = [
  {
    key: "standup",
    name: "Stand-up",
    about:
      "Today's meetings, approved PRs ready to merge, your team's PRs waiting for your review, and team PRs.",
    multi: false,
  },
  {
    key: "focus",
    name: "Focus time",
    about: "Books focus blocks in free gaps of 30 minutes or more.",
    multi: false,
  },
  {
    key: "review",
    name: "Review worker",
    about:
      "Reviews each PR once and posts the review as comments on the PR, as you. If there is nothing to report, it posts LGTM 😃⭐😸. One Slack line says each review is posted. Test prints the comments instead.",
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
  reviewer: "Reviewer",
  wake: "Mac wake",
};

// The tono settings a worker.json from before 0.5.8 meant when it left them
// out. Only the migration uses them. The worker's Config.migrate does the same.
const LEGACY = {
  args: "{pr} --all -l high -R {repo}",
  logs: "~/.cache/tono/logs",
  marker: "tono",
  repo: "epidemicsound/tonometer",
  script: "tono",
  team: "epidemicsound/content-protection",
  claudeEnv: "TONO_CLAUDE={claude} ",
};

// The reviewer settings before 0.5.8, replaced by command and logs.
interface LegacyCommand {
  format?: string;
  args?: string[];
}

// quoteWord keeps a word with spaces or quotes as one word of the command.
function quoteWord(w: string): string {
  if (!/[ \t"']/.test(w)) return w;
  return w.includes("'") ? `"${w}"` : `'${w}'`;
}

// migratedCommand is the command and logs folder for a reviewer from before
// 0.5.8: tono, run with its Claude wrapper and the old arguments. Its logs were
// read unless the format was "result-json".
function migratedCommand(old: LegacyCommand): { command: string; logs: string } {
  const kept = (old.args ?? [])
    .map((a) => a.trim())
    .filter(Boolean)
    .map(quoteWord);
  return {
    command: `${LEGACY.claudeEnv}{reviewer} ${kept.length ? kept.join(" ") : LEGACY.args}`,
    logs: old.format === "result-json" ? "" : LEGACY.logs,
  };
}

// The flat tono settings worker.json held before 0.5.4. merge() carries them
// over once, like the worker's Config.migrate, and the next save drops them.
interface LegacyConfig {
  tonoPath?: string;
  tonoMine?: boolean;
  tonoMineMaxAgeDays?: number;
  tonoTeamPRs?: boolean;
  tonoOwnPRsOnly?: boolean;
  tonoTeam?: string;
  tonoMaxAgeDays?: number;
  tonoRepos?: string[];
}

function migrateReviewer(old: LegacyConfig): ReviewerConfig {
  const d = DEFAULTS.reviewer;
  const path = (old.tonoPath ?? "").trim();
  return {
    ...d,
    repo: LEGACY.repo,
    script: LEGACY.script,
    ...migratedCommand({}),
    markerPrefix: LEGACY.marker,
    ...(path ? { source: "local" as const, path } : {}),
    mine: {
      enabled: old.tonoMine ?? d.mine.enabled,
      maxAgeDays: old.tonoMineMaxAgeDays || d.mine.maxAgeDays,
    },
    team: {
      enabled: old.tonoOwnPRsOnly ? false : (old.tonoTeamPRs ?? true),
      maxAgeDays: old.tonoMaxAgeDays || d.team.maxAgeDays,
      team: old.tonoTeam || LEGACY.team,
      repos: old.tonoRepos ?? [],
    },
  };
}

export function merge(raw: string): WorkerConfig {
  const {
    tonoPath,
    tonoMine,
    tonoMineMaxAgeDays,
    tonoTeamPRs,
    tonoOwnPRsOnly,
    tonoTeam,
    tonoMaxAgeDays,
    tonoRepos,
    ...parsed
  } = JSON.parse(raw || "{}") as Partial<WorkerConfig> & LegacyConfig;
  const legacy = {
    tonoPath,
    tonoMine,
    tonoMineMaxAgeDays,
    tonoTeamPRs,
    tonoOwnPRsOnly,
    tonoTeam,
    tonoMaxAgeDays,
    tonoRepos,
  };
  const hasLegacy = Object.values(legacy).some((v) => v !== undefined);
  let r: Partial<ReviewerConfig> & LegacyCommand =
    parsed.reviewer ?? (hasLegacy ? migrateReviewer(legacy) : DEFAULTS.reviewer);
  // A reviewer section without the command key is from before 0.5.8. An
  // empty command stays empty: only a missing key counts as old.
  if (!("command" in r)) {
    r = { ...r, ...migratedCommand(r), markerPrefix: r.markerPrefix?.trim() || LEGACY.marker };
  }
  const { format: _format, args: _args, ...reviewer } = r;
  const { tono, ...jobs } = parsed.jobs ?? {};
  return {
    ...DEFAULTS,
    ...parsed,
    reviewer: {
      ...DEFAULTS.reviewer,
      ...reviewer,
      mine: { ...DEFAULTS.reviewer.mine, ...reviewer.mine },
      team: { ...DEFAULTS.reviewer.team, ...reviewer.team, repos: reviewer.team?.repos ?? [] },
    },
    slack: { ...DEFAULTS.slack, ...parsed.slack },
    focus: { ...DEFAULTS.focus, ...parsed.focus },
    jobs: { ...DEFAULTS.jobs, ...(tono && !jobs.review ? { review: tono } : {}), ...jobs },
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
          "absolute left-0 top-[2px] size-[14px] rounded-full bg-white transition-transform",
          on ? "translate-x-[16px]" : "translate-x-[2px]",
        )}
      />
    </button>
  );
}

// RepoList adds and removes "owner/repo" names. A bare name means <org>/<name>,
// where org is the team's org. With no org, it needs owner/repo.
function RepoList({
  repos,
  org,
  onChange,
}: {
  repos: string[];
  org: string;
  onChange: (next: string[]) => void;
}) {
  const [draft, setDraft] = useState("");
  const [problem, setProblem] = useState<string | null>(null);

  const add = () => {
    const name = draft.trim();
    setProblem(null);
    if (!name) return;
    if (!/^[A-Za-z0-9._-]+(\/[A-Za-z0-9._-]+)?$/.test(name)) {
      setProblem(org ? "Use owner/repo, or a bare repo name" : "Use owner/repo");
      return;
    }
    if (!name.includes("/") && !org) {
      setProblem("Use owner/repo, or set the team first");
      return;
    }
    const full = name.includes("/") ? name : `${org}/${name}`;
    if (repos.some((r) => r.toLowerCase() === full.toLowerCase())) {
      setProblem("Already in the list");
      return;
    }
    onChange([...repos, full]);
    setDraft("");
  };

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1.5">
        {repos.map((repo) => (
          <span
            key={repo}
            className="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-md bg-white/[0.05] border border-white/[0.08] text-white/85 font-mono"
          >
            {repo}
            <button
              type="button"
              onClick={() => onChange(repos.filter((r) => r !== repo))}
              className="opacity-50 hover:opacity-100 hover:text-error transition-opacity"
              title={`Remove ${repo}`}
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        {repos.length === 0 && (
          <span className="text-[10px] text-tertiary">
            No repos yet. Only PRs that ask for your review are reviewed.
          </span>
        )}
      </div>
      <div className="flex gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add();
            }
          }}
          placeholder={org ? `owner/repo, or repo for ${org}` : "owner/repo"}
          className={cn(inputClass, "flex-1 py-1.5 font-mono placeholder:text-tertiary")}
        />
        <button
          type="button"
          onClick={add}
          disabled={!draft.trim()}
          className="flex items-center gap-1 px-2.5 py-1.5 text-xs rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Plus className="size-3" />
          <span className="text-white">Add</span>
        </button>
      </div>
      {problem && <p className="text-[10px] text-error">{problem}</p>}
    </div>
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

  // setReviewer changes a text box as you type, and onBlur saves it.
  // saveReviewer saves at once, for switches and numbers.
  const setReviewer = (patch: Partial<ReviewerConfig>) =>
    setCfg({ ...cfg, reviewer: { ...cfg.reviewer, ...patch } });
  const saveReviewer = (patch: Partial<ReviewerConfig>) =>
    save({ ...cfg, reviewer: { ...cfg.reviewer, ...patch } });

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
        text:
          text.trim() ||
          (test
            ? "(no output)"
            : job === "review"
              ? "Done. The reviews are on the PRs."
              : "Done. Check your Slack DMs."),
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
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm text-white font-medium">Worker</h4>
          <p className="text-xs text-muted-foreground mt-1">
            Stand-up, focus time and PR reviews on weekdays. Runs in the background, even with Koko
            closed.
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
        <p className="text-[10px] text-tertiary">It needs the chat:write scope.</p>
      </div>

      {/* Jobs */}
      <div className="space-y-5">
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

              {key === "review" && (
                <div className="space-y-2.5 pt-1">
                  <div>
                    <p className="text-xs text-white/80">Reviewer</p>
                    <p className="text-[10px] text-tertiary">
                      Any CLI that follows the reviewer contract (plans 030 and 031). It reviews one
                      PR and never posts. The worker posts its comments.
                    </p>
                  </div>
                  <div className="space-y-2 text-[10px] text-tertiary">
                    <div className="flex items-center gap-2">
                      <span className="w-14 shrink-0">Command</span>
                      <input
                        type="text"
                        value={cfg.reviewer.command}
                        onChange={(e) => setReviewer({ command: e.target.value })}
                        onBlur={() => save(cfg)}
                        placeholder="{reviewer} {pr} --all"
                        className={cn(inputClass, "flex-1 font-mono placeholder:text-tertiary")}
                      />
                    </div>
                    <p>
                      {"{reviewer}"} is the CLI set below. {"{pr}"}, {"{repo}"}, {"{url}"} and{" "}
                      {"{sha}"} are filled in for each PR, and {"{claude}"} is a claude that cannot
                      post. NAME=value words at the start set environment variables.
                    </p>
                    <div className="flex items-center gap-2">
                      <span className="w-14 shrink-0">Logs</span>
                      <input
                        type="text"
                        value={cfg.reviewer.logs}
                        onChange={(e) => setReviewer({ logs: e.target.value })}
                        onBlur={() => save(cfg)}
                        placeholder="Off"
                        className={cn(inputClass, "flex-1 font-mono placeholder:text-tertiary")}
                      />
                    </div>
                    <p>
                      Off: the reviewer writes its comments as JSON to $REVIEW_RESULT. A folder: the
                      worker reads the review from the reviewer's logs there instead.
                    </p>
                    <div className="flex items-center gap-2">
                      <span className="w-14 shrink-0">Marker</span>
                      <input
                        type="text"
                        value={cfg.reviewer.markerPrefix}
                        onChange={(e) => setReviewer({ markerPrefix: e.target.value })}
                        onBlur={() => save(cfg)}
                        placeholder="acme"
                        className={cn(inputClass, "w-28 font-mono placeholder:text-tertiary")}
                      />
                    </div>
                    <p>
                      Each comment starts with {"<!-- marker:"}. A PR with a comment behind the
                      marker counts as reviewed.
                    </p>
                  </div>
                  <p className="text-[10px] text-tertiary">
                    Where {"{reviewer}"} comes from. Managed: the worker keeps its own clone, so the
                    branch you have checked out never changes what runs.
                  </p>
                  <div className="flex gap-1.5">
                    {(["managed", "local"] as const).map((src) => (
                      <button
                        key={src}
                        type="button"
                        onClick={() =>
                          // A local source saves once it has a path, from the
                          // path box below. Saved without one, it would fail reviews.
                          src === "local" && !cfg.reviewer.path.trim()
                            ? setReviewer({ source: src })
                            : saveReviewer({ source: src })
                        }
                        className={cn(
                          "px-2.5 py-1 text-[11px] rounded-md border transition-colors",
                          cfg.reviewer.source === src
                            ? "bg-white/[0.08] border-white/[0.12] text-white"
                            : "border-white/[0.06] text-tertiary hover:text-muted-foreground",
                        )}
                      >
                        {src === "managed" ? "Managed" : "Local path"}
                      </button>
                    ))}
                  </div>
                  {cfg.reviewer.source === "managed" ? (
                    <>
                      <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                        <span>Repo</span>
                        <input
                          type="text"
                          value={cfg.reviewer.repo}
                          onChange={(e) => setReviewer({ repo: e.target.value })}
                          onBlur={() => save(cfg)}
                          placeholder="owner/repo"
                          className={cn(inputClass, "flex-1 min-w-40 font-mono")}
                        />
                        <span>branch</span>
                        <input
                          type="text"
                          value={cfg.reviewer.branch}
                          onChange={(e) => setReviewer({ branch: e.target.value })}
                          onBlur={() => save(cfg)}
                          placeholder="main"
                          className={cn(inputClass, "w-28 font-mono")}
                        />
                      </div>
                      <div className="flex items-center justify-between gap-2">
                        <label htmlFor="reviewer-update" className="text-[10px] text-tertiary">
                          Update to the newest commit on the branch before each run
                        </label>
                        <Toggle
                          id="reviewer-update"
                          on={cfg.reviewer.autoUpdate}
                          onClick={() => saveReviewer({ autoUpdate: !cfg.reviewer.autoUpdate })}
                        />
                      </div>
                      <p className="text-[10px] text-tertiary">
                        {status?.reviewer?.commit
                          ? `Last run used ${status.reviewer.commitBranch} @ ${status.reviewer.commit}${
                              status.reviewer.updated
                                ? `, updated ${formatWhen(status.reviewer.updated)}`
                                : ""
                            }.`
                          : "Not cloned yet. The next review run clones it."}
                      </p>
                    </>
                  ) : (
                    <input
                      type="text"
                      value={cfg.reviewer.path}
                      onChange={(e) => setReviewer({ path: e.target.value })}
                      onBlur={() => cfg.reviewer.path.trim() && save(cfg)}
                      placeholder="Path to the reviewer CLI, for example ~/repos/reviewer/review"
                      className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
                    />
                  )}
                </div>
              )}

              {key === "review" && (
                <div className="space-y-2 pt-3 border-t border-white/[0.05]">
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <label htmlFor="review-mine" className="text-xs text-white/80">
                        My PRs
                      </label>
                      <p className="text-[10px] text-tertiary">
                        Your open PRs, outside archived repos.
                      </p>
                    </div>
                    <Toggle
                      id="review-mine"
                      on={cfg.reviewer.mine.enabled}
                      onClick={() =>
                        saveReviewer({
                          mine: { ...cfg.reviewer.mine, enabled: !cfg.reviewer.mine.enabled },
                        })
                      }
                    />
                  </div>
                  {cfg.reviewer.mine.enabled && (
                    <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                      <span>Opened in the last</span>
                      <input
                        type="number"
                        min={1}
                        value={cfg.reviewer.mine.maxAgeDays}
                        onChange={(e) =>
                          saveReviewer({
                            mine: {
                              ...cfg.reviewer.mine,
                              maxAgeDays: Number.parseInt(e.target.value, 10) || 0,
                            },
                          })
                        }
                        className={cn(inputClass, "w-12 tabular-nums")}
                      />
                      <span>days</span>
                    </div>
                  )}
                </div>
              )}

              {key === "review" && (
                <div className="space-y-2.5 pt-3 border-t border-white/[0.05]">
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <label htmlFor="review-team" className="text-xs text-white/80">
                        Team PRs
                      </label>
                      <p className="text-[10px] text-tertiary">
                        PRs that ask for your review, and every open PR in the repos below.
                      </p>
                    </div>
                    <Toggle
                      id="review-team"
                      on={cfg.reviewer.team.enabled}
                      onClick={() =>
                        saveReviewer({
                          team: { ...cfg.reviewer.team, enabled: !cfg.reviewer.team.enabled },
                        })
                      }
                    />
                  </div>
                  {cfg.reviewer.team.enabled && (
                    <>
                      <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                        <span>Opened by</span>
                        <input
                          type="text"
                          value={cfg.reviewer.team.team}
                          onChange={(e) =>
                            setReviewer({ team: { ...cfg.reviewer.team, team: e.target.value } })
                          }
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
                          value={cfg.reviewer.team.maxAgeDays}
                          onChange={(e) =>
                            saveReviewer({
                              team: {
                                ...cfg.reviewer.team,
                                maxAgeDays: Number.parseInt(e.target.value, 10) || 0,
                              },
                            })
                          }
                          className={cn(inputClass, "w-12 tabular-nums")}
                        />
                        <span>days</span>
                      </div>
                      <RepoList
                        repos={cfg.reviewer.team.repos}
                        org={cfg.reviewer.team.team.split("/")[0].trim()}
                        onChange={(repos) =>
                          saveReviewer({ team: { ...cfg.reviewer.team, repos } })
                        }
                      />
                    </>
                  )}
                </div>
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
            Checks Slack, GitHub, Jira, Google Calendar, the reviewer and Mac wake.
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
