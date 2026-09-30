import {
  AlertTriangle,
  Clock,
  Folder,
  FolderOpen,
  GitBranch,
  Loader2,
  Plus,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { PickDirectory } from "../../wailsjs/go/main/App";
import { ListConversations } from "../../wailsjs/go/main/ClaudeService";
import { GetSessions } from "../../wailsjs/go/main/ConfigService";
import { CreateWorktree, GetBranchName } from "../../wailsjs/go/main/GitService";
import type { main } from "../../wailsjs/go/models";
import { type ReconnectChoice, type ResumeTarget, reconnectMessage } from "../hooks/useSessionTabs";
import type { SessionHistoryEntry, SessionTab } from "../types";
import ConversationPicker, { type ConversationHolder, holderAction } from "./ConversationPicker";

function timeAgo(ts: number): string {
  const seconds = Math.floor((Date.now() - ts) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function shortenPath(path: string): string {
  const home = path.match(/^\/Users\/[^/]+/)?.[0];
  if (home) return path.replace(home, "~");
  return path;
}

interface SessionDialogProps {
  open: boolean;
  onClose: () => void;
  // Rejects when the session cannot start, for example when the conversation
  // is already open elsewhere. The dialog shows the reason and stays open.
  onCreate: (
    name: string,
    directory: string,
    worktreePath?: string,
    resume?: ResumeTarget,
  ) => Promise<void>;
  // Switch to the tab that already holds a conversation, reconnecting it if
  // it is disconnected. One conversation, one tab.
  onOpenHeld: (tabId: string) => void;
  // Reconnect mode: the dialog reconnects this tab rather than creating one.
  reconnect?: SessionTab;
  onReconnect: (tab: SessionTab, choice: ReconnectChoice) => Promise<void>;
  // Opened from the worktrees module on this existing worktree. The directory
  // is fixed, and the new tab is marked so close-time cleanup is offered.
  worktree?: string;
  history: SessionHistoryEntry[];
  activeDirs: string[];
  tabs: SessionTab[];
}

/**
 * A selection to apply once a directory's conversations have loaded.
 * uuid "" means a new conversation. null means nothing chosen yet (D1).
 */
interface PendingSelection {
  directory: string;
  uuid: string | null;
  // Shown if the conversation is older than the newest ones listed.
  fallback?: main.Conversation;
}

/** A closed session's conversation, shaped like a listed one. */
function historyConversation(entry: SessionHistoryEntry): main.Conversation {
  return {
    uuid: entry.claudeSessionId ?? "",
    title: entry.name,
    preview: entry.lastMessage ?? "",
    modifiedAt: entry.closedAt,
    sizeBytes: 0,
  };
}

/**
 * A tab's own conversation, shaped like a listed one. Its real date is not
 * known here, so modifiedAt is 0 and the picker shows no age rather than the
 * tab's creation date, which could be weeks before its last use.
 */
function tabConversation(tab: SessionTab): main.Conversation {
  return {
    uuid: tab.claudeSessionId ?? "",
    title: tab.name,
    preview: tab.lastMsg ?? "",
    modifiedAt: 0,
    sizeBytes: 0,
  };
}

type AnimState = "closed" | "open" | "closing";

function dirBasename(path: string): string {
  const parts = path.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || path;
}

function dirParent(path: string): string {
  const stripped = path.replace(/\/+$/, "");
  const idx = stripped.lastIndexOf("/");
  return idx === -1 ? stripped : stripped.slice(0, idx);
}

/** The main button's label, which says what pressing it will do. */
export function submitLabel(s: {
  creatingWorktree: boolean;
  holder?: ConversationHolder;
  selected: string | null;
  useWorktree: boolean;
  reconnecting?: boolean;
}): string {
  if (s.creatingWorktree) return "Creating worktree...";
  if (s.holder) {
    const action = holderAction(s.holder);
    return action.charAt(0).toUpperCase() + action.slice(1);
  }
  if (s.selected === null) return "Choose a conversation";
  if (s.selected) return "Resume Conversation";
  if (s.reconnecting) return "Start New Conversation";
  return s.useWorktree ? "Create Worktree + Session" : "Create Session";
}

function branchSlug(branch: string): string {
  return branch.replace(/[^a-zA-Z0-9-_]/g, "-").toLowerCase();
}

function randomSlug(): string {
  return Math.random().toString(36).slice(2, 8);
}

export default function SessionDialog({
  open,
  onClose,
  onCreate,
  onOpenHeld,
  reconnect: reconnectProp,
  onReconnect,
  worktree: worktreeProp,
  history,
  activeDirs,
  tabs,
}: SessionDialogProps) {
  // App remounts the dialog on every open (key), so everything below starts
  // from the props at that moment. Mode is frozen for the whole open: a tab
  // switch behind the dialog cannot swap its target while it is up, and the
  // first open render already has the right selection.
  const [reconnect] = useState(reconnectProp);
  const [worktree] = useState(worktreeProp);
  // A reconnect or an existing worktree fixes the directory.
  const locked = !!reconnect || !!worktree;

  const [state, setState] = useState<AnimState>("closed");
  const [name, setName] = useState("");
  const [directory, setDirectory] = useState(() => reconnect?.directory ?? worktree ?? "");
  const [recentDirs, setRecentDirs] = useState<string[]>([]);

  // Step 2: which conversation to open. "" means start a new one, null means
  // nothing is chosen yet, which only happens when reconnecting (D1). Set up
  // front rather than after the list loads, so Enter can never start a fresh
  // session for a D1 tab.
  const [conversations, setConversations] = useState<main.Conversation[] | null>([]);
  const [selected, setSelected] = useState<string | null>(() =>
    reconnect ? reconnect.claudeSessionId || null : "",
  );
  const [pending, setPending] = useState<PendingSelection | null>(() =>
    reconnect
      ? {
          directory: reconnect.directory,
          uuid: reconnect.claudeSessionId || null,
          fallback: reconnect.claudeSessionId ? tabConversation(reconnect) : undefined,
        }
      : null,
  );
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // Every tab that has a conversation open, dead or alive. A dead session
  // still owns its conversation, because reconnecting it resumes it. The tab
  // being reconnected is left out: its own conversation is the one to resume.
  const holders = useMemo(() => {
    const m = new Map<string, ConversationHolder>();
    for (const t of tabs) {
      if (t.claudeSessionId && t.id !== reconnect?.id) {
        m.set(t.claudeSessionId, {
          tabId: t.id,
          label: t.slug || t.name,
          connected: t.connected,
        });
      }
    }
    return m;
  }, [tabs, reconnect?.id]);
  const holder = selected ? holders.get(selected) : undefined;

  // Worktree state
  const [useWorktree, setUseWorktree] = useState(false);
  const [worktreeBranch, setWorktreeBranch] = useState("");
  const [worktreePath, setWorktreePath] = useState("");
  const [createNewBranch, setCreateNewBranch] = useState(true);
  const [currentBranch, setCurrentBranch] = useState<string | null>(null);
  const [creatingWorktree, setCreatingWorktree] = useState(false);
  const [worktreeError, setWorktreeError] = useState<string | null>(null);

  const dirCollides = useMemo(
    () => directory !== "" && activeDirs.includes(directory),
    [directory, activeDirs],
  );

  // Load recent dirs from Go backend
  useEffect(() => {
    if (open) {
      GetSessions()
        .then((s) => setRecentDirs(s.recentDirs || []))
        .catch(() => {});
    }
  }, [open]);

  // Read through a ref so the effect below runs on a directory change only.
  // App passes a fresh array on every render, and the polling hooks re-render
  // it every second or so, which re-seeded these defaults each time: a new
  // random branch name, any typed name overwritten, and the toggle reset.
  const activeDirsRef = useRef(activeDirs);
  activeDirsRef.current = activeDirs;

  // When a directory is picked, look up its current branch and seed worktree
  // defaults. If the dir collides with another active session, auto-enable
  // the worktree toggle.
  useEffect(() => {
    const activeDirs = activeDirsRef.current;
    if (!directory) {
      setCurrentBranch(null);
      setWorktreeBranch("");
      setWorktreePath("");
      setUseWorktree(false);
      setWorktreeError(null);
      return;
    }
    // Never when the directory is fixed. Opening an existing worktree that
    // another tab uses would otherwise tick this, and Enter would create a
    // new worktree inside the worktree.
    setUseWorktree(!locked && activeDirs.includes(directory));
    GetBranchName(directory)
      .then((branch) => {
        setCurrentBranch(branch);
        const base = branchSlug(branch || "wt");
        const suffix = randomSlug();
        const newBranch = `${base}-${suffix}`;
        setWorktreeBranch(newBranch);
        setWorktreePath(`${dirParent(directory)}/${dirBasename(directory)}-${suffix}`);
      })
      .catch(() => {
        setCurrentBranch(null);
        const suffix = randomSlug();
        setWorktreeBranch(`wt-${suffix}`);
        setWorktreePath(`${dirParent(directory)}/${dirBasename(directory)}-${suffix}`);
      });
  }, [directory, locked]);

  // Load the directory's conversations whenever it changes. A counter drops
  // replies that arrive after the user has moved on to another directory.
  const loadSeq = useRef(0);
  useEffect(() => {
    const seq = ++loadSeq.current;
    const pick = pending && pending.directory === directory ? pending : null;
    // Select straight away, not after the load. On the first run this repeats
    // the initial selection. After a directory change it resets to new.
    setSelected(pick ? pick.uuid : "");
    if (pick?.uuid) setUseWorktree(false);
    setCreateError(null);
    if (!directory) {
      setConversations([]);
      return;
    }
    setConversations(null);
    ListConversations(directory)
      .catch(() => [] as main.Conversation[])
      .then((list) => {
        if (seq !== loadSeq.current) return;
        let rows = list ?? [];
        // A preselected conversation may be older than the newest 20 listed.
        // Keep it reachable rather than silently dropping it, which is the
        // bug Recent Sessions had.
        if (pick?.uuid && pick.fallback && !rows.some((c) => c.uuid === pick.uuid)) {
          rows = [pick.fallback, ...rows];
        }
        setConversations(rows);
      });
  }, [directory, pending]);

  const nameRef = useRef<HTMLInputElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open && state === "closed") {
      setState("open");
    } else if (!open && state === "open") {
      setState("closing");
    }
  }, [open, state]);

  // Take focus from the terminal behind as soon as the panel is up. Without
  // it the terminal received the same Escape and Enter, which interrupted or
  // submitted to Claude. A reconnect has no name field, so focus the panel.
  useEffect(() => {
    if (state !== "open") return;
    (reconnect ? panelRef.current : nameRef.current)?.focus();
  }, [state, reconnect]);

  function handleAnimationEnd() {
    if (state === "closing") {
      setState("closed");
    }
  }

  const handleBrowse = useCallback(async () => {
    const dir = await PickDirectory();
    if (dir) setDirectory(dir);
  }, []);

  const handleCreate = useCallback(async () => {
    // null means nothing is chosen yet (D1), so there is nothing to do.
    if (!directory || creating || selected === null) return;
    setWorktreeError(null);
    setCreateError(null);

    // D2: a conversation another tab holds is opened in that tab, never
    // twice. Switching reconnects it if it is disconnected.
    if (holder) {
      onOpenHeld(holder.tabId);
      return;
    }

    // Reconnect mode: restart this same tab, keeping its slug, into the
    // chosen conversation or a fresh one.
    if (reconnect) {
      setCreating(true);
      try {
        await onReconnect(
          reconnect,
          selected === "" ? { fresh: true } : { claudeSessionId: selected },
        );
      } catch (err) {
        setCreateError(reconnectMessage(err));
      } finally {
        setCreating(false);
      }
      return;
    }

    let finalDir = directory;
    // A new worktree has no conversations, so the two never combine.
    if (useWorktree && !selected && !locked) {
      if (!worktreeBranch.trim() || !worktreePath.trim()) {
        setWorktreeError("Branch and worktree path are required");
        return;
      }
      setCreatingWorktree(true);
      try {
        const wt = await CreateWorktree(
          directory,
          worktreePath.trim(),
          worktreeBranch.trim(),
          createNewBranch,
        );
        finalDir = wt.path || worktreePath.trim();
      } catch (err) {
        setCreatingWorktree(false);
        setWorktreeError(
          err instanceof Error ? err.message : String(err ?? "Failed to create worktree"),
        );
        return;
      }
      setCreatingWorktree(false);
    }

    const inWorktree = useWorktree && !selected && !locked;
    const sessionName =
      name.trim() || (inWorktree ? branchSlug(worktreeBranch) : dirBasename(finalDir));

    let resume: ResumeTarget | undefined;
    if (selected) {
      // Reuse the slug the conversation had when its tab closed, so koko-1
      // still names it.
      const closed = history.find((h) => h.claudeSessionId === selected);
      resume = { claudeSessionId: selected, slug: closed?.slug || undefined };
    }

    setCreating(true);
    try {
      await onCreate(sessionName, finalDir, inWorktree ? finalDir : worktree, resume);
    } catch (err) {
      // Most likely the conversation was opened elsewhere while the dialog
      // was up. Say so and stay open, rather than failing silently.
      setCreateError(reconnectMessage(err));
    } finally {
      setCreating(false);
    }
  }, [
    name,
    directory,
    creating,
    holder,
    onOpenHeld,
    reconnect,
    onReconnect,
    locked,
    worktree,
    selected,
    history,
    useWorktree,
    worktreeBranch,
    worktreePath,
    createNewBranch,
    onCreate,
  ]);

  // Picking a conversation and creating a worktree are mutually exclusive.
  const selectConversation = useCallback((uuid: string) => {
    setSelected(uuid);
    setCreateError(null);
    if (uuid) setUseWorktree(false);
  }, []);

  useEffect(() => {
    if (state !== "open") return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        onClose();
      } else if (e.key === "Enter" && directory && !creatingWorktree) {
        handleCreate();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state, directory, creatingWorktree, onClose, handleCreate]);

  if (state === "closed") return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop */}
      <div
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm ${
          state === "closing" ? "animate-backdrop-out" : "animate-backdrop-in"
        }`}
        onClick={onClose}
      />

      <div
        ref={panelRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        className={`relative w-[480px] flex flex-col rounded-xl border shadow-2xl glass-overlay inset-highlight outline-none ${
          state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
        }`}
        style={{
          backgroundColor: "rgba(255, 255, 255, 0.08)",
          borderColor: "var(--color-glass-border)",
        }}
        onAnimationEnd={handleAnimationEnd}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/[0.08]">
          <div className="flex items-center gap-2.5">
            <span className="text-accent">
              <Plus className="size-4" />
            </span>
            <h2 className="text-white text-sm font-medium">
              {reconnect
                ? // A live tab lands here when reloaded with no stored
                  // conversation, so it is a reload rather than a reconnect.
                  `${reconnect.connected ? "Reload" : "Reconnect"} ${reconnect.slug || reconnect.name}`
                : "New Session"}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-md text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Content */}
        <div className="p-5 space-y-5">
          {/* Name input. A reconnected tab keeps its name. */}
          {!reconnect && (
            <div className="space-y-2">
              <label
                className="text-xs font-medium text-muted-foreground uppercase tracking-wider"
                htmlFor="session-name"
              >
                Session Name
              </label>
              <input
                ref={nameRef}
                id="session-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={directory ? dirBasename(directory) : "Session name"}
                className="w-full px-3 py-2.5 text-sm bg-white/[0.04] border border-white/[0.06] rounded-lg text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
              />
            </div>
          )}

          {/* Directory picker */}
          <div className="space-y-2">
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
              Project Directory
            </span>

            {locked ? (
              // Fixed: a reconnect stays in the tab's own directory, and an
              // existing worktree is the directory.
              <div className="w-full flex items-center gap-3 px-3 py-2.5 text-sm bg-white/[0.04] border border-white/[0.06] rounded-md text-white/80">
                <FolderOpen className="size-4 text-muted-foreground shrink-0" />
                <span className="truncate">{shortenPath(directory)}</span>
              </div>
            ) : directory ? (
              <button
                type="button"
                onClick={handleBrowse}
                className="w-full flex items-center gap-3 px-3 py-2.5 text-sm bg-accent/10 border border-accent/30 rounded-md text-white hover:bg-accent/15 transition-colors text-left"
              >
                <FolderOpen className="size-4 text-accent shrink-0" />
                <span className="truncate">{shortenPath(directory)}</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={handleBrowse}
                className="w-full flex items-center justify-center gap-2 px-3 py-3 text-sm bg-white/5 border border-border border-dashed rounded-md text-muted-foreground hover:text-white hover:border-accent/30 hover:bg-white/[0.07] transition-colors"
              >
                <Folder className="size-4" />
                <span>Browse...</span>
              </button>
            )}

            {/* D1: a tab saved before conversation ids were captured. */}
            {reconnect && !reconnect.claudeSessionId && (
              <p className="text-[12px] text-muted-foreground">
                This tab has no stored conversation. Choose one to reopen, or start a new one.
              </p>
            )}

            {/* Worktree section, shown once a directory is selected. Not when
                the directory is fixed: a reconnect stays put, and an existing
                worktree needs no new one. */}
            {directory && !locked && (
              <div className="mt-3 rounded-md border border-white/[0.06] bg-white/[0.02] overflow-hidden">
                {dirCollides && (
                  <div className="flex items-start gap-2 px-3 py-2 bg-warning/8 border-b border-warning/15 text-[11px] text-warning">
                    <AlertTriangle className="size-3.5 shrink-0 mt-0.5" />
                    <span>
                      Another session is using this directory. Sessions on the same directory can
                      pollute each other&apos;s files, builds, and git state.
                    </span>
                  </div>
                )}
                <label className="flex items-center gap-2 px-3 py-2 cursor-pointer select-none hover:bg-white/[0.03] transition-colors">
                  <input
                    type="checkbox"
                    checked={useWorktree}
                    onChange={(e) => {
                      setUseWorktree(e.target.checked);
                      // A new worktree has no conversations to reopen.
                      if (e.target.checked) setSelected("");
                    }}
                    className="accent-accent"
                  />
                  <GitBranch className="size-3.5 text-muted-foreground" />
                  <span className="text-[12px] text-white/90">Create as a git worktree</span>
                  {currentBranch && (
                    <span className="ml-auto text-[10px] text-tertiary">from {currentBranch}</span>
                  )}
                </label>

                {useWorktree && (
                  <div className="px-3 pb-3 space-y-2.5 border-t border-white/[0.06] pt-2.5">
                    <div className="space-y-1">
                      <label
                        className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider"
                        htmlFor="worktree-branch"
                      >
                        Branch
                      </label>
                      <input
                        id="worktree-branch"
                        value={worktreeBranch}
                        onChange={(e) => setWorktreeBranch(e.target.value)}
                        className="w-full px-2.5 py-1.5 text-[12px] font-mono bg-white/[0.04] border border-white/[0.06] rounded text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
                      />
                      <label className="flex items-center gap-2 text-[11px] text-muted-foreground cursor-pointer select-none">
                        <input
                          type="checkbox"
                          checked={createNewBranch}
                          onChange={(e) => setCreateNewBranch(e.target.checked)}
                          className="accent-accent"
                        />
                        Create as new branch
                        {!createNewBranch && (
                          <span className="text-[10px] text-tertiary">(checkout existing)</span>
                        )}
                      </label>
                    </div>
                    <div className="space-y-1">
                      <label
                        className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider"
                        htmlFor="worktree-path"
                      >
                        Worktree Path
                      </label>
                      <input
                        id="worktree-path"
                        value={worktreePath}
                        onChange={(e) => setWorktreePath(e.target.value)}
                        className="w-full px-2.5 py-1.5 text-[11px] font-mono bg-white/[0.04] border border-white/[0.06] rounded text-white/80 placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors"
                      />
                    </div>
                    {worktreeError && (
                      <div className="flex items-start gap-2 px-2 py-1.5 rounded bg-error/10 border border-error/20 text-[11px] text-error">
                        <AlertTriangle className="size-3 shrink-0 mt-0.5" />
                        <span className="break-words">{worktreeError}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Step 2: new conversation, or one of those already stored */}
            {directory && (
              <div className="pt-2">
                <ConversationPicker
                  conversations={conversations}
                  selected={selected}
                  onSelect={selectConversation}
                  holders={holders}
                  // D1 starts with nothing chosen, so the new-conversation row
                  // must show even with no stored conversations, or the tab
                  // could never be reconnected.
                  showWhenEmpty={!!reconnect}
                />
              </div>
            )}

            {/* Session history / Recent directories */}
            {!directory && (
              <div className="space-y-1 pt-1">
                {history.length > 0 && (
                  <>
                    <span className="text-xs text-muted-foreground/60 px-1 flex items-center gap-1.5">
                      <Clock className="size-3" />
                      Recent Sessions
                    </span>
                    {history
                      .filter((h) => !activeDirs.includes(h.directory))
                      .slice(0, 8)
                      .map((entry, idx) => (
                        <button
                          type="button"
                          key={`${entry.directory}-${entry.closedAt}-${idx}`}
                          onClick={() => {
                            // Select, don't create. This used to start a
                            // fresh session and drop the stored
                            // conversation. Now it opens the picker with
                            // that conversation chosen, so Enter reopens it.
                            setName(entry.name);
                            setPending({
                              directory: entry.directory,
                              uuid: entry.claudeSessionId ?? "",
                              fallback: entry.claudeSessionId
                                ? historyConversation(entry)
                                : undefined,
                            });
                            setDirectory(entry.directory);
                          }}
                          className="w-full flex items-center gap-2.5 px-3 py-2 text-sm rounded-md text-muted-foreground hover:text-white hover:bg-white/5 transition-colors text-left"
                        >
                          <Folder className="size-3.5 shrink-0" />
                          <div className="flex-1 min-w-0">
                            <span className="text-white/80 block truncate">{entry.name}</span>
                            <span className="text-[10px] text-tertiary block truncate">
                              {shortenPath(entry.directory)}
                            </span>
                            {entry.lastMessage && (
                              <span className="text-[10px] text-muted-foreground/50 block truncate mt-0.5 italic">
                                {entry.lastMessage}
                              </span>
                            )}
                          </div>
                          <span className="text-[10px] text-tertiary shrink-0">
                            {timeAgo(entry.closedAt)}
                          </span>
                        </button>
                      ))}
                  </>
                )}
                {history.length === 0 && recentDirs.length > 0 && (
                  <>
                    <span className="text-xs text-muted-foreground/60 px-1">Recent</span>
                    {recentDirs.map((dir) => (
                      <button
                        type="button"
                        key={dir}
                        onClick={() => setDirectory(dir)}
                        className="w-full flex items-center gap-2.5 px-3 py-2 text-sm rounded-md text-muted-foreground hover:text-white hover:bg-white/5 transition-colors text-left"
                      >
                        <Folder className="size-3.5 shrink-0" />
                        <span className="truncate">{shortenPath(dir)}</span>
                      </button>
                    ))}
                  </>
                )}
              </div>
            )}
          </div>
        </div>

        {createError && (
          <div
            role="alert"
            className="mx-5 mb-1 flex items-start gap-2 px-3 py-2 rounded bg-error/10 border border-error/20 text-[12px] text-error"
          >
            <AlertTriangle className="size-3.5 shrink-0 mt-0.5" />
            <span className="break-words">{createError}</span>
          </div>
        )}

        {/* Footer */}
        <div className="px-5 py-4 border-t border-white/[0.08] flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm rounded-md text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleCreate}
            disabled={!directory || creatingWorktree || creating || selected === null}
            className="px-4 py-2 text-sm rounded-md font-medium transition-all disabled:opacity-30 disabled:cursor-not-allowed text-white relative overflow-hidden border-2 border-transparent flex items-center gap-2"
            style={{
              background: directory
                ? "linear-gradient(rgba(255,255,255,0.08), rgba(255,255,255,0.08)) padding-box, linear-gradient(to right, #1FF2AB, #24A965) border-box"
                : undefined,
            }}
          >
            {(creatingWorktree || creating) && <Loader2 className="size-3.5 animate-spin" />}
            {submitLabel({
              creatingWorktree,
              holder,
              selected,
              useWorktree,
              reconnecting: !!reconnect,
            })}
          </button>
        </div>
      </div>
    </div>
  );
}
