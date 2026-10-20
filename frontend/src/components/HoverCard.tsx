import {
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

const OPEN_DELAY_MS = 400;
// Long enough to move the pointer from the trigger onto the card.
const CLOSE_DELAY_MS = 150;
const GAP = 8;
const MARGIN = 8;

interface HoverCardProps {
  // The card's content. null turns the card off, for example while renaming.
  content: ReactNode | null;
  // Gets the card's id while it is open, for aria-describedby on the trigger.
  children: (describedBy: string | undefined) => ReactNode;
  className?: string;
}

/**
 * A small card beside its trigger, after a short delay on hover or keyboard
 * focus. Escape closes it. It renders into document.body, so a scrolling list
 * cannot clip it.
 *
 * Escape is never stopped here: dialogs listen for it on window.
 */
export default function HoverCard({ content, children, className }: HoverCardProps) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const triggerRef = useRef<HTMLDivElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clear = useCallback(() => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
  }, []);

  const show = useCallback(() => {
    clear();
    if (content === null) return;
    timer.current = setTimeout(() => setOpen(true), OPEN_DELAY_MS);
  }, [clear, content]);

  const hide = useCallback(
    (delay = CLOSE_DELAY_MS) => {
      clear();
      if (delay === 0) {
        setOpen(false);
        return;
      }
      timer.current = setTimeout(() => setOpen(false), delay);
    },
    [clear],
  );

  useEffect(() => clear, [clear]);

  // Turning the content off closes the card at once.
  useEffect(() => {
    if (content === null) hide(0);
  }, [content, hide]);

  // Escape anywhere closes it, even with focus in the terminal.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") hide(0);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, hide]);

  // Beside the trigger, moved up if it would run off the bottom.
  useLayoutEffect(() => {
    if (!open || !triggerRef.current) {
      setPos(null);
      return;
    }
    const rect = triggerRef.current.getBoundingClientRect();
    const height = cardRef.current?.offsetHeight ?? 0;
    const top = Math.max(MARGIN, Math.min(rect.top, window.innerHeight - height - MARGIN));
    setPos({ left: rect.right + GAP, top });
  }, [open]);

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: hover and focus only open a card, the trigger inside stays the control
    <div
      ref={triggerRef}
      className={className}
      onMouseEnter={show}
      onMouseLeave={() => hide()}
      onFocus={show}
      onBlur={(e) => {
        if (!cardRef.current?.contains(e.relatedTarget as Node | null)) hide(0);
      }}
    >
      {children(open ? id : undefined)}
      {open &&
        content !== null &&
        createPortal(
          <div
            ref={cardRef}
            id={id}
            role="tooltip"
            className="fixed z-[60] w-80 rounded-xl border border-glass-border bg-[rgba(15,17,23,0.92)] shadow-2xl glass-overlay inset-highlight p-3 animate-fade-in"
            style={{
              left: pos?.left ?? 0,
              top: pos?.top ?? 0,
              visibility: pos ? "visible" : "hidden",
            }}
            onMouseEnter={clear}
            onMouseLeave={() => hide()}
          >
            {content}
          </div>,
          document.body,
        )}
    </div>
  );
}
