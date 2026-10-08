import { describe, expect, it } from "vitest";
import { merge } from "../components/WorkerSettings";

// A worker.json from before 0.5.4, in the shape of a real one.
const legacy = JSON.stringify({
  calendarId: "primary",
  enabled: true,
  timeZone: "Europe/Stockholm",
  focus: { minMinutes: 30, windowEnd: "17:00", windowStart: "09:00" },
  jobs: {
    focus: { enabled: true, times: ["09:15"] },
    standup: { enabled: true, times: ["07:00"] },
    tono: { enabled: false, times: ["10:00", "15:00"] },
  },
  tonoMaxAgeDays: 1,
  tonoMine: true,
  tonoPath: "",
  tonoRepos: ["epidemicsound/kalimba"],
  tonoTeam: "epidemicsound/content-protection",
  tonoTeamPRs: true,
  slack: { botToken: "x", userId: "U1" },
});

describe("merge", () => {
  it("carries a pre-0.5.4 config over to the reviewer section", () => {
    const cfg = merge(legacy);
    expect(cfg.reviewer.source).toBe("managed");
    expect(cfg.reviewer.repo).toBe("epidemicsound/tonometer");
    expect(cfg.reviewer.branch).toBe("main");
    expect(cfg.reviewer.command).toBe(
      "TONO_CLAUDE={claude} {reviewer} {pr} --all -l high -R {repo}",
    );
    expect(cfg.reviewer.team.enabled).toBe(true);
    // The real values survive, not the defaults.
    expect(cfg.reviewer.team.maxAgeDays).toBe(1);
    expect(cfg.reviewer.team.repos).toEqual(["epidemicsound/kalimba"]);
    expect(cfg.reviewer.mine).toEqual({ enabled: true, maxAgeDays: 14 });
    // jobs.tono becomes jobs.review, with its own times and switch.
    expect(cfg.jobs.review).toEqual({ enabled: false, times: ["10:00", "15:00"] });
    expect(cfg.jobs.tono).toBeUndefined();
  });

  it("drops the legacy keys, so a save writes only the new shape", () => {
    const saved = JSON.parse(JSON.stringify(merge(legacy)));
    for (const key of [
      "tonoPath",
      "tonoMine",
      "tonoTeam",
      "tonoTeamPRs",
      "tonoMaxAgeDays",
      "tonoRepos",
    ]) {
      expect(saved).not.toHaveProperty(key);
    }
  });

  it("turns a set tono path into a local reviewer", () => {
    const cfg = merge(JSON.stringify({ tonoPath: "/opt/tono/tono", tonoOwnPRsOnly: true }));
    expect(cfg.reviewer.source).toBe("local");
    expect(cfg.reviewer.path).toBe("/opt/tono/tono");
    expect(cfg.reviewer.team.enabled).toBe(false);
  });

  it("keeps a reviewer section as it is", () => {
    const cfg = merge(
      JSON.stringify({
        tonoMaxAgeDays: 9,
        reviewer: { source: "managed", repo: "o/r", branch: "dev", team: { maxAgeDays: 3 } },
        jobs: {
          review: { enabled: true, times: ["11:00"] },
          tono: { enabled: true, times: ["09:30"] },
        },
      }),
    );
    expect(cfg.reviewer.branch).toBe("dev");
    expect(cfg.reviewer.team.maxAgeDays).toBe(3);
    expect(cfg.reviewer.team.team).toBe("");
    expect(cfg.jobs.review.times).toEqual(["11:00"]);
  });

  // A real 0.5.7 reviewer section: no command, format, args or marker. The
  // worker's TestReviewerMigration checks the same shape.
  const edward = {
    reviewer: {
      autoUpdate: true,
      branch: "main",
      mine: { enabled: true, maxAgeDays: 14 },
      path: "",
      repo: "epidemicsound/tonometer",
      script: "tono",
      source: "managed",
      team: { enabled: true, maxAgeDays: 1, repos: [], team: "epidemicsound/content-protection" },
    },
  };

  it("gives a 0.5.7 config tono's command, logs and marker", () => {
    const cfg = merge(JSON.stringify(edward));
    expect(cfg.reviewer.command).toBe(
      "TONO_CLAUDE={claude} {reviewer} {pr} --all -l high -R {repo}",
    );
    expect(cfg.reviewer.logs).toBe("~/.cache/tono/logs");
    expect(cfg.reviewer.markerPrefix).toBe("tono");
    expect(cfg.reviewer.repo).toBe("epidemicsound/tonometer");
    expect(cfg.reviewer.team.team).toBe("epidemicsound/content-protection");
    // A save writes the new shape, and loading it again changes nothing.
    const saved = JSON.parse(JSON.stringify(cfg));
    expect(saved.reviewer).not.toHaveProperty("format");
    expect(saved.reviewer).not.toHaveProperty("args");
    expect(merge(JSON.stringify(saved))).toEqual(cfg);
  });

  it("keeps a 0.5.7 result-json reviewer off the logs", () => {
    const cfg = merge(
      JSON.stringify({
        reviewer: { format: "result-json", args: ["{url}", "a b"], markerPrefix: "acme" },
      }),
    );
    expect(cfg.reviewer.command).toBe("TONO_CLAUDE={claude} {reviewer} {url} 'a b'");
    expect(cfg.reviewer.logs).toBe("");
    expect(cfg.reviewer.markerPrefix).toBe("acme");
  });

  it("leaves an empty command empty", () => {
    const cfg = merge(JSON.stringify({ reviewer: { command: "", logs: "" } }));
    expect(cfg.reviewer.command).toBe("");
    expect(cfg.reviewer.markerPrefix).toBe("");
  });

  it("names no reviewer and no team on a new install", () => {
    const cfg = merge("{}");
    expect(cfg.reviewer.command).toBe("");
    expect(cfg.reviewer.repo).toBe("");
    expect(cfg.reviewer.team).toEqual({ enabled: false, maxAgeDays: 4, team: "", repos: [] });
    expect(cfg.jobs.review.enabled).toBe(false);
  });
});
