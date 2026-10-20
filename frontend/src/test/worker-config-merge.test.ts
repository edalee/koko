import { describe, expect, it } from "vitest";
import { canonicalZone, merge, toggleDay } from "../components/worker/config";

describe("merge", () => {
  it("keeps a reviewer section as it is", () => {
    const cfg = merge(
      JSON.stringify({
        reviewer: { source: "managed", repo: "o/r", branch: "dev", team: { maxAgeDays: 3 } },
        jobs: { review: { enabled: true, times: ["11:00"] } },
      }),
    );
    expect(cfg.reviewer.branch).toBe("dev");
    expect(cfg.reviewer.team.maxAgeDays).toBe(3);
    expect(cfg.reviewer.team.team).toBe("");
    expect(cfg.jobs.review.times).toEqual(["11:00"]);
    expect(cfg.jobs.standup.times).toEqual(["07:00"]);
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
    expect(cfg.reviewer.team).toEqual({ enabled: false, maxAgeDays: 4, team: "", repos: [] });
    expect(cfg.jobs.review.enabled).toBe(false);
    expect(cfg.jiraSite).toBe("");
    // The time zone is this computer's own.
    expect(cfg.timeZone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone);
  });

  it("gives a job without days Monday to Friday, and keeps set days", () => {
    const cfg = merge(
      JSON.stringify({
        jobs: { standup: { enabled: true, times: ["07:00"] }, review: { days: ["wed"] } },
      }),
    );
    expect(cfg.jobs.standup.days).toEqual(["mon", "tue", "wed", "thu", "fri"]);
    expect(cfg.jobs.focus.days).toEqual(["mon", "tue", "wed", "thu", "fri"]);
    expect(cfg.jobs.review.days).toEqual(["wed"]);
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
