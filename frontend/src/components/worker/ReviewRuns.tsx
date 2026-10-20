import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { cn } from "../../lib/utils";
import RepoList from "../RepoList";
import Toggle from "../Toggle";
import { inputClass, type RunRecord } from "./config";
import JobSchedule from "./JobSchedule";
import RunControls from "./RunControls";
import { newRun, prOnlyIn, type ReviewRun } from "./runs";
import { Choice, Field } from "./ui";

const days = (value: string) => Number.parseInt(value, 10) || 0;

// ReviewRuns edits the runs of the reviewer command. Each run is the job
// "review:<name>", with its own arguments, days and times, its PR scopes or
// its repos, and its own Test and Run now. onEdit keeps a change without
// saving it, for text as you type. onSave saves the runs.
export default function ReviewRuns({
  runs,
  command,
  jobs,
  running,
  setRunning,
  onRan,
  onEdit,
  onSave,
}: {
  runs: ReviewRun[];
  command: string;
  jobs?: Record<string, { lastRun?: RunRecord }>;
  running: string | null;
  setRunning: (running: string | null) => void;
  onRan: () => void;
  onEdit: (runs: ReviewRun[]) => void;
  onSave: (runs: ReviewRun[]) => void;
}) {
  const change = (i: number, patch: Partial<ReviewRun>) =>
    runs.map((r, j) => (j === i ? { ...r, ...patch } : r));
  const edit = (i: number, patch: Partial<ReviewRun>) => onEdit(change(i, patch));
  const save = (i: number, patch: Partial<ReviewRun>) => onSave(change(i, patch));

  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-2">
        <p className="text-[10px] text-tertiary leading-relaxed">
          Each run adds its own arguments to the command, on its own days and times. A PRs run
          reviews each PR once. A repos run runs once per repo, in a clean checkout of its default
          branch, for a tool that changes repos.
        </p>
        <button
          type="button"
          onClick={() => onSave([...runs, newRun(runs)])}
          className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] shrink-0"
          title="Add a run"
        >
          <Plus className="size-3" />
          <span className="text-white">Add run</span>
        </button>
      </div>

      {runs.map((run, i) => {
        const job = `review:${run.name}`;
        const blocked = prOnlyIn(command, run);
        return (
          <section
            // The name alone can repeat while a second run is renamed.
            key={`${run.name}-${i}`}
            aria-label={`Run ${run.name}`}
            className="space-y-2.5 p-2.5 rounded-md bg-white/[0.02] border border-white/[0.05]"
          >
            <div className="flex items-center gap-2">
              <NameInput name={run.name} onSave={(name) => save(i, { name })} />
              <input
                type="text"
                aria-label="Extra arguments"
                value={run.args}
                onChange={(e) => edit(i, { args: e.target.value })}
                onBlur={() => onSave(runs)}
                placeholder="Extra arguments, for example --update-deps"
                className={cn(inputClass, "flex-1 min-w-0 font-mono placeholder:text-tertiary")}
              />
              <Toggle
                id={`run-${i}`}
                label={`Run ${run.name} on`}
                on={run.enabled}
                onClick={() => save(i, { enabled: !run.enabled })}
              />
              <button
                type="button"
                disabled={runs.length === 1}
                onClick={() => onSave(runs.filter((_, j) => j !== i))}
                className="opacity-50 hover:opacity-100 hover:text-error disabled:opacity-20 disabled:hover:text-inherit"
                title={runs.length === 1 ? "Keep at least one run" : "Remove this run"}
              >
                <Trash2 className="size-3" />
              </button>
            </div>

            <JobSchedule
              jobKey={job}
              job={run}
              multi
              onChange={({ enabled, times, days }) => save(i, { enabled, times, days })}
            />

            <Field label="Works on">
              <Choice
                value={run.target}
                options={[
                  { value: "prs", label: "PRs" },
                  { value: "repos", label: "Repos" },
                ]}
                onPick={(target) => save(i, { target })}
              />
            </Field>

            {run.target === "prs" ? (
              <>
                <Field
                  label="My PRs"
                  htmlFor={`run-${i}-mine`}
                  help="Your open PRs, outside archived repos."
                >
                  <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                    <Toggle
                      id={`run-${i}-mine`}
                      on={run.mine.enabled}
                      onClick={() => save(i, { mine: { ...run.mine, enabled: !run.mine.enabled } })}
                    />
                    {run.mine.enabled && (
                      <>
                        <span>opened in the last</span>
                        <input
                          type="number"
                          min={1}
                          aria-label="My PRs age in days"
                          value={run.mine.maxAgeDays}
                          onChange={(e) =>
                            save(i, { mine: { ...run.mine, maxAgeDays: days(e.target.value) } })
                          }
                          className={cn(inputClass, "w-12 tabular-nums")}
                        />
                        <span>days</span>
                      </>
                    )}
                  </div>
                </Field>
                <Field
                  label="Team PRs"
                  htmlFor={`run-${i}-team`}
                  help="PRs that ask for your review, and every open PR in these repos. The stand-up uses the team of the first PRs run that is on."
                >
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                      <Toggle
                        id={`run-${i}-team`}
                        on={run.team.enabled}
                        onClick={() =>
                          save(i, { team: { ...run.team, enabled: !run.team.enabled } })
                        }
                      />
                      {run.team.enabled && (
                        <>
                          <span>opened in the last</span>
                          <input
                            type="number"
                            min={1}
                            aria-label="Team PRs age in days"
                            value={run.team.maxAgeDays}
                            onChange={(e) =>
                              save(i, { team: { ...run.team, maxAgeDays: days(e.target.value) } })
                            }
                            className={cn(inputClass, "w-12 tabular-nums")}
                          />
                          <span>days</span>
                        </>
                      )}
                    </div>
                    {/* The team box shows with Team PRs off too, because the stand-up uses the team. */}
                    <div className="flex items-center gap-2 text-[10px] text-tertiary">
                      <span className="shrink-0">Opened by</span>
                      <input
                        type="text"
                        aria-label="Team"
                        value={run.team.team}
                        onChange={(e) => edit(i, { team: { ...run.team, team: e.target.value } })}
                        onBlur={() => onSave(runs)}
                        placeholder="org/team-slug"
                        className={cn(
                          inputClass,
                          "flex-1 min-w-0 font-mono placeholder:text-tertiary",
                        )}
                      />
                    </div>
                    {run.team.enabled && (
                      <RepoList
                        repos={run.team.repos}
                        org={run.team.team.split("/")[0].trim()}
                        empty="No repos yet. Only PRs that ask for your review are reviewed."
                        onChange={(repos) => save(i, { team: { ...run.team, repos } })}
                      />
                    )}
                  </div>
                </Field>
              </>
            ) : (
              <>
                <Field
                  label="Repos"
                  help="The command runs once per repo, one after another. Exit 0 is done, 2 is nothing to do. One Slack DM lists each repo, with the last 5 links in its output. Switching the worker or the run off stops a scheduled run before its next repo. Test prints the commands and runs nothing."
                >
                  <RepoList
                    repos={run.repos}
                    org=""
                    empty="No repos yet. Add owner/repo names."
                    onChange={(repos) => save(i, { repos })}
                  />
                </Field>
                <Field label="Time limit" htmlFor={`run-${i}-timeout`}>
                  <div className="flex items-center gap-2 text-[10px] text-tertiary">
                    <input
                      id={`run-${i}-timeout`}
                      type="number"
                      min={1}
                      value={run.timeoutMinutes || 60}
                      onChange={(e) => save(i, { timeoutMinutes: days(e.target.value) })}
                      className={cn(inputClass, "w-14 tabular-nums")}
                    />
                    <span>minutes per repo</span>
                  </div>
                </Field>
                {blocked.length > 0 && (
                  <p className="text-[10px] text-warning">
                    {`A repos run has no PR, so the worker refuses a command with ${blocked.join(", ")}.`}
                  </p>
                )}
              </>
            )}

            <RunControls
              // A new key clears the output when a removed run's place goes to another run.
              key={job}
              jobKey={job}
              last={jobs?.[job]?.lastRun}
              done={
                run.target === "prs"
                  ? "Done. The reviews are on the PRs."
                  : "Done. Check your Slack DMs."
              }
              running={running}
              setRunning={setRunning}
              onRan={onRan}
            />
          </section>
        );
      })}
    </div>
  );
}

// NameInput edits a run's name as a draft and saves it when you leave the box,
// so the run's card keeps its place, and focus, while you type.
function NameInput({ name, onSave }: { name: string; onSave: (name: string) => void }) {
  const [draft, setDraft] = useState(name);
  return (
    <input
      type="text"
      aria-label="Run name"
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => draft !== name && onSave(draft)}
      placeholder="daily"
      className={cn(inputClass, "w-28 font-mono placeholder:text-tertiary")}
    />
  );
}
