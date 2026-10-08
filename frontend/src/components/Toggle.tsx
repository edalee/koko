import { cn } from "../lib/utils";

// Toggle is the on/off switch used in Settings.
export default function Toggle({
  id,
  on,
  onClick,
}: {
  id: string;
  on: boolean;
  onClick: () => void;
}) {
  return (
    <button
      id={id}
      type="button"
      onClick={onClick}
      className={cn(
        "w-8 h-[18px] rounded-full transition-colors relative shrink-0",
        on ? "bg-accent" : "bg-white/[0.12]",
      )}
    >
      <span
        className={cn(
          "absolute left-0 top-[2px] size-[14px] rounded-full bg-white transition-transform",
          on ? "translate-x-[16px]" : "translate-x-[2px]",
        )}
      />
    </button>
  );
}
