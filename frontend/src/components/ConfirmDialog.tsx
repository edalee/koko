import { AlertTriangle } from "lucide-react";
import { useEffect, useRef, useState } from "react";

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  // One or two short sentences: what happens, and what is lost.
  message: string;
  confirmLabel: string;
  // Destructive actions use the error colour, so they read as such.
  destructive?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

type AnimState = "closed" | "open" | "closing";

/**
 * A small yes-or-no dialog for an action that loses something. Escape and a
 * backdrop click cancel.
 *
 * The dialog takes focus when it opens. Without that, the terminal kept focus
 * and received the same keypress: Escape interrupted the Claude the dialog was
 * protecting, and Enter submitted its half-typed prompt. Enter acts on the
 * focused button, and a destructive dialog focuses Cancel, so a stray Enter
 * cancels rather than confirms.
 */
export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  destructive = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const [state, setState] = useState<AnimState>("closed");
  const cancelRef = useRef<HTMLButtonElement>(null);
  const confirmRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (open && state === "closed") setState("open");
    else if (!open && state === "open") setState("closing");
  }, [open, state]);

  // Take focus away from the terminal as soon as the dialog is up.
  useEffect(() => {
    if (state !== "open") return;
    (destructive ? cancelRef : confirmRef).current?.focus();
  }, [state, destructive]);

  // Escape only. Enter is left to the focused button, so it can never
  // confirm while Cancel has focus.
  useEffect(() => {
    if (state !== "open") return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onCancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [state, onCancel]);

  if (state === "closed") return null;

  const confirmClass = destructive
    ? "bg-error/20 text-error border-error/30 hover:bg-error/30"
    : "bg-accent/15 text-accent border-accent/30 hover:bg-accent/25";

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop, Escape also cancels */}
      <div
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm ${
          state === "closing" ? "animate-backdrop-out" : "animate-backdrop-in"
        }`}
        onClick={onCancel}
      />
      <div
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        className={`relative w-[400px] rounded-xl border shadow-2xl glass-overlay inset-highlight ${
          state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
        }`}
        style={{
          backgroundColor: "rgba(255, 255, 255, 0.08)",
          borderColor: "var(--color-glass-border)",
        }}
        onAnimationEnd={() => {
          if (state === "closing") setState("closed");
        }}
      >
        <div className="px-5 py-4 border-b border-white/[0.08] flex items-center gap-2.5">
          <AlertTriangle className={`size-4 ${destructive ? "text-error" : "text-warning"}`} />
          <h2 id="confirm-title" className="text-white text-sm font-medium">
            {title}
          </h2>
        </div>
        <p className="px-5 py-4 text-[12px] text-white/85 leading-relaxed">{message}</p>
        <div className="px-5 py-3 border-t border-white/[0.08] flex justify-end gap-2">
          <button
            ref={cancelRef}
            type="button"
            onClick={onCancel}
            className="px-3 py-1.5 text-xs rounded-md text-muted-foreground hover:text-white hover:bg-white/[0.06] transition-colors focus-visible:ring-1 focus-visible:ring-white/40 outline-none"
          >
            Cancel
          </button>
          <button
            ref={confirmRef}
            type="button"
            onClick={onConfirm}
            className={`px-3 py-1.5 text-xs rounded-md border transition-colors ${confirmClass}`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
