import { Check, FolderX, History, Loader2 } from "lucide-react";
import { useState } from "react";
import { RemoveWorktrees } from "../../wailsjs/go/main/App";
import ConfirmDialog from "./ConfirmDialog";

export interface HousekeepingProps {
  historyCount: number;
  onClearHistory: () => void;
  // Worktrees Koko created for sessions that have closed, and the user kept
  // at close. A folder may since have gone, if the user deleted it by hand.
  worktrees: string[];
  onWorktreesRemoved: (paths: string[]) => void;
}

type Pending = "history" | "worktrees" | null;

/**
 * Settings > General: clear Koko's session history, and remove the worktrees
 * Koko created (plan 028 step 7b). Both are housekeeping, so they live here
 * rather than in the session dialog.
 */
export default function HousekeepingSettings({
  historyCount,
  onClearHistory,
  worktrees,
  onWorktreesRemoved,
}: HousekeepingProps) {
  const [pending, setPending] = useState<Pending>(null);
  const [historyCleared, setHistoryCleared] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [report, setReport] = useState<string[] | null>(null);

  const confirm = async () => {
    const what = pending;
    setPending(null);
    if (what === "history") {
      onClearHistory();
      setHistoryCleared(true);
      setTimeout(() => setHistoryCleared(false), 2000);
      return;
    }
    if (what !== "worktrees") return;
    setRemoving(true);
    try {
      const result = await RemoveWorktrees(worktrees);
      const removed = result.removed ?? [];
      const gone = result.gone ?? [];
      const skipped = result.skipped ?? [];
      onWorktreesRemoved([...removed, ...gone]);
      setReport([
        `Removed ${removed.length} ${removed.length === 1 ? "worktree" : "worktrees"}.`,
        ...gone.map((p) => `Already gone: ${p}. Run git worktree prune in its repo.`),
        ...skipped.map((s) => `Kept ${s.path}: ${s.reason}.`),
      ]);
    } catch (err) {
      setReport([String(err)]);
    } finally {
      setRemoving(false);
    }
  };

  const buttonClass =
    "flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded-lg transition-colors border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40 shrink-0";

  return (
    <div className="space-y-3 pt-3 border-t border-white/[0.06]">
      <div>
        <h4 className="text-sm text-white font-medium">Housekeeping</h4>
      </div>

      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-xs text-white/80">Session history</p>
          <p className="text-[10px] text-tertiary">
            {historyCount} closed {historyCount === 1 ? "session" : "sessions"} in Recent Sessions.
            Clearing them leaves Claude's conversations and any worktrees in place.
          </p>
        </div>
        <button
          type="button"
          disabled={historyCount === 0 || historyCleared}
          onClick={() => setPending("history")}
          className={buttonClass}
        >
          {historyCleared ? (
            <Check className="size-3 text-accent" />
          ) : (
            <History className="size-3 text-muted-foreground" />
          )}
          <span className="text-white">{historyCleared ? "Cleared" : "Clear history"}</span>
        </button>
      </div>

      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-xs text-white/80">Koko's worktrees</p>
          <p className="text-[10px] text-tertiary">
            {worktrees.length} {worktrees.length === 1 ? "worktree" : "worktrees"} kept from closed
            sessions. Any with uncommitted or ignored files stay: remove those from the Worktrees
            module.
          </p>
        </div>
        <button
          type="button"
          disabled={worktrees.length === 0 || removing}
          onClick={() => setPending("worktrees")}
          className={buttonClass}
        >
          {removing ? (
            <Loader2 className="size-3 animate-spin text-muted-foreground" />
          ) : (
            <FolderX className="size-3 text-muted-foreground" />
          )}
          <span className="text-white">Remove worktrees</span>
        </button>
      </div>

      {report && (
        <ul className="space-y-0.5">
          {report.map((line) => (
            <li key={line} className="text-[10px] text-muted-foreground break-all">
              {line}
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={pending !== null}
        title={pending === "history" ? "Clear session history?" : "Remove Koko's worktrees?"}
        message={
          pending === "history"
            ? worktrees.length > 0
              ? `Koko forgets its closed sessions. Claude's conversations stay. Koko also stops listing ${worktrees.length === 1 ? "1 kept worktree" : `${worktrees.length} kept worktrees`}, though the folders stay: remove them first if you want Koko to do it.`
              : "Koko forgets its closed sessions. Claude's conversations stay."
            : "This removes the worktree folders Koko created for closed sessions. Any with uncommitted or ignored files, or still in use, are kept."
        }
        confirmLabel={pending === "history" ? "Clear" : "Remove"}
        destructive
        onConfirm={confirm}
        onCancel={() => setPending(null)}
      />
    </div>
  );
}
