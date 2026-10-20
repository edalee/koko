import { type ReactNode, useState } from "react";
import Toggle from "../Toggle";
import { DEFAULTS, daysLabel, type JobConfig, jobTimes, nextSlot } from "./config";
import JobSchedule from "./JobSchedule";
import RunControls, { LastRun } from "./RunControls";
import { Block, Disclosure } from "./ui";
import type { WorkerConfigApi } from "./useWorkerConfig";

// CardProps are what the shell gives each job card.
export interface CardProps {
  api: WorkerConfigApi;
  running: string | null;
  setRunning: (running: string | null) => void;
}

// JobCard is one job: its title, switch and summary, then its schedule, its
// own settings, and Test and Run now. Closed, it still shows whether the job
// is on and when it runs next.
export default function JobCard({
  jobKey,
  name,
  about,
  multi = false,
  done,
  api,
  running,
  setRunning,
  children,
}: CardProps & {
  jobKey: string;
  name: string;
  about: string;
  multi?: boolean;
  done: string;
  children?: ReactNode;
}) {
  const { cfg, status, save, refreshStatus } = api;
  const [open, setOpen] = useState(false);
  const job = cfg.jobs[jobKey] ?? DEFAULTS.jobs[jobKey];
  const last = status?.jobs?.[jobKey]?.lastRun;
  // No job runs next if the worker is off, its agent is stopped, or its config
  // has an error. The General header says which.
  const runs = cfg.enabled && !status?.configError && status?.agentLoaded !== false;
  const next = runs ? nextSlot(job, cfg.timeZone) : null;
  const times = jobTimes(job);
  const titleId = `job-${jobKey}-title`;
  const bodyId = `job-${jobKey}-body`;

  const setJob = (next: JobConfig) => save({ ...cfg, jobs: { ...cfg.jobs, [jobKey]: next } });

  return (
    <section
      aria-labelledby={titleId}
      className="rounded-lg bg-white/[0.02] border border-white/[0.05]"
    >
      <div className="flex items-start gap-3 p-3">
        <Disclosure
          open={open}
          onToggle={() => setOpen(!open)}
          controls={bodyId}
          title={
            <>
              <span id={titleId} className="block text-xs text-white/90 font-medium">
                {name}
              </span>
              <span className="block text-[10px] text-tertiary">{about}</span>
            </>
          }
          summary={
            <>
              {job.enabled ? (
                <>
                  {daysLabel(job.days)} {times.length ? `at ${times.join(", ")}` : "with no times"}
                  {next && <> · next {next}</>}
                </>
              ) : (
                "Off"
              )}
              {!open && (
                <>
                  {" · "}
                  <LastRun last={last} />
                </>
              )}
            </>
          }
        />
        <Toggle
          id={`job-${jobKey}`}
          label={`${name} on`}
          on={job.enabled}
          onClick={() => setJob({ ...job, enabled: !job.enabled })}
        />
      </div>

      {/* Hidden, not removed, so a run's output stays when the card closes. */}
      <div id={bodyId} hidden={!open} className="px-3 pb-3 space-y-3">
        <Block title="Schedule">
          <JobSchedule jobKey={jobKey} job={job} multi={multi} onChange={setJob} />
        </Block>
        {children}
        <Block title="Run">
          <RunControls
            jobKey={jobKey}
            last={last}
            done={done}
            running={running}
            setRunning={setRunning}
            onRan={refreshStatus}
          />
        </Block>
      </div>
    </section>
  );
}
