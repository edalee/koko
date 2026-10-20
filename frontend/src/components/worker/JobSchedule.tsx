import { Plus, X } from "lucide-react";
import { cn } from "../../lib/utils";
import { DAYS, dayName, inputClass, type JobConfig, toggleDay } from "./config";
import { Field } from "./ui";

// JobSchedule is a job's days and times. With multi set, the job can run at
// more than one time a day.
export default function JobSchedule({
  jobKey,
  job,
  multi = false,
  onChange,
}: {
  jobKey: string;
  job: JobConfig;
  multi?: boolean;
  onChange: (job: JobConfig) => void;
}) {
  return (
    <>
      <Field label="Days">
        <div className="flex flex-wrap gap-1">
          {DAYS.map((day) => (
            <button
              key={day}
              type="button"
              aria-pressed={job.days.includes(day)}
              onClick={() => onChange({ ...job, days: toggleDay(job.days, day) })}
              className={cn(
                "w-8 py-0.5 text-[10px] rounded-md border transition-colors",
                job.days.includes(day)
                  ? "bg-white/[0.08] border-white/[0.12] text-white"
                  : "border-white/[0.06] text-tertiary hover:text-muted-foreground",
              )}
            >
              {dayName(day)}
            </button>
          ))}
        </div>
      </Field>
      <Field label={multi ? "Times" : "Time"}>
        <div className="flex flex-wrap items-center gap-1.5">
          {job.times.map((t, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: each time is edited in place by its position
            <span key={`${jobKey}-${i}`} className="inline-flex items-center gap-1">
              <input
                type="time"
                aria-label={`Time ${i + 1}`}
                value={t}
                onChange={(e) => {
                  const times = [...job.times];
                  times[i] = e.target.value;
                  onChange({ ...job, times });
                }}
                className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
              />
              {multi && job.times.length > 1 && (
                <button
                  type="button"
                  onClick={() => onChange({ ...job, times: job.times.filter((_, j) => j !== i) })}
                  className="opacity-50 hover:opacity-100 hover:text-error"
                  title="Remove this time"
                >
                  <X className="size-3" />
                </button>
              )}
            </span>
          ))}
          {multi && (
            <button
              type="button"
              onClick={() => onChange({ ...job, times: [...job.times, "16:00"] })}
              className="p-1 rounded-md border border-white/[0.08] hover:bg-white/[0.06]"
              title="Add a time"
            >
              <Plus className="size-3" />
            </button>
          )}
        </div>
      </Field>
    </>
  );
}
