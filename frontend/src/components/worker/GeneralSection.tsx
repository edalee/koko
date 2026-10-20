import { Check, CircleAlert, Eye, EyeOff, Loader2 } from "lucide-react";
import { useState } from "react";
import { Check as CheckConnections } from "../../../wailsjs/go/main/WorkerService";
import { cn } from "../../lib/utils";
import Toggle from "../Toggle";
import { type CheckResult, CONNECTION_NAMES, formatWhen, inputClass } from "./config";
import { Field, Group } from "./ui";
import type { WorkerConfigApi } from "./useWorkerConfig";

// GeneralSection holds the settings that no one job card owns: the master switch,
// Slack, the time zone, calendar and Jira site, lunch, and the connection checks.
export default function GeneralSection({ api }: { api: WorkerConfigApi }) {
  const { cfg, setCfg, status, error, setError, save, saveField, switching, setEnabled } = api;
  const { lunchDraft, setLunchDraft, lunchError, saveLunchTimes, toggleLunch } = api;
  const [showToken, setShowToken] = useState(false);
  const [checks, setChecks] = useState<CheckResult[] | null>(null);
  const [checking, setChecking] = useState(false);

  const handleCheck = async () => {
    setChecking(true);
    try {
      setChecks(JSON.parse(await CheckConnections("")));
    } catch (e) {
      setError(String(e));
    } finally {
      setChecking(false);
    }
  };

  const agentState = !status
    ? "unknown"
    : status.configError
      ? "config error"
      : status.agentLoaded
        ? "running"
        : "stopped";

  return (
    <section aria-labelledby="worker-general" className="space-y-5">
      <div className="space-y-2">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h4 id="worker-general" className="text-sm text-white font-medium">
              Worker
            </h4>
            <p className="text-xs text-muted-foreground mt-1">
              Stand-up, focus time, PR reviews and a team summary on the days you choose. Runs in
              the background, even with Koko closed.
            </p>
          </div>
          {switching ? (
            <Loader2 className="size-4 animate-spin text-muted-foreground shrink-0" />
          ) : (
            <Toggle
              id="worker-toggle"
              label="Worker on"
              on={cfg.enabled}
              onClick={() => setEnabled(!cfg.enabled)}
            />
          )}
        </div>
        <p className="text-[10px] text-tertiary">
          Agent:{" "}
          <span className={cn(agentState === "running" ? "text-success" : "text-white/70")}>
            {agentState}
          </span>
          {status?.nextRun && <> · next run {formatWhen(status.nextRun)}</>}
          {status?.wakeBooked && <> · Mac wakes {formatWhen(status.wakeBooked)}</>}
        </p>
        {status?.configError && <p className="text-[10px] text-error">{status.configError}</p>}
        {error && <p className="text-[10px] text-error">{error}</p>}
      </div>

      {/* Edits wait while the master switch runs, because it writes worker.json too. */}
      <fieldset disabled={switching} className="min-w-0 space-y-5 disabled:opacity-60">
        <Group title="Slack">
          <Field label="Bot token" htmlFor="slack-token" help="The bot needs the chat:write scope.">
            <div className="relative">
              <input
                id="slack-token"
                type={showToken ? "text" : "password"}
                value={cfg.slack.botToken}
                onChange={(e) =>
                  setCfg({ ...cfg, slack: { ...cfg.slack, botToken: e.target.value } })
                }
                onBlur={() => save(cfg)}
                placeholder="xoxb-..."
                className={cn(inputClass, "w-full pr-9 font-mono placeholder:text-tertiary")}
              />
              <button
                type="button"
                onClick={() => setShowToken(!showToken)}
                aria-label={showToken ? "Hide the token" : "Show the token"}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted-foreground hover:text-white transition-colors"
              >
                {showToken ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
              </button>
            </div>
          </Field>
          <Field label="Member ID" htmlFor="slack-user" help="Your Slack member ID. DMs go to you.">
            <input
              id="slack-user"
              type="text"
              value={cfg.slack.userId}
              onChange={(e) => setCfg({ ...cfg, slack: { ...cfg.slack, userId: e.target.value } })}
              onBlur={() => save(cfg)}
              placeholder="U0123456789"
              className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
            />
          </Field>
        </Group>

        <Group title="Calendar and Jira">
          {(
            [
              [
                "timeZone",
                "Time zone",
                "Europe/London",
                "An IANA name. Job times are in this zone.",
              ],
              ["calendarId", "Calendar", "primary", "A Google Calendar ID."],
              [
                "jiraSite",
                "Jira site",
                "acme.atlassian.net",
                "Leave it empty and the stand-up reads no tickets.",
              ],
            ] as const
          ).map(([key, label, placeholder, help]) => (
            <Field key={key} label={label} htmlFor={`worker-${key}`} help={help}>
              <input
                id={`worker-${key}`}
                type="text"
                value={cfg[key]}
                onChange={(e) => setCfg({ ...cfg, [key]: e.target.value })}
                onBlur={() => saveField(key)}
                placeholder={placeholder}
                className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
              />
            </Field>
          ))}
        </Group>

        <Group title="Lunch">
          <Field
            label="Block lunch"
            htmlFor="lunch-toggle"
            help="Focus time books around lunch. Lunch is not added to your calendar."
          >
            <div className="flex flex-wrap items-center gap-2">
              <Toggle id="lunch-toggle" on={cfg.lunch.enabled} onClick={toggleLunch} />
              {cfg.lunch.enabled && (
                <fieldset
                  aria-label="Lunch times"
                  className="flex items-center gap-2 min-w-0 text-[10px] text-tertiary"
                  onBlur={(e) => {
                    // Moving from one time box to the other is not leaving the pair.
                    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) saveLunchTimes();
                  }}
                >
                  <input
                    type="time"
                    aria-label="Lunch start"
                    value={lunchDraft.start}
                    onChange={(e) => setLunchDraft({ ...lunchDraft, start: e.target.value })}
                    className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
                  />
                  <span>to</span>
                  <input
                    type="time"
                    aria-label="Lunch end"
                    value={lunchDraft.end}
                    onChange={(e) => setLunchDraft({ ...lunchDraft, end: e.target.value })}
                    className={cn(inputClass, "[color-scheme:dark] tabular-nums")}
                  />
                </fieldset>
              )}
            </div>
            {cfg.lunch.enabled && lunchError && (
              <p className="text-[10px] text-error">{lunchError} It is not saved yet.</p>
            )}
          </Field>
        </Group>
      </fieldset>

      <Group
        title="Connections"
        aside={
          <button
            type="button"
            onClick={handleCheck}
            disabled={checking}
            className="flex items-center gap-1.5 px-2.5 py-1 text-[10px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40"
          >
            {checking && <Loader2 className="size-3 animate-spin" />}
            <span className="text-white">{checking ? "Checking…" : "Check all"}</span>
          </button>
        }
      >
        {checks?.map((c) => (
          <div key={c.name} className="space-y-0.5">
            <div className="flex items-center gap-1.5 text-[11px]">
              {c.ok ? (
                <Check className="size-3 text-success shrink-0" />
              ) : (
                <CircleAlert className="size-3 text-error shrink-0" />
              )}
              <span className="text-white/85">{CONNECTION_NAMES[c.name] ?? c.name}</span>
              <span className="text-tertiary truncate">{c.detail}</span>
            </div>
            {c.fix && (
              <p className="pl-[18px] text-[10px] text-muted-foreground whitespace-pre-wrap font-mono select-text">
                {c.fix}
              </p>
            )}
          </div>
        ))}
        {!checks && !checking && (
          <p className="text-[10px] text-tertiary">
            Checks Slack, GitHub, Jira, Google Calendar, the reviewer, the summary team and Mac
            wake.
          </p>
        )}
      </Group>
    </section>
  );
}
