import { Check, Eye, EyeOff, Loader2, Zap } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetConfig, SetSlackToken } from "../../wailsjs/go/main/ConfigService";
import { TestConnection } from "../../wailsjs/go/main/SlackService";
import type { SafeWorkingConfig } from "../hooks/useSafeWorking";
import { cn } from "../lib/utils";

interface SettingsPanelProps {
  onTokenSaved?: () => void;
  safeWorkingConfig: SafeWorkingConfig;
  onSafeWorkingChange: (config: SafeWorkingConfig) => void;
}

const BREAK_PRESETS = [
  { label: "90 / 15", work: 90, rest: 15 },
  { label: "60 / 10", work: 60, rest: 10 },
  { label: "45 / 5", work: 45, rest: 5 },
];

export default function SettingsPanel({
  onTokenSaved,
  safeWorkingConfig,
  onSafeWorkingChange,
}: SettingsPanelProps) {
  const [slackToken, setSlackToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<string | null>(null);

  useEffect(() => {
    GetConfig()
      .then((config) => {
        if (config.slackToken) {
          setSlackToken(config.slackToken);
        }
      })
      .catch(() => {});
  }, []);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setSaved(false);
    try {
      await SetSlackToken(slackToken.trim());
      setSaved(true);
      onTokenSaved?.();
      setTimeout(() => setSaved(false), 2000);
    } catch {
      // ignore
    } finally {
      setSaving(false);
    }
  }, [slackToken, onTokenSaved]);

  return (
    <div className="p-4 space-y-6">
      {/* Slack */}
      <div className="space-y-3">
        <div>
          <h4 className="text-sm text-white font-medium">Slack Integration</h4>
          <p className="text-xs text-muted-foreground mt-1">
            Enter a Slack user token (xoxp-...) to see DMs and @mentions.
          </p>
        </div>

        <div className="space-y-2">
          <label htmlFor="slack-token" className="text-xs text-muted-foreground">
            User Token
          </label>
          <div className="relative">
            <input
              id="slack-token"
              type={showToken ? "text" : "password"}
              value={slackToken}
              onChange={(e) => setSlackToken(e.target.value)}
              placeholder="xoxp-..."
              className="w-full px-3 py-2 pr-10 text-sm bg-white/[0.04] border border-white/[0.06] rounded-lg text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors font-mono"
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSave();
              }}
            />
            <button
              type="button"
              onClick={() => setShowToken(!showToken)}
              className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted-foreground hover:text-white transition-colors"
            >
              {showToken ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
            </button>
          </div>
          <p className="text-[10px] text-tertiary">
            Requires scopes: im:history, im:read, users:read
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg transition-colors border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-50"
          >
            {saving ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : saved ? (
              <Check className="size-3.5 text-accent" />
            ) : null}
            <span className="text-white">{saved ? "Saved" : "Save Token"}</span>
          </button>

          <button
            type="button"
            onClick={async () => {
              setTesting(true);
              setTestResult(null);
              try {
                const result = await TestConnection();
                setTestResult(result);
              } catch (err) {
                setTestResult(`Error: ${err}`);
              } finally {
                setTesting(false);
              }
            }}
            disabled={testing || !slackToken.trim()}
            className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg transition-colors border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-50"
          >
            {testing ? <Loader2 className="size-3.5 animate-spin" /> : <Zap className="size-3.5" />}
            <span className="text-white">Test</span>
          </button>
        </div>

        {testResult && (
          <pre
            className={`text-xs whitespace-pre-wrap font-mono p-2 rounded-lg bg-white/[0.04] ${testResult.startsWith("Error") ? "text-red-400" : "text-accent"}`}
          >
            {testResult}
          </pre>
        )}
      </div>

      {/* Safe Working */}
      <div className="space-y-4 pt-3 border-t border-white/[0.06]">
        <div>
          <h4 className="text-sm text-white font-medium">Safe Working</h4>
          <p className="text-xs text-muted-foreground mt-1">
            Set boundaries to protect your wellbeing.
          </p>
        </div>

        {/* Quiet Hours */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label htmlFor="quiet-toggle" className="text-xs text-white/80">
              Quiet Hours
            </label>
            <button
              id="quiet-toggle"
              type="button"
              onClick={() =>
                onSafeWorkingChange({
                  ...safeWorkingConfig,
                  quietHoursEnabled: !safeWorkingConfig.quietHoursEnabled,
                })
              }
              className={cn(
                "w-8 h-[18px] rounded-full transition-colors relative",
                safeWorkingConfig.quietHoursEnabled ? "bg-accent" : "bg-white/[0.12]",
              )}
            >
              <span
                className={cn(
                  "absolute top-[2px] size-[14px] rounded-full bg-white transition-transform",
                  safeWorkingConfig.quietHoursEnabled ? "translate-x-[16px]" : "translate-x-[2px]",
                )}
              />
            </button>
          </div>
          <p className="text-[10px] text-tertiary">Block access during set hours.</p>
          {safeWorkingConfig.quietHoursEnabled && (
            <div className="flex items-center gap-2">
              <input
                type="time"
                value={safeWorkingConfig.quietHoursStart}
                onChange={(e) =>
                  onSafeWorkingChange({ ...safeWorkingConfig, quietHoursStart: e.target.value })
                }
                className="px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40 [color-scheme:dark]"
              />
              <span className="text-xs text-muted-foreground">to</span>
              <input
                type="time"
                value={safeWorkingConfig.quietHoursEnd}
                onChange={(e) =>
                  onSafeWorkingChange({ ...safeWorkingConfig, quietHoursEnd: e.target.value })
                }
                className="px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40 [color-scheme:dark]"
              />
            </div>
          )}
        </div>

        {/* Break Reminders */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label htmlFor="break-toggle" className="text-xs text-white/80">
              Break Reminders
            </label>
            <button
              id="break-toggle"
              type="button"
              onClick={() =>
                onSafeWorkingChange({
                  ...safeWorkingConfig,
                  breakEnabled: !safeWorkingConfig.breakEnabled,
                })
              }
              className={cn(
                "w-8 h-[18px] rounded-full transition-colors relative",
                safeWorkingConfig.breakEnabled ? "bg-accent" : "bg-white/[0.12]",
              )}
            >
              <span
                className={cn(
                  "absolute top-[2px] size-[14px] rounded-full bg-white transition-transform",
                  safeWorkingConfig.breakEnabled ? "translate-x-[16px]" : "translate-x-[2px]",
                )}
              />
            </button>
          </div>
          <p className="text-[10px] text-tertiary">Scheduled breaks at regular intervals.</p>
          {safeWorkingConfig.breakEnabled && (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <div className="flex-1">
                  <label htmlFor="work-mins" className="text-[10px] text-tertiary block mb-1">
                    Work (min)
                  </label>
                  <input
                    id="work-mins"
                    type="number"
                    min={1}
                    value={safeWorkingConfig.workMinutes}
                    onChange={(e) =>
                      onSafeWorkingChange({
                        ...safeWorkingConfig,
                        workMinutes: Math.max(1, Number.parseInt(e.target.value, 10) || 1),
                      })
                    }
                    className="w-full px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40 tabular-nums"
                  />
                </div>
                <div className="flex-1">
                  <label htmlFor="break-mins" className="text-[10px] text-tertiary block mb-1">
                    Break (min)
                  </label>
                  <input
                    id="break-mins"
                    type="number"
                    min={1}
                    value={safeWorkingConfig.breakMinutes}
                    onChange={(e) =>
                      onSafeWorkingChange({
                        ...safeWorkingConfig,
                        breakMinutes: Math.max(1, Number.parseInt(e.target.value, 10) || 1),
                      })
                    }
                    className="w-full px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40 tabular-nums"
                  />
                </div>
              </div>
              <div className="flex items-center gap-1.5">
                {BREAK_PRESETS.map((p) => (
                  <button
                    key={p.label}
                    type="button"
                    onClick={() =>
                      onSafeWorkingChange({
                        ...safeWorkingConfig,
                        workMinutes: p.work,
                        breakMinutes: p.rest,
                      })
                    }
                    className={cn(
                      "px-2 py-0.5 text-[10px] rounded transition-colors",
                      safeWorkingConfig.workMinutes === p.work &&
                        safeWorkingConfig.breakMinutes === p.rest
                        ? "bg-white/[0.08] text-white"
                        : "text-tertiary hover:text-muted-foreground hover:bg-white/[0.04]",
                    )}
                  >
                    {p.label}
                  </button>
                ))}
                <span className="text-[10px] text-tertiary ml-1">work / break</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Config file location */}
      <div className="pt-3 border-t border-white/[0.06]">
        <p className="text-[10px] text-tertiary">Config stored at ~/.config/koko/config.json</p>
      </div>
    </div>
  );
}
