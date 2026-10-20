import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DEFAULTS } from "../components/worker/config";
import SummaryCard from "../components/worker/SummaryCard";

describe("SummaryCard", () => {
  it("says it cannot run without a team, and saves a team on blur", () => {
    const onSave = vi.fn();
    render(<SummaryCard summary={DEFAULTS.summary} jiraSite="" onSave={onSave} />);
    expect(screen.getByText("The summary cannot run until a team is set.")).toBeTruthy();
    const team = screen.getByLabelText("Team");
    fireEvent.change(team, { target: { value: "platform" } });
    fireEvent.blur(team);
    expect(screen.getByText("Use org/team-slug")).toBeTruthy();
    expect(onSave).not.toHaveBeenCalled();
    fireEvent.change(team, { target: { value: " acme/platform " } });
    fireEvent.blur(team);
    expect(onSave).toHaveBeenCalledWith({
      team: "acme/platform",
      repos: [],
      jira: true,
      prose: true,
    });
  });

  it("adds repos for the team's org and saves the switches", () => {
    const onSave = vi.fn();
    const summary = { ...DEFAULTS.summary, team: "acme/platform", repos: ["acme/api"] };
    render(<SummaryCard summary={summary} jiraSite="acme.atlassian.net" onSave={onSave} />);
    fireEvent.change(screen.getByPlaceholderText("owner/repo, or repo for acme"), {
      target: { value: "platform" },
    });
    fireEvent.click(screen.getByRole("button", { name: /add/i }));
    expect(onSave).toHaveBeenLastCalledWith({ ...summary, repos: ["acme/api", "acme/platform"] });
    fireEvent.click(screen.getByRole("button", { name: "Tickets" }));
    expect(onSave).toHaveBeenLastCalledWith({ ...summary, jira: false });
    fireEvent.click(screen.getByRole("button", { name: "Paragraphs" }));
    expect(onSave).toHaveBeenLastCalledWith({ ...summary, prose: false });
  });
});
