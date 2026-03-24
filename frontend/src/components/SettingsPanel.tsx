import { Check, Copy, Eye, EyeOff, Loader2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetConfig, SaveConfig } from "../../wailsjs/go/main/ConfigService";
import type { SafeWorkingConfig } from "../hooks/useSafeWorking";
import { cn } from "../lib/utils";

interface SettingsPanelProps {
  safeWorkingConfig: SafeWorkingConfig;
  onSafeWorkingChange: (config: SafeWorkingConfig) => void;
}

const BREAK_PRESETS = [
  { label: "90 / 15", work: 90, rest: 15 },
  { label: "60 / 10", work: 60, rest: 10 },
  { label: "45 / 5", work: 45, rest: 5 },
];

export default function SettingsPanel({
  safeWorkingConfig,
  onSafeWorkingChange,
}: SettingsPanelProps) {
  const [slackToken, setSlackToken] = useState("");
  const [slackOwnerID, setSlackOwnerID] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [apiEnabled, setApiEnabled] = useState(true);
  const [apiPort, setApiPort] = useState(19876);
  const [apiKey, setApiKey] = useState("");
  const [keyCopied, setKeyCopied] = useState(false);

  useEffect(() => {
    GetConfig()
      .then((config) => {
        if (config.slackToken) {
          setSlackToken(config.slackToken);
        }
        if (config.slackOwnerId) {
          setSlackOwnerID(config.slackOwnerId);
        }
        setApiEnabled(config.apiEnabled ?? true);
        setApiPort(config.apiPort || 19876);
        setApiKey(config.apiKey || "");
      })
      .catch(() => {});
  }, []);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setSaved(false);
    try {
      const config = await GetConfig();
      config.slackToken = slackToken.trim();
      config.slackOwnerId = slackOwnerID.trim();
      await SaveConfig(config);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch {
      // ignore
    } finally {
      setSaving(false);
    }
  }, [slackToken]);

  return (
    <div className="p-4 space-y-6">
      {/* Slack Bot */}
      <div className="space-y-3">
        <div>
          <h4 className="text-sm text-white font-medium">Slack Bot</h4>
          <p className="text-xs text-muted-foreground mt-1">
            DM the bot to control Koko sessions from Slack.
          </p>
        </div>

        <div className="space-y-2">
          <label htmlFor="slack-token" className="text-xs text-muted-foreground">
            Bot Token
          </label>
          <div className="relative">
            <input
              id="slack-token"
              type={showToken ? "text" : "password"}
              value={slackToken}
              onChange={(e) => setSlackToken(e.target.value)}
              placeholder="xoxb-..."
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
            Bot token scopes: im:history, im:read, chat:write
          </p>
        </div>

        <div className="space-y-2">
          <label htmlFor="slack-owner" className="text-xs text-muted-foreground">
            Your Slack Member ID
          </label>
          <input
            id="slack-owner"
            type="text"
            value={slackOwnerID}
            onChange={(e) => setSlackOwnerID(e.target.value)}
            placeholder="U02F4AZV2"
            className="w-full px-3 py-2 text-sm bg-white/[0.04] border border-white/[0.06] rounded-lg text-white placeholder:text-tertiary outline-none focus:border-accent/40 transition-colors font-mono"
          />
          <p className="text-[10px] text-tertiary">
            Only respond to DMs from this user. Find in Slack profile → ⋯ → Copy member ID.
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
        </div>
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

      {/* Remote API */}
      <div className="space-y-3 pt-3 border-t border-white/[0.06]">
        <div>
          <h4 className="text-sm text-white font-medium">Remote API</h4>
          <p className="text-xs text-muted-foreground mt-1">
            HTTP/WebSocket API for MCP, Slack commands, and CLI access.
          </p>
        </div>

        <div className="flex items-center justify-between">
          <label htmlFor="api-toggle" className="text-xs text-white/80">
            Enabled
          </label>
          <button
            id="api-toggle"
            type="button"
            onClick={async () => {
              const next = !apiEnabled;
              setApiEnabled(next);
              const config = await GetConfig();
              config.apiEnabled = next;
              await SaveConfig(config);
            }}
            className={cn(
              "w-8 h-[18px] rounded-full transition-colors relative",
              apiEnabled ? "bg-accent" : "bg-white/[0.12]",
            )}
          >
            <span
              className={cn(
                "absolute top-[2px] size-[14px] rounded-full bg-white transition-transform",
                apiEnabled ? "translate-x-[16px]" : "translate-x-[2px]",
              )}
            />
          </button>
        </div>

        {apiEnabled && (
          <>
            <div className="space-y-1">
              <label htmlFor="api-port" className="text-[10px] text-tertiary">
                Port
              </label>
              <input
                id="api-port"
                type="number"
                value={apiPort}
                onChange={async (e) => {
                  const port = Number.parseInt(e.target.value, 10) || 19876;
                  setApiPort(port);
                  const config = await GetConfig();
                  config.apiPort = port;
                  await SaveConfig(config);
                }}
                className="w-full px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40 tabular-nums font-mono"
              />
            </div>

            <div className="space-y-1">
              <label htmlFor="api-key-display" className="text-[10px] text-tertiary">
                API Key
              </label>
              <div className="flex items-center gap-2">
                <code
                  id="api-key-display"
                  className="flex-1 px-2 py-1 text-[10px] bg-white/[0.04] border border-white/[0.06] rounded-md text-white/60 font-mono truncate"
                >
                  {apiKey ? `${apiKey.slice(0, 8)}...${apiKey.slice(-8)}` : "generating..."}
                </code>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(apiKey);
                    setKeyCopied(true);
                    setTimeout(() => setKeyCopied(false), 2000);
                  }}
                  className="p-1.5 rounded-md border border-white/[0.08] hover:bg-white/[0.06] transition-colors"
                  title="Copy API key"
                >
                  {keyCopied ? (
                    <Check className="size-3 text-accent" />
                  ) : (
                    <Copy className="size-3 text-muted-foreground" />
                  )}
                </button>
              </div>
              <p className="text-[10px] text-tertiary">
                Used by MCP server and koko-cli. Auto-generated on first run.
              </p>
            </div>
          </>
        )}
      </div>

      {/* Config file location */}
      <div className="pt-3 border-t border-white/[0.06]">
        <p className="text-[10px] text-tertiary">Config stored at ~/.config/koko/config.json</p>
      </div>
    </div>
  );
}
