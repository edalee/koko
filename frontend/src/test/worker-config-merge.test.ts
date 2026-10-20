import { describe, expect, it } from "vitest";
import { canonicalZone, merge, toggleDay } from "../components/worker/config";

describe("merge", () => {
  it("turns the old review job and scopes into one daily run", () => {
    const cfg = merge(
      JSON.stringify({
        reviewer: { source: "managed", repo: "o/r", branch: "dev", team: { maxAgeDays: 3 } },
        jobs: { review: { enabled: true, times: ["11:00"] } },
      }),
    );
    expect(cfg.reviewer.branch).toBe("dev");
    expect(cfg.reviewer.runs).toEqual([
      {
        name: "daily",
        args: "",
        enabled: true,
        times: ["11:00"],
        days: ["mon", "tue", "wed", "thu", "fri"],
        target: "prs",
        mine: { enabled: true, maxAgeDays: 14 },
        team: { enabled: false, maxAgeDays: 3, team: "", repos: [] },
        repos: [],
        timeoutMinutes: 0,
      },
    ]);
    // The old shape is not saved again.
    expect(cfg.jobs.review).toBeUndefined();
    expect("team" in cfg.reviewer).toBe(false);
    expect(cfg.jobs.standup.times).toEqual(["07:00"]);
  });

  it("keeps runs as they are, and fills in what a run leaves out", () => {
    const cfg = merge(
      JSON.stringify({
        reviewer: {
          mine: { enabled: false },
          runs: [
            {
              name: "weekly",
              args: "--update-deps",
              times: ["08:00"],
              target: "repos",
              repos: ["acme/api"],
              team: { repos: null },
            },
          ],
        },
        jobs: { review: { enabled: true, times: ["11:00"] } },
      }),
    );
    expect(cfg.reviewer.runs).toHaveLength(1);
    const [run] = cfg.reviewer.runs;
    expect(run.name).toBe("weekly");
    expect(run.args).toBe("--update-deps");
    expect(run.target).toBe("repos");
    expect(run.repos).toEqual(["acme/api"]);
    expect(run.enabled).toBe(false);
    expect(run.times).toEqual(["08:00"]);
    expect(run.days).toEqual(["mon", "tue", "wed", "thu", "fri"]);
    expect(run.mine).toEqual({ enabled: true, maxAgeDays: 14 });
    expect(run.team.repos).toEqual([]);
  });

  it("leaves an empty command empty", () => {
    const cfg = merge(JSON.stringify({ reviewer: { command: "", logs: "" } }));
    expect(cfg.reviewer.command).toBe("");
    expect(cfg.reviewer.markerPrefix).toBe("");
  });

  it("keeps the time zone, calendar and Jira site", () => {
    const cfg = merge(
      JSON.stringify({
        timeZone: "Europe/London",
        calendarId: "work",
        jiraSite: "acme.atlassian.net",
      }),
    );
    expect(cfg.timeZone).toBe("Europe/London");
    expect(cfg.calendarId).toBe("work");
    expect(cfg.jiraSite).toBe("acme.atlassian.net");
  });

  it("names no reviewer, team or Jira site on a new install", () => {
    const cfg = merge("{}");
    expect(cfg.reviewer.command).toBe("");
    expect(cfg.reviewer.repo).toBe("");
    expect(cfg.reviewer.runs).toHaveLength(1);
    expect(cfg.reviewer.runs[0].team).toEqual({
      enabled: false,
      maxAgeDays: 4,
      team: "",
      repos: [],
    });
    expect(cfg.reviewer.runs[0].enabled).toBe(false);
    expect(cfg.jiraSite).toBe("");
    // The time zone is this computer's own.
    expect(cfg.timeZone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone);
  });

  it("gives a job without days Monday to Friday, and keeps set days", () => {
    const cfg = merge(
      JSON.stringify({
        jobs: { standup: { enabled: true, times: ["07:00"] }, focus: { days: ["wed"] } },
      }),
    );
    expect(cfg.jobs.standup.days).toEqual(["mon", "tue", "wed", "thu", "fri"]);
    expect(cfg.jobs.focus.days).toEqual(["wed"]);
  });

  it("turns lunch on when it is missing, and keeps it off when it is off", () => {
    expect(merge("{}").lunch).toEqual({ enabled: true, start: "12:00", end: "13:00" });
    expect(merge(JSON.stringify({ lunch: { enabled: false } })).lunch).toEqual({
      enabled: false,
      start: "12:00",
      end: "13:00",
    });
  });
});

describe("toggleDay", () => {
  it("adds a day in week order and keeps the last day on", () => {
    expect(toggleDay(["mon", "fri"], "wed")).toEqual(["mon", "wed", "fri"]);
    expect(toggleDay(["mon", "wed"], "mon")).toEqual(["wed"]);
    expect(toggleDay(["sat"], "sat")).toEqual(["sat"]);
  });
});

describe("review orgs and excluded repos", () => {
  it("keeps the lists, and turns null into an empty list", () => {
    const kept = merge(
      JSON.stringify({ reviewer: { orgs: ["acme"], excludeRepos: ["acme/legacy"] } }),
    );
    expect(kept.reviewer.orgs).toEqual(["acme"]);
    expect(kept.reviewer.excludeRepos).toEqual(["acme/legacy"]);
    const empty = merge(JSON.stringify({ reviewer: { orgs: null, excludeRepos: null } }));
    expect(empty.reviewer.orgs).toEqual([]);
    expect(empty.reviewer.excludeRepos).toEqual([]);
  });
});

describe("canonicalZone", () => {
  it("accepts a real zone in any case, and nothing else", () => {
    expect(canonicalZone("Europe/London")).toBe("Europe/London");
    expect(canonicalZone("europe/london")).toBe("Europe/London");
    expect(canonicalZone("Europe/Lon")).toBeNull();
    expect(canonicalZone("Local")).toBeNull();
  });
});

describe("merge for the summary", () => {
  it("starts off at 08:30 on weekdays, with no team and Jira and prose on", () => {
    const cfg = merge("{}");
    expect(cfg.summary).toEqual({ team: "", repos: [], jira: true, prose: true });
    expect(cfg.jobs.summary).toEqual({
      enabled: false,
      times: ["08:30"],
      days: ["mon", "tue", "wed", "thu", "fri"],
    });
  });

  it("keeps saved settings and turns null repos into an empty list", () => {
    const cfg = merge(
      JSON.stringify({
        summary: { team: "acme/platform", repos: null, prose: false },
        jobs: { summary: { enabled: true, times: ["09:00"], days: ["mon"] } },
      }),
    );
    expect(cfg.summary).toEqual({ team: "acme/platform", repos: [], jira: true, prose: false });
    expect(cfg.jobs.summary.days).toEqual(["mon"]);
    expect(cfg.jobs.standup.times).toEqual(["07:00"]);
  });
});
