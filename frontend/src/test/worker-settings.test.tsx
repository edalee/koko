import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  GetConfig,
  RunNow,
  SaveConfig,
  SetEnabled,
  Status,
} from "../../wailsjs/go/main/WorkerService";
import WorkerSettings from "../components/WorkerSettings";
import { DEFAULTS, daysLabel, merge, nextSlot, reviewerSetUp } from "../components/worker/config";

vi.mock("../../wailsjs/go/main/WorkerService", () => ({
  Check: vi.fn().mockResolvedValue("[]"),
  GetConfig: vi.fn(),
  ReadLog: vi.fn().mockResolvedValue(""),
  RunNow: vi.fn(),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  SetEnabled: vi.fn().mockResolvedValue(undefined),
  Status: vi.fn(),
}));

const mockGetConfig = GetConfig as ReturnType<typeof vi.fn>;
const mockRunNow = RunNow as ReturnType<typeof vi.fn>;
const mockSave = SaveConfig as ReturnType<typeof vi.fn>;
const mockStatus = Status as ReturnType<typeof vi.fn>;
const mockSetEnabled = SetEnabled as ReturnType<typeof vi.fn>;

// A full config, as merge gives it, so a save writes every key.
const full = () =>
  merge(
    JSON.stringify({
      enabled: true,
      timeZone: "Europe/London",
      jiraSite: "acme.atlassian.net",
      slack: { botToken: "xoxb-test", userId: "U123" },
      reviewer: {
        repo: "acme/reviewer",
        script: "review",
        command: "{reviewer} {pr}",
        markerPrefix: "acme",
        orgs: ["acme"],
        team: { enabled: true, maxAgeDays: 4, team: "acme/platform", repos: ["acme/api"] },
      },
    }),
  );

const lastSaved = () => JSON.parse(mockSave.mock.calls[mockSave.mock.calls.length - 1][0]);

async function renderLoaded(cfg = full()) {
  mockGetConfig.mockResolvedValue(JSON.stringify(cfg));
  render(<WorkerSettings />);
  await waitFor(() => expect(screen.getByLabelText("Member ID")).toHaveValue("U123"));
}

// openCard opens a job card and gives back its region.
function openCard(name: string) {
  const card = screen.getByRole("region", { name });
  fireEvent.click(within(card).getAllByRole("button", { expanded: false })[0]);
  return card;
}

