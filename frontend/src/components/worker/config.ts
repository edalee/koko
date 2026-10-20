import { mergeRuns, type ReviewRun } from "./runs";

// Mirrors cmd/koko-worker/config.go. The worker fills in defaults for missing
// or empty values, so an empty text box keeps the default.
export interface JobConfig {
  enabled: boolean;
  times: string[];
  days: string[];
}

export interface ReviewerConfig {
  source: "managed" | "local";
  repo: string;
  branch: string;
  autoUpdate: boolean;
  script: string;
  path: string;
  command: string;
  logs: string;
  markerPrefix: string;
  // runs replace the old review job and its mine and team scopes.
  runs: ReviewRun[];
  orgs: string[];
  excludeRepos: string[];
}

// SummaryConfig is the team summary: whose work it reports, and where.
export interface SummaryConfig {
  team: string;
  repos: string[];
  jira: boolean;
  prose: boolean;
}

export interface WorkerConfig {
  enabled: boolean;
  timeZone: string;
  calendarId: string;
  jiraSite: string;
  reviewer: ReviewerConfig;
  slack: { botToken: string; userId: string };
  focus: { windowStart: string; windowEnd: string; minMinutes: number };
  lunch: { enabled: boolean; start: string; end: string };
  summary: SummaryConfig;
  jobs: Record<string, JobConfig>;
}

export interface RunRecord {
  status: "ok" | "failed" | "retry" | "offline";
  at: string;
  message?: string;
}

export interface WorkerStatus {
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

export interface CheckResult {
  name: string;
  ok: boolean;
  detail: string;
  fix?: string;
}

export const DEFAULTS: WorkerConfig = {
  enabled: false,
  timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
  calendarId: "primary",
  jiraSite: "",
  // No reviewer is set up, so the review run starts off.
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
    runs: mergeRuns(undefined, undefined),
    orgs: [],
    excludeRepos: [],
  },
  slack: { botToken: "", userId: "" },
  focus: { windowStart: "09:00", windowEnd: "17:00", minMinutes: 30 },
  lunch: { enabled: true, start: "12:00", end: "13:00" },
  summary: { team: "", repos: [], jira: true, prose: true },
  jobs: {
    standup: { enabled: true, times: ["07:00"], days: ["mon", "tue", "wed", "thu", "fri"] },
    focus: { enabled: true, times: ["09:15"], days: ["mon", "tue", "wed", "thu", "fri"] },
    // No team is set, so the summary starts off.
    summary: { enabled: false, times: ["08:30"], days: ["mon", "tue", "wed", "thu", "fri"] },
  },
};

// DAYS are the names a job's days use, in week order.
export const DAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];

export const STATUS_LABELS: Record<string, string> = {
  ok: "ok",
  failed: "failed",
  retry: "failed, trying again",
  offline: "waiting for internet",
};

export const CONNECTION_NAMES: Record<string, string> = {
  slack: "Slack",
  github: "GitHub",
  claude: "Claude Code",
  jira: "Jira",
  calendar: "Google Calendar",
  reviewer: "Reviewer",
  summary: "Team summary",
  wake: "Mac wake",
};

// canonicalZone is the IANA name for zone, in its usual case, or null if
// there is no such zone. "Local" is not a zone.
export function canonicalZone(zone: string): string | null {
  try {
    return new Intl.DateTimeFormat("en", { timeZone: zone.trim() }).resolvedOptions().timeZone;
  } catch {
    return null;
  }
}

export function merge(raw: string): WorkerConfig {
  const parsed = JSON.parse(raw || "{}") as Omit<Partial<WorkerConfig>, "reviewer"> & {
    reviewer?: Partial<ReviewerConfig> & Parameters<typeof mergeRuns>[0];
  };
  // The old review job and scopes become the runs, and are not saved again.
  const { mine: _mine, team: _team, ...reviewer } = parsed.reviewer ?? {};
  const { review: _review, ...jobs } = parsed.jobs ?? {};
  return {
    ...DEFAULTS,
    ...parsed,
    reviewer: {
      ...DEFAULTS.reviewer,
      ...reviewer,
      runs: mergeRuns(parsed.reviewer, parsed.jobs),
      // Go writes an empty list as null.
      orgs: reviewer.orgs ?? [],
      excludeRepos: reviewer.excludeRepos ?? [],
    },
    slack: { ...DEFAULTS.slack, ...parsed.slack },
    focus: { ...DEFAULTS.focus, ...parsed.focus },
    lunch: { ...DEFAULTS.lunch, ...parsed.lunch },
    summary: {
      ...DEFAULTS.summary,
      ...parsed.summary,
      // Go writes an empty list as null.
      repos: parsed.summary?.repos ?? [],
    },
    jobs: Object.fromEntries(
      Object.entries({ ...DEFAULTS.jobs, ...jobs }).map(([key, job]) => [
        key,
        // A job without days runs Monday to Friday, as in the worker.
        { ...job, days: job.days?.length ? job.days : DEFAULTS.jobs.standup.days },
      ]),
    ),
  };
}

