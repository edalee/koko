import { FlaskConical, Loader2, Play } from "lucide-react";
import { useState } from "react";
import { RunNow } from "../../../wailsjs/go/main/WorkerService";
import { cn } from "../../lib/utils";
import { formatWhen, type RunRecord, STATUS_LABELS } from "./config";

// LastRun is a job's last run and how it went.
export function LastRun({ last }: { last?: RunRecord }) {
  if (!last) return <>Not run yet</>;
  return (
    <>
      Last run {formatWhen(last.at)}:{" "}
      <span
        className={cn(
          last.status === "ok" && "text-success",
          last.status === "failed" && "text-error",
          (last.status === "retry" || last.status === "offline") && "text-warning",
        )}
      >
        {STATUS_LABELS[last.status] ?? last.status}
      </span>
    </>
  );
}

// RunControls is a job's Test and Run now buttons, its last run and its own
// output. running is shared by every card, so one job runs at a time.
export default function RunControls({
  jobKey,
  last,
  done,
  running,
  setRunning,
  onRan,
}: {
  jobKey: string;
  last?: RunRecord;
  done: string;
  running: string | null;
  setRunning: (running: string | null) => void;
  onRan: () => void;
}) {
  const [output, setOutput] = useState<string | null>(null);

  const run = async (test: boolean) => {
    setRunning(jobKey + (test ? ":test" : ""));
    setOutput(null);
    try {
      const text = await RunNow(jobKey, test);
      setOutput(text.trim() || (test ? "(no output)" : done));
      onRan();
    } catch (e) {
      setOutput(String(e));
    } finally {
      setRunning(null);
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-[10px] text-tertiary truncate" title={last?.message}>
          <LastRun last={last} />
        </p>
        <div className="flex items-center gap-1 shrink-0">
          <button
            type="button"
            disabled={running !== null}
            onClick={() => run(true)}
            className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40"
            title="Print the result here. Sends and books nothing."
          >
            {running === `${jobKey}:test` ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <FlaskConical className="size-3" />
            )}
            <span className="text-white">Test</span>
          </button>
          <button
            type="button"
            disabled={running !== null}
            onClick={() => run(false)}
            className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40"
            title="Run it for real now"
          >
            {running === jobKey ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <Play className="size-3" />
            )}
            <span className="text-white">Run now</span>
          </button>
        </div>
      </div>
      {output !== null && (
        <pre className="max-h-64 overflow-auto p-2 text-[10px] leading-relaxed bg-black/30 rounded-md text-white/75 whitespace-pre-wrap">
          {output}
        </pre>
      )}
    </div>
  );
}