describe("WorkerSettings layout (plan 035 part 2)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStatus.mockResolvedValue(JSON.stringify({ enabled: true, agentLoaded: true, jobs: {} }));
  });

  it("shows each job's output in its own card", async () => {
    mockRunNow.mockImplementation(async (job: string) => `${job} output`);
    await renderLoaded();
    const standup = openCard("Stand-up");
    const focus = openCard("Focus time");

    fireEvent.click(within(standup).getByRole("button", { name: "Test" }));
    expect(await within(standup).findByText("standup output")).toBeInTheDocument();
    fireEvent.click(within(focus).getByRole("button", { name: "Run now" }));
    expect(await within(focus).findByText("focus output")).toBeInTheDocument();

    // Running focus time leaves the stand-up's output where it is.
    expect(within(standup).getByText("standup output")).toBeInTheDocument();
    expect(within(standup).queryByText("focus output")).toBeNull();
    expect(mockRunNow).toHaveBeenCalledWith("standup", true);
    expect(mockRunNow).toHaveBeenCalledWith("focus", false);
  });

  it("shows on or off and the days on a closed card", async () => {
    await renderLoaded();
    const review = screen.getByRole("region", { name: "Review worker" });
    expect(within(review).getByText(/^Off · Not run yet/)).toBeInTheDocument();
    const standup = screen.getByRole("region", { name: "Stand-up" });
    expect(within(standup).getByText(/Mon to Fri at 07:00 · next/)).toBeInTheDocument();
  });

  it("splits the review card into Reviewer and Which PRs", async () => {
    await renderLoaded();
    const review = openCard("Review worker");

    // A reviewer is set up, so its group starts closed with a summary.
    expect(
      within(review).getByText("Managed clone of acme/reviewer, branch main"),
    ).toBeInTheDocument();
    expect(within(review).queryByRole("textbox", { name: "Command" })).toBeNull();
    fireEvent.click(within(review).getByRole("button", { name: /Reviewer/, expanded: false }));
    expect(within(review).getByRole("textbox", { name: "Command" })).toHaveValue("{reviewer} {pr}");
    expect(within(review).getByRole("textbox", { name: "Marker" })).toHaveValue("acme");

    expect(within(review).getByText("Which PRs")).toBeInTheDocument();
    for (const label of ["Orgs", "Excluded repos", "My PRs", "Team PRs"]) {
      expect(within(review).getByText(label)).toBeInTheDocument();
    }
  });

  it("opens the Reviewer group when no reviewer is set up", async () => {
    mockGetConfig.mockResolvedValue("{}");
    render(<WorkerSettings />);
    const review = openCard("Review worker");
    expect(within(review).getByRole("textbox", { name: "Command" })).toBeInTheDocument();
  });

  // The open group is decided once, at load. A click that sets the reviewer up
  // gives no focus event, so only that load step keeps the group open.
  it("keeps the Reviewer group open after a click sets the reviewer up", async () => {
    const cfg = full();
    cfg.reviewer.repo = "";
    cfg.reviewer.path = "~/bin/review";
    await renderLoaded(cfg);
    const review = openCard("Review worker");
    expect(within(review).getByRole("textbox", { name: "Command" })).toBeInTheDocument();

    fireEvent.click(within(review).getByRole("button", { name: "Local path" }));
    expect(lastSaved().reviewer.source).toBe("local");
    expect(within(review).getByRole("textbox", { name: "Command" })).toBeInTheDocument();
  });

  it("shows the team box with Team PRs off, for the stand-up", async () => {
    const cfg = full();
    cfg.reviewer.team.enabled = false;
    await renderLoaded(cfg);
    const review = openCard("Review worker");
    expect(within(review).getByRole("textbox", { name: "Team" })).toHaveValue("acme/platform");
  });

  it("lists a closed card's times in order, without empty ones", async () => {
    const cfg = full();
    cfg.jobs.review = { enabled: true, times: ["14:00", "", "09:30"], days: ["mon"] };
    await renderLoaded(cfg);
    const review = screen.getByRole("region", { name: "Review worker" });
    expect(within(review).getByText(/^Mon at 09:30, 14:00 · next/)).toBeInTheDocument();
  });

  it("shows no next run while the agent is stopped", async () => {
    mockStatus.mockResolvedValue(JSON.stringify({ enabled: true, agentLoaded: false, jobs: {} }));
    await renderLoaded();
    await waitFor(() => expect(screen.getByText("stopped")).toBeInTheDocument());
    const standup = screen.getByRole("region", { name: "Stand-up" });
    expect(within(standup).getByText(/^Mon to Fri at 07:00 · Not run yet/)).toBeInTheDocument();
  });

  it("names the review worker's team on the stand-up card", async () => {
    await renderLoaded();
    const standup = openCard("Stand-up");
    expect(within(standup).getByText("acme/platform")).toBeInTheDocument();
    expect(within(standup).getByText(/uses the review worker's team/)).toBeInTheDocument();
    expect(within(standup).getByText(/open PRs in 1 repo\./)).toBeInTheDocument();
  });
});

describe("WorkerSettings saves", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStatus.mockResolvedValue(JSON.stringify({ enabled: true, agentLoaded: true, jobs: {} }));
  });

  it("saves a text field on blur, with the whole config", async () => {
    await renderLoaded();
    const id = screen.getByLabelText("Member ID");
    fireEvent.change(id, { target: { value: "U999" } });
    expect(mockSave).not.toHaveBeenCalled();
    fireEvent.blur(id);

    await waitFor(() => expect(mockSave).toHaveBeenCalledTimes(1));
    const want = full();
    want.slack.userId = "U999";
    expect(lastSaved()).toEqual(want);
  });

  it("saves a switch, a number and a day at once", async () => {
    await renderLoaded();
    const review = openCard("Review worker");

    fireEvent.click(within(review).getByRole("button", { name: "My PRs" }));
    const want = full();
    want.reviewer.mine.enabled = false;
    expect(lastSaved()).toEqual(want);

    fireEvent.change(within(review).getByLabelText("Team PRs age in days"), {
      target: { value: "7" },
    });
    want.reviewer.team.maxAgeDays = 7;
    expect(lastSaved()).toEqual(want);

    fireEvent.click(within(review).getByRole("button", { name: "Sat" }));
    want.jobs.review.days = ["mon", "tue", "wed", "thu", "fri", "sat"];
    expect(lastSaved()).toEqual(want);

    fireEvent.click(within(review).getByRole("button", { name: "Review worker on" }));
    want.jobs.review.enabled = true;
    expect(lastSaved()).toEqual(want);
  });

  it("puts back the saved zone after a typo, and saves nothing", async () => {
    await renderLoaded();
    const zone = screen.getByLabelText("Time zone");
    fireEvent.change(zone, { target: { value: "Europe/Lon" } });
    fireEvent.blur(zone);

    expect(await screen.findByText(/"Europe\/Lon" is not a time zone/)).toBeInTheDocument();
    expect(zone).toHaveValue("Europe/London");
    expect(mockSave).not.toHaveBeenCalled();
  });

  it("does not save a lunch that ends before it starts", async () => {
    await renderLoaded();
    const end = screen.getByLabelText("Lunch end");
    fireEvent.change(end, { target: { value: "11:00" } });
    fireEvent.focusOut(end, { relatedTarget: null });

    expect(screen.getByText(/Lunch must end after it starts/)).toBeInTheDocument();
    expect(mockSave).not.toHaveBeenCalled();
  });
});

