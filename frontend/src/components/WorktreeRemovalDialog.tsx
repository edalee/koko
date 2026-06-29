import { AlertTriangle, Folder, GitBranch, Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { RemoveWorktree } from "../../wailsjs/go/main/GitService";

function shortenPath(path: string): string {
  const home = path.match(/^\/Users\/[^/]+/)?.[0];
  if (home) return path.replace(home, "~");
  return path;
}

interface WorktreeRemovalDialogProps {
  open: boolean;
  worktreePath: string;
  sessionName: string;
  onResolved: () => void; // called after the user makes a decision (close proceeds)
}

type AnimState = "closed" | "open" | "closing";

/**
 * Asks the user whether to delete the Koko-created worktree when its
 * session is being closed. Always proceeds with the session close
 * (via onResolved) regardless of the worktree decision.
 */
export default function WorktreeRemovalDialog({
  open,
  worktreePath,
  sessionName,
  onResolved,
}: WorktreeRemovalDialogProps) {
  const [state, setState] = useState<AnimState>("closed");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [needsForce, setNeedsForce] = useState(false);

  useEffect(() => {
    if (open && state === "closed") {
      setState("open");
      setBusy(false);
      setError(null);
      setNeedsForce(false);
    } else if (!open && state === "open") {
      setState("closing");
    }
  }, [open, state]);

  function handleAnimationEnd() {
    if (state === "closing") setState("closed");
  }

  async function attemptRemove(force: boolean) {
    setBusy(true);
    setError(null);
    try {
      await RemoveWorktree(worktreePath, force);
      onResolved();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      // git worktree remove without --force surfaces "dirty" / "contains
      // refs" — offer the force path.
      if (!force) setNeedsForce(true);
    } finally {
      setBusy(false);
    }
  }

  function handleKeep() {
    onResolved();
  }

  if (state === "closed") return null;

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop */}
      <div
        className={`absolute inset-0 bg-black/50 backdrop-blur-sm ${
          state === "closing" ? "animate-backdrop-out" : "animate-backdrop-in"
        }`}
        onClick={busy ? undefined : handleKeep}
      />
      <div
        className={`relative w-[440px] rounded-xl border shadow-2xl glass-overlay inset-highlight ${
          state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
        }`}
        style={{
          backgroundColor: "rgba(255, 255, 255, 0.08)",
          borderColor: "var(--color-glass-border)",
        }}
        onAnimationEnd={handleAnimationEnd}
      >
        <div className="px-5 py-4 border-b border-white/[0.08] flex items-center gap-2.5">
          <GitBranch className="size-4 text-warning" />
          <h2 className="text-white text-sm font-medium">Remove worktree?</h2>
        </div>

        <div className="px-5 py-4 space-y-3">
          <p className="text-[12px] text-white/85 leading-relaxed">
            Koko created a worktree for{" "}
            <span className="text-white font-medium">{sessionName}</span>. Closing the session
            won&apos;t remove it automatically.
          </p>
          <div className="flex items-start gap-2 px-3 py-2 rounded-md bg-white/[0.04] border border-white/[0.06]">
            <Folder className="size-3.5 text-muted-foreground shrink-0 mt-0.5" />
            <span className="text-[11px] font-mono text-white/80 break-all">
              {shortenPath(worktreePath)}
            </span>
          </div>

          {error && (
            <div className="flex items-start gap-2 px-2.5 py-2 rounded-md bg-error/10 border border-error/20 text-[11px] text-error">
              <AlertTriangle className="size-3.5 shrink-0 mt-0.5" />
              <span className="break-words">{error}</span>
            </div>
          )}
        </div>

        <div className="px-5 py-3 border-t border-white/[0.08] flex justify-end gap-2">
          <button
            type="button"
            onClick={handleKeep}
            disabled={busy}
            className="px-3 py-1.5 text-xs rounded-md text-muted-foreground hover:text-white hover:bg-white/[0.06] transition-colors disabled:opacity-40"
          >
            Keep worktree
          </button>
          {needsForce ? (
            <button
              type="button"
              onClick={() => attemptRemove(true)}
              disabled={busy}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md bg-error/20 text-error border border-error/30 hover:bg-error/30 transition-colors disabled:opacity-40"
            >
              {busy && <Loader2 className="size-3 animate-spin" />}
              Force remove
            </button>
          ) : (
            <button
              type="button"
              onClick={() => attemptRemove(false)}
              disabled={busy}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md bg-accent/15 text-accent border border-accent/30 hover:bg-accent/25 transition-colors disabled:opacity-40"
            >
              {busy && <Loader2 className="size-3 animate-spin" />}
              Remove
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
