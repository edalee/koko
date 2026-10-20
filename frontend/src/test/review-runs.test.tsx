import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GetConfig, RunNow, SaveConfig, Status } from "../../wailsjs/go/main/WorkerService";
import WorkerSettings from "../components/WorkerSettings";
import ReviewRuns from "../components/worker/ReviewRuns";
import {
  mergeRuns,
  newRun,
  type ReviewRun,
  runsProblem,
  standupRun,
} from "../components/worker/runs";

vi.mock("../../wailsjs/go/main/WorkerService", () => ({
  Check: vi.fn().mockResolvedValue("[]"),
  GetConfig: vi.fn(),
  ReadLog: vi.fn().mockResolvedValue(""),
  RunNow: vi.fn(),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  SetEnabled: vi.fn().mockResolvedValue(undefined),
  Status: vi.fn(),
}));

const mockRunNow = RunNow as ReturnType<typeof vi.fn>;
const mockSave = SaveConfig as ReturnType<typeof vi.fn>;

const daily = mergeRuns({ runs: [{ name: "daily", enabled: true }] }, undefined)[0];

function renderRuns(runs: ReviewRun[], command = "acme {pr}") {
  const onSave = vi.fn();
  render(
    <ReviewRuns
      runs={runs}
      command={command}
      running={null}
      setRunning={vi.fn()}
      onRan={vi.fn()}
      onEdit={vi.fn()}
      onSave={onSave}
    />,
  );
  return onSave;
}

beforeEach(() => {
  vi.clearAllMocks();
  (Status as ReturnType<typeof vi.fn>).mockResolvedValue(
    JSON.stringify({ enabled: true, agentLoaded: true, jobs: {} }),
  );
});

