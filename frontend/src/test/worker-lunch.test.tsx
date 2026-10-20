import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SaveConfig } from "../../wailsjs/go/main/WorkerService";
import WorkerSettings from "../components/WorkerSettings";
import { lunchProblem } from "../components/worker/config";

vi.mock("../../wailsjs/go/main/WorkerService", () => ({
  Check: vi.fn().mockResolvedValue("[]"),
  GetConfig: vi.fn().mockResolvedValue("{}"),
  ReadLog: vi.fn().mockResolvedValue(""),
  RunNow: vi.fn().mockResolvedValue(""),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  SetEnabled: vi.fn().mockResolvedValue(undefined),
  Status: vi
    .fn()
    .mockResolvedValue(JSON.stringify({ enabled: false, agentLoaded: false, jobs: {} })),
}));

describe("lunchProblem", () => {
  it("wants an end after the start", () => {
    expect(lunchProblem("12:00", "13:00")).toBeNull();
    expect(lunchProblem("14:00", "13:00")).toBe("Lunch must end after it starts.");
    expect(lunchProblem("12:00", "12:00")).toBe("Lunch must end after it starts.");
    expect(lunchProblem("", "13:00")).not.toBeNull();
  });
});

describe("lunch times", () => {
  beforeEach(() => vi.mocked(SaveConfig).mockClear());

  it("moves lunch later when the start changes first", async () => {
    render(<WorkerSettings />);
    const start = await screen.findByLabelText("Lunch start");
    const end = screen.getByLabelText("Lunch end");

    // A start after the old end keeps the typed value, says why, and saves nothing.
    fireEvent.change(start, { target: { value: "14:00" } });
    fireEvent.focusOut(start, { relatedTarget: null });
    expect(start).toHaveValue("14:00");
    expect(screen.getByText(/Lunch must end after it starts/)).toBeInTheDocument();
    expect(SaveConfig).not.toHaveBeenCalled();

    // Moving to the end box is not leaving the pair, so nothing is checked yet.
    fireEvent.change(end, { target: { value: "15:00" } });
    fireEvent.focusOut(end, { relatedTarget: null });
    await waitFor(() => expect(SaveConfig).toHaveBeenCalledTimes(1));
    const saved = JSON.parse(vi.mocked(SaveConfig).mock.calls[0][0]);
    expect(saved.lunch).toEqual({ enabled: true, start: "14:00", end: "15:00" });
    expect(screen.queryByText(/Lunch must end after it starts/)).not.toBeInTheDocument();
  });

  it("does not check the pair while focus moves between its boxes", async () => {
    render(<WorkerSettings />);
    const start = await screen.findByLabelText("Lunch start");
    const end = screen.getByLabelText("Lunch end");
    fireEvent.change(start, { target: { value: "14:00" } });
    fireEvent.focusOut(start, { relatedTarget: end });
    expect(screen.queryByText(/Lunch must end after it starts/)).not.toBeInTheDocument();
    expect(SaveConfig).not.toHaveBeenCalled();
  });
});
