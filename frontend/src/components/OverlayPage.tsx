import { X } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";

interface OverlayPageProps {
  open: boolean;
  onClose: () => void;
  title: string;
  icon: ReactNode;
  children: ReactNode;
}

type AnimState = "closed" | "open" | "closing";

export default function OverlayPage({ open, onClose, title, icon, children }: OverlayPageProps) {
  const [state, setState] = useState<AnimState>("closed");

  useEffect(() => {
    if (open && state === "closed") {
      setState("open");
    } else if (!open && state === "open") {
      setState("closing");
    }
  }, [open, state]);

  function handleAnimationEnd() {
    if (state === "closing") {
      setState("closed");
    }
  }

  if (state === "closed") return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop click to close */}
      <div
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm ${
          state === "closing" ? "animate-backdrop-out" : "animate-backdrop-in"
        }`}
        onClick={onClose}
      />

      {/* Card */}
      <div
        className={`relative w-[520px] max-h-[70vh] flex flex-col rounded-2xl border shadow-2xl ${
          state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
        }`}
        style={{
          backgroundColor: "var(--color-glass)",
          borderColor: "var(--color-glass-border)",
          backdropFilter: "blur(40px)",
          WebkitBackdropFilter: "blur(40px)",
        }}
        onAnimationEnd={handleAnimationEnd}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/[0.08]">
          <div className="flex items-center gap-2.5">
            <span className="text-accent">{icon}</span>
            <h2 className="text-white text-sm font-medium">{title}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto">{children}</div>
      </div>
    </div>
  );
}
