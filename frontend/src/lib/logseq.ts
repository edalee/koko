// toLogseq turns Markdown, as "Copy as Markdown" makes it from the terminal,
// into Logseq's outliner Markdown (https://www.markdownguide.org/tools/logseq/):
//
//   - Every paragraph, heading, list item, quote, table and code block is a
//     block that starts with "- ". Its other lines are indented two spaces.
//   - A nested list item is a child block, one tab deeper.
//   - A numbered item gets the logseq.order-list-type:: number property, so
//     Logseq numbers it.
//   - A task list item becomes TODO or DONE, because Logseq has no task lists.
//   - A fenced code block stays whole in one block.
//   - A table drawn with box lines, as Claude Code shows one, becomes a
//     Markdown table.
//   - Blank lines go, because a blank line has no place in an outline.

// indent is where the block's text starts, and item is true for a list item,
// so a wrapped line can tell whether it belongs to the block.
type Block = { depth: number; lines: string[]; indent: number; item: boolean };

const BULLET = /^([-*+•⏺⎿])\s+(.*)$/;
const NUMBERED = /^(\d+)[.)]\s+(.*)$/;
const TASK = /^\[([ xX])\]\s+(.*)$/;
const FENCE = /^(`{3,}|~{3,})/;
const RULE = /^([-*_─━])(\s*\1){2,}$/;
const TABLE_ROW = /^[│┃|]/;
const TABLE_BORDER = /^[┌┏╭└┗╰├┣┝]/;
const TABLE_DIVIDER = /^[├┣┝]/;

export function toLogseq(markdown: string): string {
  const lines = markdown.replace(/\r\n?/g, "\n").split("\n");
  const blocks: Block[] = [];
  // listIndents holds the indent of each open list level, outermost first.
  const listIndents: number[] = [];
  let current: Block | null = null;
  let afterBlank = true;

  const add = (depth: number, first: string, indent = 0, item = false): Block => {
    const block = { depth, lines: [first], indent, item };
    blocks.push(block);
    return block;
  };
  // depthFor is the depth of a block that is not a list item at indent: a
  // child of the list item it sits under, else the top level.
  const depthFor = (indent: number) => {
    while (listIndents.length && listIndents[listIndents.length - 1] >= indent) listIndents.pop();
    return listIndents.length;
  };

  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i].trimEnd();
    const text = raw.trimStart();
    const indent = raw.length - text.length;

    if (text === "") {
      afterBlank = true;
      continue;
    }
    // A code block or table right under a line belongs to that line's block level.
    const depth = afterBlank || !current ? depthFor(indent) : current.depth;

    const fence = FENCE.exec(text);
    if (fence) {
      // The fenced code, up to its closing fence, is one block.
      current = add(depth, text);
      for (i++; i < lines.length; i++) {
        const line = lines[i].trimEnd();
        current.lines.push(line.slice(Math.min(indent, line.length - line.trimStart().length)));
        if (line.trim().startsWith(fence[1])) break;
      }
      // The block is closed, so the next line starts a new one.
      afterBlank = true;
      continue;
    }

    if (TABLE_ROW.test(text) || TABLE_BORDER.test(text)) {
      const rows: string[] = [];
      for (; i < lines.length; i++) {
        const row = lines[i].trim();
        if (!TABLE_ROW.test(row) && !TABLE_BORDER.test(row)) break;
        rows.push(row);
      }
      i--;
      const table = markdownTable(rows);
      if (table.length) {
        current = add(depth, table[0]);
        current.lines.push(...table.slice(1));
      }
      afterBlank = true;
      continue;
    }

    const bullet = BULLET.exec(text);
    const numbered = bullet ? null : NUMBERED.exec(text);
    if (bullet || numbered) {
      while (listIndents.length && listIndents[listIndents.length - 1] > indent) listIndents.pop();
      if (listIndents[listIndents.length - 1] !== indent) listIndents.push(indent);
      // Claude Code's tool output (⎿) sits under the line before it.
      const under = bullet?.[1] === "⎿" && current ? current.depth + 1 : 0;
      let item = bullet ? bullet[2] : (numbered?.[2] ?? "");
      const task = TASK.exec(item);
      if (task) item = `${task[1] === " " ? "TODO" : "DONE"} ${task[2]}`;
      current = add(Math.max(listIndents.length - 1, under), item, indent, true);
      if (numbered) current.lines.push("logseq.order-list-type:: number");
      afterBlank = false;
      continue;
    }

    if (RULE.test(text)) {
      current = add(depthFor(indent), "---");
      afterBlank = true;
      continue;
    }

    const wrapped =
      current &&
      !afterBlank &&
      !text.startsWith("#") &&
      !current.lines[0].startsWith("#") &&
      (current.item ? indent > current.indent : indent >= current.indent);
    if (current && wrapped) {
      // A wrapped line of the same paragraph, list item or quote.
      current.lines.push(text);
      continue;
    }
    current = add(depthFor(indent), text, indent);
    afterBlank = false;
  }

  return blocks
    .map((b) => {
      const tabs = "\t".repeat(b.depth);
      return [`${tabs}- ${b.lines[0]}`, ...b.lines.slice(1).map((l) => `${tabs}  ${l}`)].join("\n");
    })
    .join("\n");
}

// markdownTable turns table rows into Markdown table rows. Rows drawn with
// box lines become Markdown rows, and a Markdown table passes through. The
// first divider after the header becomes the header divider. Borders and the
// other dividers go.
function markdownTable(rows: string[]): string[] {
  const out: string[] = [];
  let header = false;
  for (const row of rows) {
    if (TABLE_BORDER.test(row)) {
      if (TABLE_DIVIDER.test(row) && !header && out.length === 1) {
        const cells = out[0].split("|").length - 2;
        out.push(`|${" --- |".repeat(Math.max(cells, 1))}`);
        header = true;
      }
      continue;
    }
    const cells = row
      .replace(/^[│┃|]|[│┃|]$/g, "")
      .split(/[│┃|]/)
      .map((c) => c.trim());
    if (cells.every((c) => /^:?-{3,}:?$/.test(c))) header = true;
    out.push(`| ${cells.join(" | ")} |`);
  }
  return out;
}
