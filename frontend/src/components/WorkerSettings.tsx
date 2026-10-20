import { useState } from "react";
import { ReadLog } from "../../wailsjs/go/main/WorkerService";
import FocusCard from "./worker/FocusCard";
import GeneralSection from "./worker/GeneralSection";
import JobCard from "./worker/JobCard";
import ReviewCard from "./worker/ReviewCard";
import StandupCard from "./worker/StandupCard";
import SummaryCard from "./worker/SummaryCard";
import { useWorkerConfig } from "./worker/useWorkerConfig";

// WorkerSettings is the Worker tab: General, then one card per job, then the
// log. Each section lives in worker/.
export default function WorkerSettings() {
  const api = useWorkerConfig();
  // running is the job that runs now, shared so only one runs at a time.
  const [running, setRunning] = useState<string | null>(null);
  const [log, setLog] = useState<string | null>(null);
  const card = { api, running, setRunning };

  return (
    <div className="space-y-6">
      <GeneralSection api={api} />

      {/* Job edits wait while the master switch runs, because it writes worker.json too. */}
      <fieldset
        disabled={api.switching}
        className="min-w-0 space-y-3 pt-5 border-t border-white/[0.06] disabled:opacity-60"
      >
        <h4 className="text-sm text-white font-medium">Jobs</h4>
        <StandupCard {...card} />
        <FocusCard {...card} />
        <ReviewCard {...card} />
        <JobCard
          {...card}
          jobKey="summary"
          name="Team summary"
          about="A Slack DM with what each member of a team merged, opened and reviewed since the last summary. A missed day is never lost."
          done="Done. Check your Slack DMs."
        >
          <SummaryCard
            summary={api.cfg.summary}
            jiraSite={api.cfg.jiraSite}
            onSave={(summary) => api.save({ ...api.cfg, summary })}
          />
        </JobCard>
      </fieldset>

      <div className="space-y-1">
        <button
          type="button"
          onClick={async () => setLog(log === null ? await ReadLog(40).catch(() => "") : null)}
          className="text-[10px] text-tertiary hover:text-muted-foreground"
        >
          {log === null ? "Show log" : "Hide log"}
        </button>
        {log !== null && (
          <pre className="max-h-48 overflow-auto p-2 text-[10px] bg-black/30 rounded-md text-white/60 whitespace-pre-wrap">
            {log || "The log is empty."}
          </pre>
        )}
      </div>

      <p className="text-[10px] text-tertiary">
        Worker config stored at ~/Library/Application Support/koko/worker.json
      </p>
    </div>
  );
}
