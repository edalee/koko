import { Check } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetPRPanel, SetPRPanel } from "../../wailsjs/go/main/ConfigService";
import { GetConfig as GetWorkerConfig } from "../../wailsjs/go/main/WorkerService";
import { main } from "../../wailsjs/go/models";
import { cn } from "../lib/utils";
import RepoList from "./RepoList";
import Toggle from "./Toggle";

interface Scope {
  enabled: boolean;
  maxAgeDays: number;
}

export interface PRPanel {
  mine: Scope;
  team: Scope & { team: string; repos: string[] };
}

const DEFAULTS: PRPanel = {
  mine: { enabled: true, maxAgeDays: 30 },
  team: { enabled: true, maxAgeDays: 14, team: "", repos: [] },
};

const inputClass =
  "px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40";

// fromWorker is the review worker's scopes in worker.json, or null if it has
// none. The PR panel's settings have the same shape (plan 032).
export function fromWorker(raw: string): PRPanel | null {
  const r = (JSON.parse(raw || "{}") as { reviewer?: Partial<PRPanel> }).reviewer;
  if (!r?.mine && !r?.team) return null;
  return {
    mine: { ...DEFAULTS.mine, ...r.mine },
    team: { ...DEFAULTS.team, ...r.team, repos: r.team?.repos ?? [] },
  };
}

// PRPanelSettings picks which PRs the PR panel shows: yours, and your team's,
// set the same way as the review worker's.
export default function PRPanelSettings({ onChange }: { onChange?: () => void }) {
  const [cfg, setCfg] = useState<PRPanel>(DEFAULTS);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [worker, setWorker] = useState<PRPanel | null>(null);

  useEffect(() => {
    GetPRPanel()
      .then((p) => setCfg({ mine: p.mine, team: { ...p.team, repos: p.team.repos ?? [] } }))
      .catch((e) => setError(String(e)));
    GetWorkerConfig()
      .then((raw) => setWorker(fromWorker(raw)))
      .catch(() => setWorker(null));
  }, []);

  const save = useCallback(
    async (next: PRPanel) => {
      setCfg(next);
      setError(null);
      try {
        await SetPRPanel(main.PRPanelConfig.createFrom(next));
        setSaved(true);
        setTimeout(() => setSaved(false), 1500);
        onChange?.();
      } catch (e) {
        setError(String(e));
      }
    },
    [onChange],
  );

  const days = (value: string) => Number.parseInt(value, 10) || 0;
  const org = cfg.team.team.split("/")[0].trim();

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-xs text-white/80">PRs in the PR panel</p>
        {saved && <Check className="size-3 text-accent" />}
      </div>
      {worker && (
        <button
          type="button"
          onClick={() => save(worker)}
          className="px-2.5 py-1 text-[11px] rounded-md border border-white/[0.08] hover:bg-white/[0.06] text-white"
        >
          Use the review worker's settings
        </button>
      )}

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-2">
          <div>
            <label htmlFor="prs-mine" className="text-xs text-white/80">
              My PRs
            </label>
            <p className="text-[10px] text-tertiary">Your open PRs, drafts included.</p>
          </div>
          <Toggle
            id="prs-mine"
            on={cfg.mine.enabled}
            onClick={() => save({ ...cfg, mine: { ...cfg.mine, enabled: !cfg.mine.enabled } })}
          />
        </div>
        {cfg.mine.enabled && (
          <div className="flex items-center gap-2 text-[10px] text-tertiary">
            <span>Opened in the last</span>
            <input
              type="number"
              min={1}
              value={cfg.mine.maxAgeDays}
              onChange={(e) =>
                save({ ...cfg, mine: { ...cfg.mine, maxAgeDays: days(e.target.value) } })
              }
              className={cn(inputClass, "w-12 tabular-nums")}
            />
            <span>days</span>
          </div>
        )}
      </div>

      <div className="space-y-2.5 pt-3 border-t border-white/[0.05]">
        <div className="flex items-center justify-between gap-2">
          <div>
            <label htmlFor="prs-team" className="text-xs text-white/80">
              Team PRs
            </label>
            <p className="text-[10px] text-tertiary">
              PRs that ask for your review, and every open PR in the repos below. Not bots' PRs or
              drafts.
            </p>
          </div>
          <Toggle
            id="prs-team"
            on={cfg.team.enabled}
            onClick={() => save({ ...cfg, team: { ...cfg.team, enabled: !cfg.team.enabled } })}
          />
        </div>
        {cfg.team.enabled && (
          <>
            <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
              <span>Opened by</span>
              <input
                type="text"
                value={cfg.team.team}
                onChange={(e) => setCfg({ ...cfg, team: { ...cfg.team, team: e.target.value } })}
                onBlur={() => save(cfg)}
                placeholder="org/team-slug, or anyone"
                className={cn(inputClass, "flex-1 min-w-40 font-mono placeholder:text-tertiary")}
              />
              <span>in the last</span>
              <input
                type="number"
                min={1}
                value={cfg.team.maxAgeDays}
                onChange={(e) =>
                  save({ ...cfg, team: { ...cfg.team, maxAgeDays: days(e.target.value) } })
                }
                className={cn(inputClass, "w-12 tabular-nums")}
              />
              <span>days</span>
            </div>
            <RepoList
              repos={cfg.team.repos}
              org={org}
              empty="No repos yet. Only PRs that ask for your review show."
              onChange={(repos) => save({ ...cfg, team: { ...cfg.team, repos } })}
            />
          </>
        )}
      </div>
      {error && <p className="text-[10px] text-error">{error}</p>}
    </div>
  );
}
