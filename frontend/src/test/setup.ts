import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// jsdom reports every element as 0x0. TerminalPane treats a zero-size box as
// "not laid out yet" and skips refitting, so component tests need a plausible
// default. Individual tests override this to assert the zero-size path.
Element.prototype.getBoundingClientRect = vi.fn(function (this: Element) {
  return {
    x: 0,
    y: 0,
    width: 800,
    height: 600,
    top: 0,
    left: 0,
    right: 800,
    bottom: 600,
    toJSON: () => ({}),
  } as DOMRect;
});

// Mock Wails runtime — EventsOn returns a cleanup function
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(() => vi.fn()),
  EventsOnce: vi.fn(() => vi.fn()),
  EventsOnMultiple: vi.fn(() => vi.fn()),
  EventsEmit: vi.fn(),
  EventsOff: vi.fn(),
  EventsOffAll: vi.fn(),
  BrowserOpenURL: vi.fn(),
}));

// Mock all Wails service bindings
vi.mock("../../wailsjs/go/main/TerminalManager", () => ({
  CreateSession: vi.fn().mockResolvedValue("session-1"),
  CreateShellSession: vi.fn().mockResolvedValue("shell-1"),
  CloseSession: vi.fn().mockResolvedValue(undefined),
  Write: vi.fn().mockResolvedValue(undefined),
  Resize: vi.fn().mockResolvedValue(undefined),
  ReplayBuffer: vi.fn().mockResolvedValue(""),
  GetSessions: vi.fn().mockResolvedValue([]),
  GetSessionState: vi.fn().mockResolvedValue("idle"),
  GetSessionPID: vi.fn().mockResolvedValue(0),
}));

vi.mock("../../wailsjs/go/main/ClaudeService", () => ({
  GetAgents: vi.fn().mockResolvedValue([]),
  GetCommands: vi.fn().mockResolvedValue([]),
  GetMCPServers: vi.fn().mockResolvedValue([]),
  GetLastMessage: vi.fn().mockResolvedValue(""),
}));

vi.mock("../../wailsjs/go/main/ConfigService", () => ({
  GetConfig: vi.fn().mockResolvedValue({}),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  GetSessions: vi.fn().mockResolvedValue({ tabs: [], history: [], recentDirs: [] }),
  SaveSessions: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("../../wailsjs/go/main/App", () => ({
  PickDirectory: vi.fn().mockResolvedValue(""),
  GetContextInfo: vi.fn().mockResolvedValue({}),
  GetVersion: vi.fn().mockResolvedValue("0.0.0"),
  CheckForUpdate: vi.fn().mockResolvedValue(null),
  EnsureStatusLine: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("../../wailsjs/go/main/GitHubService", () => ({
  FetchPRs: vi.fn().mockResolvedValue([]),
  ApprovePR: vi.fn().mockResolvedValue(undefined),
  MergePR: vi.fn().mockResolvedValue(undefined),
  FetchNotifications: vi.fn().mockResolvedValue([]),
  MarkNotificationRead: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("../../wailsjs/go/main/GitService", () => ({
  GetBranchName: vi.fn().mockResolvedValue("main"),
  GetFileChanges: vi.fn().mockResolvedValue([]),
}));

vi.mock("../../wailsjs/go/main/ProcessMonitor", () => ({
  GetChildProcesses: vi.fn().mockResolvedValue([]),
}));
