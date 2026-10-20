import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "../../lib/utils";

// Group is one titled part of a section, such as Slack or Lunch.
export function Group({
  title,
  aside,
  children,
}: {
  title: string;
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between gap-2">
        <h5 className="text-xs text-white/85 font-medium">{title}</h5>
        {aside}
      </div>
      {children}
    </div>
  );
}

// Field is one row of the label and control grid. Help sits under the
// control, in the tertiary colour.
export function Field({
  label,
  htmlFor,
  help,
  children,
}: {
  label: string;
  htmlFor?: string;
  help?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-[6.5rem_1fr] gap-x-3 items-start">
      <label htmlFor={htmlFor} className="pt-1 text-[11px] text-muted-foreground">
        {label}
      </label>
      <div className="min-w-0 space-y-1">
        {children}
        {help && <p className="text-[10px] text-tertiary leading-relaxed">{help}</p>}
      </div>
    </div>
  );
}

// Block is one step of a job card: Schedule, Settings or Run.
export function Block({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="space-y-2.5 pt-3 border-t border-white/[0.05]">
      <p className="text-[10px] uppercase tracking-wider text-tertiary">{title}</p>
      {children}
    </div>
  );
}

// Disclosure is a button that opens and closes a part of the page. It shows a
// one-line summary under its title.
export function Disclosure({
  open,
  onToggle,
  title,
  summary,
  controls,
}: {
  open: boolean;
  onToggle: () => void;
  title: ReactNode;
  summary?: ReactNode;
  controls: string;
}) {
  return (
    <button
      type="button"
      aria-expanded={open}
      aria-controls={controls}
      onClick={onToggle}
      className="flex items-start gap-1.5 min-w-0 flex-1 text-left group"
    >
      <ChevronRight
        className={cn(
          "size-3.5 mt-0.5 shrink-0 text-tertiary group-hover:text-muted-foreground transition-transform",
          open && "rotate-90",
        )}
      />
      <span className="min-w-0 space-y-0.5">
        {title}
        {summary && <span className="block text-[10px] text-tertiary truncate">{summary}</span>}
      </span>
    </button>
  );
}

// Choice is a row of buttons, one of them picked.
export function Choice<T extends string>({
  value,
  options,
  onPick,
}: {
  value: T;
  options: { value: T; label: string }[];
  onPick: (value: T) => void;
}) {
  return (
    <div className="flex flex-wrap gap-1">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => onPick(o.value)}
          className={cn(
            "px-2.5 py-1 text-[11px] rounded-md border transition-colors",
            value === o.value
              ? "bg-white/[0.08] border-white/[0.12] text-white"
              : "border-white/[0.06] text-tertiary hover:text-muted-foreground",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
