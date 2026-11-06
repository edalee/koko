import { describe, expect, it } from "vitest";
import { toLogseq } from "../lib/logseq";

describe("toLogseq", () => {
  it("makes each paragraph and heading a block, and joins wrapped lines", () => {
    expect(
      toLogseq("# Plan\n\nThe worker runs a command\non a schedule.\n\nIt reads the exit code."),
    ).toBe("- # Plan\n- The worker runs a command\n  on a schedule.\n- It reads the exit code.");
  });

  it("nests list items, numbers numbered ones and turns tasks into TODO and DONE", () => {
    const md = [
      "- one",
      "  - one a",
      "    - deep",
      "- two",
      "1. first",
      "2. second",
      "- [ ] write it",
      "- [x] test it",
    ].join("\n");
    expect(toLogseq(md)).toBe(
      [
        "- one",
        "\t- one a",
        "\t\t- deep",
        "- two",
        "- first",
        "  logseq.order-list-type:: number",
        "- second",
        "  logseq.order-list-type:: number",
        "- TODO write it",
        "- DONE test it",
      ].join("\n"),
    );
  });

  it("keeps a fenced code block whole in one block, under its list item", () => {
    const md = [
      "- Run it:",
      "",
      "  ```bash",
      "  make build",
      "",
      "  make test",
      "  ```",
      "Done.",
    ].join("\n");
    expect(toLogseq(md)).toBe(
      [
        "- Run it:",
        "\t- ```bash",
        "\t  make build",
        "\t  ",
        "\t  make test",
        "\t  ```",
        "- Done.",
      ].join("\n"),
    );
  });

  it("turns a box table into a Markdown table, and keeps a Markdown one", () => {
    const box = [
      "┌──────┬───────┐",
      "│ Run  │ Exit  │",
      "├──────┼───────┤",
      "│ done │ 0     │",
      "├──────┼───────┤",
      "│ skip │ 2     │",
      "└──────┴───────┘",
    ].join("\n");
    expect(toLogseq(box)).toBe(
      ["- | Run | Exit |", "  | --- | --- |", "  | done | 0 |", "  | skip | 2 |"].join("\n"),
    );
    expect(toLogseq("| a | b |\n| --- | --- |\n| 1 | 2 |")).toBe(
      "- | a | b |\n  | --- | --- |\n  | 1 | 2 |",
    );
  });

  it("puts Claude Code's tool output under its line, and keeps quotes and rules", () => {
    const md = [
      "⏺ Reading the file.",
      "  ⎿  Read 40 lines",
      "> A quote",
      "> goes on",
      "",
      "---",
      "After.",
    ].join("\n");
    expect(toLogseq(md)).toBe(
      [
        "- Reading the file.",
        "\t- Read 40 lines",
        "- > A quote",
        "  > goes on",
        "- ---",
        "- After.",
      ].join("\n"),
    );
  });

  it("keeps a wrapped list item line in its item, and starts a block for a line under it", () => {
    expect(toLogseq("- a long item\n  that wraps\nA new paragraph.")).toBe(
      "- a long item\n  that wraps\n- A new paragraph.",
    );
  });

  it("gives nothing for an empty selection", () => {
    expect(toLogseq("\n\n")).toBe("");
  });
});
