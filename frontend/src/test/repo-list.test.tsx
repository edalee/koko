import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import RepoList from "../components/RepoList";

function add(name: string) {
  fireEvent.change(screen.getByRole("textbox"), { target: { value: name } });
  fireEvent.click(screen.getByRole("button", { name: /add/i }));
}

describe("RepoList", () => {
  it("adds a bare repo name to the team's org", () => {
    const onChange = vi.fn();
    render(<RepoList repos={[]} org="acme" empty="" onChange={onChange} />);
    add("api");
    expect(onChange).toHaveBeenCalledWith(["acme/api"]);
  });

  it("lists org names in owners mode, and rejects owner/repo", () => {
    const onChange = vi.fn();
    render(<RepoList owners repos={["acme"]} org="" empty="" onChange={onChange} />);
    add("acme/api");
    expect(screen.getByText("Use an org or user name")).toBeTruthy();
    add("ACME");
    expect(screen.getByText("Already in the list")).toBeTruthy();
    add("jdoe");
    expect(onChange).toHaveBeenCalledWith(["acme", "jdoe"]);
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("accepts an EMU login with an underscore in owners mode", () => {
    const onChange = vi.fn();
    render(<RepoList owners repos={[]} org="" empty="" onChange={onChange} />);
    add("jdoe_acme");
    expect(onChange).toHaveBeenCalledWith(["jdoe_acme"]);
  });
});
