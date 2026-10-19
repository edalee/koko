import { describe, expect, it } from "vitest";
import { canonicalZone, merge } from "../components/WorkerSettings";

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
