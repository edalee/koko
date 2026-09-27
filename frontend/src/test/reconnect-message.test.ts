import { describe, expect, it } from "vitest";
import { reconnectMessage } from "../hooks/useSessionTabs";

describe("reconnectMessage", () => {
  // Wails rejects with the Go error's text, so ErrConversationBusy arrives as a
  // plain string. It used to be swallowed into console.error, leaving the tab
  // looking broken with no reason given.
  it("explains a conversation that is open elsewhere", () => {
    expect(reconnectMessage("that conversation is already open in another session")).toBe(
      "This conversation is already open in another tab.",
    );
  });

  it("reads an Error's message", () => {
    expect(reconnectMessage(new Error("failed to start PTY: no such file"))).toBe(
      "Could not reconnect: failed to start PTY: no such file",
    );
  });

  it("falls back to the raw text for anything else", () => {
    expect(reconnectMessage("boom")).toBe("Could not reconnect: boom");
  });
});
