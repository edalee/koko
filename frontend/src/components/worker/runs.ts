// Mirrors ReviewRun and its scopes in cmd/koko-worker/config.go: one run of
// the reviewer command, on its own days and times.
export interface ScopeConfig {
  enabled: boolean;
  maxAgeDays: number;
}

export interface TeamScopeConfig extends ScopeConfig {
  team: string;
  repos: string[];
}

export interface ReviewRun {
  name: string;
  args: string;
  enabled: boolean;
  times: string[];
  days: string[];
  // target is what the run works on: each PR in its scopes, or each repo in repos.
  target: "prs" | "repos";
  mine: ScopeConfig;
  team: TeamScopeConfig;
  repos: string[];
  // timeoutMinutes caps one PR or repo. 0 means 45 for PRs and 60 for repos.
  timeoutMinutes: number;
}

const WEEKDAYS = ["mon", "tue", "wed", "thu", "fri"];

// LEGACY_TIMES are a run's default times, the same as the old review job's.
const LEGACY_TIMES = ["09:30", "12:00", "14:00"];

const DEFAULT_MINE: ScopeConfig = { enabled: true, maxAgeDays: 14 };
const DEFAULT_TEAM: TeamScopeConfig = { enabled: false, maxAgeDays: 4, team: "", repos: [] };

// A run's name is part of its job and lock file names.
const NAME_RE = /^[a-z0-9-]{1,32}$/;

// PR_ONLY are the placeholders that name a PR, which a repos run does not have.
const PR_ONLY = ["{pr}", "{url}", "{sha}"];

// RawRun is a run as worker.json may hold it, with any field missing.
export type RawRun = Partial<Omit<ReviewRun, "mine" | "team" | "repos">> & {
  mine?: Partial<ScopeConfig>;
  team?: Partial<Omit<TeamScopeConfig, "repos">> & { repos?: string[] | null };
  repos?: string[] | null;
};

// withDefaults fills in what a run in worker.json leaves out, as the worker does.
function withDefaults(r: RawRun): ReviewRun {
  return {
    name: r.name ?? "",
    args: r.args ?? "",
    enabled: r.enabled ?? false,
    times: r.times?.filter(Boolean).length ? r.times : LEGACY_TIMES,
    days: r.days?.length ? r.days : WEEKDAYS,
    target: r.target === "repos" ? "repos" : "prs",
    mine: { ...DEFAULT_MINE, ...r.mine },
    // Go writes an empty list as null.
    team: { ...DEFAULT_TEAM, ...r.team, repos: r.team?.repos ?? [] },
    repos: r.repos ?? [],
    timeoutMinutes: r.timeoutMinutes ?? 0,
  };
}

// mergeRuns is the runs of a worker.json. A file without runs gets one PRs
// run named "daily" from the old review job and scopes, as in the worker.
export function mergeRuns(
  reviewer: { runs?: RawRun[] | null; mine?: RawRun["mine"]; team?: RawRun["team"] } | undefined,
  jobs: Record<string, { enabled?: boolean; times?: string[]; days?: string[] }> | undefined,
): ReviewRun[] {
  if (reviewer?.runs?.length) return reviewer.runs.map(withDefaults);
  const old = jobs?.review;
  return [
    withDefaults({
      name: "daily",
      enabled: old?.enabled ?? false,
      times: old?.times,
      days: old?.days,
      mine: reviewer?.mine,
      team: reviewer?.team,
    }),
  ];
}

// newRun is a new PRs run, switched off, on Mondays: "weekly" if that name
// is free.
export function newRun(runs: ReviewRun[]): ReviewRun {
  const taken = new Set(runs.map((r) => r.name));
  let name = "weekly";
  for (let i = 2; taken.has(name); i++) name = `run-${i}`;
  return { ...withDefaults({ name }), times: ["09:00"], days: ["mon"] };
}

// runsProblem says why the worker would refuse the runs, or null. A bad name
// stops every job, so the UI does not save one.
export function runsProblem(runs: ReviewRun[]): string | null {
  if (runs.length === 0) return "Keep at least one run.";
  const seen = new Set<string>();
  for (const r of runs) {
    if (!NAME_RE.test(r.name))
      return `"${r.name}" is not a run name. Use up to 32 lowercase letters, digits and dashes.`;
    if (seen.has(r.name)) return `Two runs are called "${r.name}". Give each run its own name.`;
    seen.add(r.name);
  }
  return null;
}

// prOnlyIn lists the PR placeholders in a repos run's command, which make
// the worker refuse to run it.
export function prOnlyIn(command: string, run: ReviewRun): string[] {
  if (run.target !== "repos") return [];
  const full = `${command} ${run.args}`;
  return PR_ONLY.filter((p) => full.includes(p));
}

// standupRun is the run the stand-up takes its team from, as in the worker:
// the first PRs run that is on, else the first PRs run. With no PRs run it is
// undefined, and the stand-up lists no team. It never gives a repos run.
export function standupRun(runs: ReviewRun[]): ReviewRun | undefined {
  const prs = runs.filter((r) => r.target === "prs");
  return prs.find((r) => r.enabled) ?? prs[0];
}
