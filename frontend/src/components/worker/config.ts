// Mirrors cmd/koko-worker/config.go. The worker fills in defaults for missing
// or empty values, so an empty text box keeps the default.
export interface JobConfig {
  enabled: boolean;
  times: string[];
  days: string[];
}

export interface ScopeConfig {
  enabled: boolean;
  maxAgeDays: number;
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
  mine: ScopeConfig;
  team: ScopeConfig & { team: string; repos: string[] };
  orgs: string[];
  excludeRepos: string[];
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
    orgs: [],
    excludeRepos: [],
  },
  slack: { botToken: "", userId: "" },
  focus: { windowStart: "09:00", windowEnd: "17:00", minMinutes: 30 },
  lunch: { enabled: true, start: "12:00", end: "13:00" },
  jobs: {
    standup: { enabled: true, times: ["07:00"], days: ["mon", "tue", "wed", "thu", "fri"] },
    focus: { enabled: true, times: ["09:15"], days: ["mon", "tue", "wed", "thu", "fri"] },
    review: {
      enabled: false,
      times: ["09:30", "12:00", "14:00"],
      days: ["mon", "tue", "wed", "thu", "fri"],
    },
  },
};

// DAYS are the names a job's days use, in week order.
export const DAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];

export const JOBS: { key: string; name: string; about: string; multi: boolean }[] = [
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
    about: "Books focus blocks in free gaps of 30 minutes or more, around lunch.",
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
  const parsed = JSON.parse(raw || "{}") as Partial<WorkerConfig>;
  const reviewer: Partial<ReviewerConfig> = parsed.reviewer ?? {};
  return {
    ...DEFAULTS,
    ...parsed,
    reviewer: {
      ...DEFAULTS.reviewer,
      ...reviewer,
      mine: { ...DEFAULTS.reviewer.mine, ...reviewer.mine },
      team: { ...DEFAULTS.reviewer.team, ...reviewer.team, repos: reviewer.team?.repos ?? [] },
      // Go writes an empty list as null.
      orgs: reviewer.orgs ?? [],
      excludeRepos: reviewer.excludeRepos ?? [],
    },
    slack: { ...DEFAULTS.slack, ...parsed.slack },
    focus: { ...DEFAULTS.focus, ...parsed.focus },
    lunch: { ...DEFAULTS.lunch, ...parsed.lunch },
    jobs: Object.fromEntries(
      Object.entries({ ...DEFAULTS.jobs, ...parsed.jobs }).map(([key, job]) => [
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
