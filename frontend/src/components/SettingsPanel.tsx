import { Check, Eye, EyeOff, Loader2, Zap } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetConfig, SetSlackToken } from "../../wailsjs/go/main/ConfigService";
import { DebugFetch, TestConnection } from "../../wailsjs/go/main/SlackService";

interface SettingsPanelProps {
  onTokenSaved?: () => void;
}

export default function SettingsPanel({ onTokenSaved }: SettingsPanelProps) {
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
            Requires scopes: im:history, im:read, search:read, users:read
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

      {/* Debug */}
      <div className="space-y-2">
        <button
          type="button"
          onClick={async () => {
            setTestResult(null);
            setTesting(true);
            try {
              const debug = await DebugFetch();
              setTestResult(debug);
            } catch (err) {
              setTestResult(`Error: ${err}`);
            } finally {
              setTesting(false);
            }
          }}
          disabled={testing || !slackToken.trim()}
          className="text-xs text-muted-foreground hover:text-white transition-colors"
        >
          Debug API responses
        </button>
      </div>

      {/* Config file location */}
      <div className="pt-3 border-t border-white/[0.06]">
        <p className="text-[10px] text-tertiary">Config stored at ~/.config/koko/config.json</p>
      </div>
    </div>
  );
}