describe("ReviewRuns", () => {
  it("adds a weekly PRs run on Mondays, switched off", () => {
    const onSave = renderRuns([daily]);
    fireEvent.click(screen.getByRole("button", { name: /add run/i }));
    const saved: ReviewRun[] = onSave.mock.calls[0][0];
    expect(saved.map((r) => r.name)).toEqual(["daily", "weekly"]);
    expect(saved[1]).toEqual({
      name: "weekly",
      args: "",
      enabled: false,
      times: ["09:00"],
      days: ["mon"],
      target: "prs",
      mine: { enabled: true, maxAgeDays: 14 },
      team: { enabled: false, maxAgeDays: 4, team: "", repos: [] },
      repos: [],
      timeoutMinutes: 0,
    });
  });

  it("removes a run, and keeps the last one", () => {
    const onSave = renderRuns([daily, newRun([daily])]);
    fireEvent.click(screen.getAllByTitle("Remove this run")[1]);
    expect(onSave).toHaveBeenCalledWith([daily]);

    cleanup();
    renderRuns([daily]);
    expect(screen.getByTitle("Keep at least one run")).toBeDisabled();
  });

  it("tests and runs each run as its own job", async () => {
    mockRunNow.mockImplementation(async (job: string) => `${job} output`);
    renderRuns([daily, newRun([daily])]);
    const weekly = screen.getByRole("region", { name: "Run weekly" });
    fireEvent.click(within(weekly).getByRole("button", { name: "Test" }));
    expect(await within(weekly).findByText("review:weekly output")).toBeInTheDocument();
    expect(mockRunNow).toHaveBeenCalledWith("review:weekly", true);
  });

  it("shows no output from a removed run on the run that takes its place", async () => {
    mockRunNow.mockImplementation(async (job: string) => `${job} output`);
    const weekly = newRun([daily]);
    const props = {
      command: "acme {pr}",
      running: null,
      setRunning: vi.fn(),
      onRan: vi.fn(),
      onEdit: vi.fn(),
      onSave: vi.fn(),
    };
    const { rerender } = render(<ReviewRuns runs={[daily, weekly]} {...props} />);
    const first = screen.getByRole("region", { name: "Run daily" });
    fireEvent.click(within(first).getByRole("button", { name: "Test" }));
    expect(await within(first).findByText("review:daily output")).toBeInTheDocument();

    rerender(<ReviewRuns runs={[weekly]} {...props} />);
    expect(screen.getByRole("region", { name: "Run weekly" })).toBeInTheDocument();
    expect(screen.queryByText("review:daily output")).toBeNull();
  });

  it("switches a run to repos, with its repos and time limit", () => {
    const onSave = renderRuns([daily]);
    expect(screen.getByText("My PRs")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Repos" }));
    expect(onSave.mock.calls[0][0][0].target).toBe("repos");

    cleanup();
    const weekly: ReviewRun = { ...newRun([daily]), target: "repos", repos: ["acme/api"] };
    const saveRepos = renderRuns([weekly]);
    expect(screen.queryByText("My PRs")).toBeNull();
    expect(screen.getByText("acme/api")).toBeInTheDocument();
    expect(screen.getByLabelText("Time limit")).toHaveValue(60);
    fireEvent.change(screen.getByLabelText("Time limit"), { target: { value: "90" } });
    expect(saveRepos.mock.calls[0][0][0].timeoutMinutes).toBe(90);
    // The shared command names a PR, which a repos run does not have.
    expect(screen.getByText(/refuses a command with \{pr\}/)).toBeInTheDocument();
  });
});

describe("run helpers", () => {
  it("rejects a bad, long or repeated name, and an empty list", () => {
    expect(runsProblem([daily])).toBeNull();
    expect(runsProblem([{ ...daily, name: "Weekly Run" }])).toMatch(/not a run name/);
    expect(runsProblem([{ ...daily, name: "a".repeat(33) }])).toMatch(/not a run name/);
    expect(runsProblem([{ ...daily, name: "a".repeat(32) }])).toBeNull();
    expect(runsProblem([daily, daily])).toMatch(/Two runs/);
    expect(runsProblem([])).toMatch(/at least one/);
  });

  it("picks a free name for a new run", () => {
    const weekly = newRun([daily]);
    expect(weekly.name).toBe("weekly");
    expect(newRun([daily, weekly]).name).toBe("run-2");
  });

  it("takes the stand-up team from the first PRs run that is on, never a repos run", () => {
    const repos: ReviewRun = { ...newRun([daily]), target: "repos", enabled: true };
    const off: ReviewRun = { ...daily, name: "off", enabled: false };
    expect(standupRun([repos, off, daily])?.name).toBe("daily");
    expect(standupRun([repos, off])?.name).toBe("off");
    expect(standupRun([repos])).toBeUndefined();
  });
});

describe("WorkerSettings with runs", () => {
  it("saves no config while a run name is bad", async () => {
    (GetConfig as ReturnType<typeof vi.fn>).mockResolvedValue(
      JSON.stringify({
        reviewer: { repo: "acme/reviewer", command: "acme {pr}", markerPrefix: "acme" },
        jobs: { review: { enabled: true, times: ["11:00"] } },
      }),
    );
    render(<WorkerSettings />);
    const card = screen.getByRole("region", { name: "Review worker" });
    fireEvent.click(within(card).getAllByRole("button", { expanded: false })[0]);
    await waitFor(() => expect(within(card).getByLabelText("Run name")).toHaveValue("daily"));
    const name = within(card).getByLabelText("Run name");
    fireEvent.change(name, { target: { value: "Daily Run" } });
    fireEvent.blur(name);
    // Another setting saves nothing either while the name is bad.
    fireEvent.click(within(card).getByRole("button", { name: "Repos" }));
    expect(screen.getByText(/not a run name/)).toBeInTheDocument();
    expect(mockSave).not.toHaveBeenCalled();

    // The card shows the bad name, kept but not saved, for you to fix.
    const shown = within(card).getByLabelText("Run name");
    expect(shown).toHaveValue("Daily Run");
    fireEvent.change(shown, { target: { value: "weekly" } });
    fireEvent.blur(shown);
    await waitFor(() => expect(mockSave).toHaveBeenCalledTimes(1));
    const saved = JSON.parse(mockSave.mock.calls[0][0]);
    expect(saved.reviewer.runs[0]).toMatchObject({ name: "weekly", target: "repos" });
    expect(saved.reviewer.team).toBeUndefined();
    expect(saved.jobs.review).toBeUndefined();
  });
});

describe("StandupCard with no PRs run", () => {
  it("says there is no team", async () => {
    (GetConfig as ReturnType<typeof vi.fn>).mockResolvedValue(
      JSON.stringify({
        reviewer: { runs: [{ name: "weekly", target: "repos", repos: ["acme/api"] }] },
      }),
    );
    render(<WorkerSettings />);
    const card = screen.getByRole("region", { name: "Stand-up" });
    fireEvent.click(within(card).getAllByRole("button", { expanded: false })[0]);
    expect(
      await within(card).findByText(/There is no PRs run, so it has no team/),
    ).toBeInTheDocument();
    expect(within(card).getByText("No team set")).toBeInTheDocument();
  });
});
