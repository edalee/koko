import { describe, expect, it } from "vitest";
import { fromWorker } from "../components/PRPanelSettings";
import { isHiddenPR } from "../lib/prs";

describe("isHiddenPR", () => {
  it("matches the full key and the short key from before 0.5.9", () => {
    const pr = { repo: "acme/api", number: 7 };
    expect(isHiddenPR(new Set(["acme/api#7"]), pr)).toBe(true);
    expect(isHiddenPR(new Set(["api#7"]), pr)).toBe(true);
    expect(isHiddenPR(new Set(["acme/api#8", "web#7"]), pr)).toBe(false);
  });
});

describe("fromWorker", () => {
  it("copies the review worker's scopes", () => {
    const raw = JSON.stringify({
      reviewer: {
        command: "acme {pr}",
        mine: { enabled: true, maxAgeDays: 14 },
        team: { enabled: true, maxAgeDays: 1, team: "acme/platform", repos: ["acme/api"] },
      },
    });
    expect(fromWorker(raw)).toEqual({
      mine: { enabled: true, maxAgeDays: 14 },
      team: { enabled: true, maxAgeDays: 1, team: "acme/platform", repos: ["acme/api"] },
    });
  });

  it("is null when the worker has no scopes", () => {
    expect(fromWorker("{}")).toBeNull();
    expect(fromWorker("")).toBeNull();
  });
});