// toggleDay switches day on or off and keeps the list in week order.
// It never turns off the only day left, because an empty list means Monday to Friday.
export function toggleDay(days: string[], day: string): string[] {
  if (!days.includes(day)) return DAYS.filter((d) => d === day || days.includes(d));
  return days.length > 1 ? days.filter((d) => d !== day) : days;
}

// minutes turns "HH:MM" into minutes after midnight.
export function minutes(clock: string): number {
  const [h, m] = clock.split(":").map(Number);
  return h * 60 + m;
}

// lunchProblem is what is wrong with a lunch from start to end, or null.
export function lunchProblem(start: string, end: string): string | null {
  if (!start || !end) return "Set a start and an end for lunch.";
  if (minutes(end) <= minutes(start)) return "Lunch must end after it starts.";
  return null;
}

// dayName is a day as a button or summary shows it, such as "Mon".
export const dayName = (day: string) => day[0].toUpperCase() + day.slice(1);

// jobTimes is a job's times in order, without empty ones. The worker drops
// an empty time when it reads worker.json.
export const jobTimes = (job: JobConfig) => job.times.filter(Boolean).sort();

// daysLabel names a job's days in a few words, such as "Mon to Fri".
export function daysLabel(days: string[]): string {
  const set = DAYS.filter((d) => days.includes(d));
  if (set.length === 7) return "Every day";
  if (set.join() === "mon,tue,wed,thu,fri") return "Mon to Fri";
  if (set.join() === "sat,sun") return "Sat and Sun";
  return set.map(dayName).join(", ");
}

// nextSlot is a job's next run in zone, such as "tomorrow 07:00", or null if the
// job is off or has no times. The worker runs jobs in its own time zone, so
// nextSlot reads the current time in that zone.
export function nextSlot(job: JobConfig, zone: string, now = new Date()): string | null {
  const times = jobTimes(job);
  if (!job.enabled || times.length === 0) return null;
  let parts: Record<string, string>;
  try {
    parts = Object.fromEntries(
      new Intl.DateTimeFormat("en-GB", {
        timeZone: canonicalZone(zone) ?? undefined,
        weekday: "short",
        hour: "2-digit",
        minute: "2-digit",
        hourCycle: "h23",
      })
        .formatToParts(now)
        .map((p) => [p.type, p.value]),
    );
  } catch {
    return null;
  }
  const today = DAYS.indexOf(parts.weekday.toLowerCase());
  const nowMinutes = Number(parts.hour) * 60 + Number(parts.minute);
  for (let i = 0; i <= 7; i++) {
    const day = DAYS[(today + i) % 7];
    if (!job.days.includes(day)) continue;
    const time = times.find((t) => i > 0 || minutes(t) > nowMinutes);
    if (!time) continue;
    return `${i === 0 ? "today" : i === 1 ? "tomorrow" : dayName(day)} ${time}`;
  }
  return null;
}

// reviewerSetUp is true if a PRs run can run with these settings. It follows
// problem() in cmd/koko-worker/config.go.
export function reviewerSetUp(r: ReviewerConfig): boolean {
  const command = r.command.trim();
  if (!command || !/^[a-z0-9-]+$/.test(r.markerPrefix.trim())) return false;
  // A command without {reviewer} names its own program, so the source is not used.
  if (!command.includes("{reviewer}")) return true;
  if (r.source === "local") return r.path.trim() !== "";
  const script = r.script.trim();
  return (
    /^[^/\s]+\/[^/\s]+$/.test(r.repo.trim()) &&
    script !== "" &&
    !script.startsWith("/") &&
    !script.includes("..")
  );
}

export function formatWhen(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export const inputClass =
  "px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40";