describe("the master switch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStatus.mockResolvedValue(JSON.stringify({ enabled: true, agentLoaded: true, jobs: {} }));
  });

  it("blocks job edits while it runs, then saves with the new state", async () => {
    let finish = () => {};
    mockSetEnabled.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    await renderLoaded();
    const review = openCard("Review worker");

    fireEvent.click(screen.getByRole("button", { name: "Worker on" }));
    expect(within(review).getByRole("button", { name: "Sat" })).toBeDisabled();
    expect(screen.getByLabelText("Member ID")).toBeDisabled();

    finish();
    await waitFor(() => expect(within(review).getByRole("button", { name: "Sat" })).toBeEnabled());
    expect(mockSetEnabled).toHaveBeenCalledWith(false);
    fireEvent.click(within(review).getByRole("button", { name: "Sat" }));
    const want = full();
    want.enabled = false;
    want.jobs.review.days = ["mon", "tue", "wed", "thu", "fri", "sat"];
    expect(lastSaved()).toEqual(want);
  });
});

describe("reviewerSetUp", () => {
  const ready = {
    ...DEFAULTS.reviewer,
    command: "{reviewer} {pr}",
    markerPrefix: "acme",
    repo: "acme/reviewer",
    script: "review",
  };

  it("follows the worker's reviewer rules", () => {
    expect(reviewerSetUp(ready)).toBe(true);
    expect(reviewerSetUp({ ...ready, command: "" })).toBe(false);
    expect(reviewerSetUp({ ...ready, markerPrefix: "" })).toBe(false);
    expect(reviewerSetUp({ ...ready, markerPrefix: "Acme!" })).toBe(false);
    expect(reviewerSetUp({ ...ready, repo: "reviewer" })).toBe(false);
    expect(reviewerSetUp({ ...ready, script: "" })).toBe(false);
    expect(reviewerSetUp({ ...ready, script: "../bin/sh" })).toBe(false);
    expect(reviewerSetUp({ ...ready, source: "local", path: "" })).toBe(false);
    expect(reviewerSetUp({ ...ready, source: "local", path: "~/bin/review" })).toBe(true);
    // A command without {reviewer} needs no source.
    expect(reviewerSetUp({ ...ready, command: "acme-review {pr}", repo: "", script: "" })).toBe(
      true,
    );
  });
});

describe("daysLabel and nextSlot", () => {
  it("names common day sets in words", () => {
    expect(daysLabel(["mon", "tue", "wed", "thu", "fri"])).toBe("Mon to Fri");
    expect(daysLabel(["sun", "sat"])).toBe("Sat and Sun");
    expect(daysLabel(["wed", "mon"])).toBe("Mon, Wed");
  });

  it("finds the next time on the job's days, in the worker's zone", () => {
    const job = { enabled: true, times: ["14:00", "09:30"], days: ["mon", "wed"] };
    // London is on UTC in winter, so this is Monday 10:00 there.
    const monday = new Date("2026-01-05T10:00:00Z");
    expect(nextSlot(job, "Europe/London", monday)).toBe("today 14:00");
    // Monday 15:00 in London: the next run is Wednesday morning.
    expect(nextSlot(job, "Europe/London", new Date("2026-01-05T15:00:00Z"))).toBe("Wed 09:30");
    // Sunday 23:30 in London: Monday is tomorrow.
    expect(nextSlot(job, "Europe/London", new Date("2026-01-04T23:30:00Z"))).toBe("tomorrow 09:30");
    expect(nextSlot({ ...job, enabled: false }, "Europe/London", monday)).toBeNull();
  });
});
