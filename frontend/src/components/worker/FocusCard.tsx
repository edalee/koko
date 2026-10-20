import { cn } from "../../lib/utils";
import { inputClass } from "./config";
import JobCard, { type CardProps } from "./JobCard";
import { Block, Field } from "./ui";

// FocusCard is the focus time job. Its window and gap length save at once.
export default function FocusCard(props: CardProps) {
  const { cfg, save } = props.api;
  const setFocus = (patch: Partial<typeof cfg.focus>) =>
    save({ ...cfg, focus: { ...cfg.focus, ...patch } });
  return (
    <JobCard
      {...props}
      jobKey="focus"
      name="Focus time"
      about="Books focus blocks in the free gaps of your calendar, around lunch."
      done="Done. Check your Slack DMs."
    >
      <Block title="Settings">
        <Field label="Window" help="Blocks are booked only between these times.">
          <div className="flex items-center gap-2 text-[10px] text-tertiary">
            <input
              type="time"
              aria-label="Window start"
              value={cfg.focus.windowStart}
              onChange={(e) => setFocus({ windowStart: e.target.value })}
              className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
            />
            <span>to</span>
            <input
              type="time"
              aria-label="Window end"
              value={cfg.focus.windowEnd}
              onChange={(e) => setFocus({ windowEnd: e.target.value })}
              className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
            />
          </div>
        </Field>
        <Field
          label="Shortest gap"
          htmlFor="focus-min"
          help="A free gap shorter than this gets no block."
        >
          <div className="flex items-center gap-2 text-[10px] text-tertiary">
            <input
              id="focus-min"
              type="number"
              min={5}
              step={5}
              value={cfg.focus.minMinutes}
              onChange={(e) => setFocus({ minMinutes: Number.parseInt(e.target.value, 10) || 0 })}
              className={cn(inputClass, "w-14 tabular-nums")}
            />
            <span>minutes</span>
          </div>
        </Field>
      </Block>
    </JobCard>
  );
}
