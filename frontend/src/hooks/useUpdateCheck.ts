import { useEffect, useState } from "react";
import { CheckForUpdate } from "../../wailsjs/go/main/App";

export interface UpdateInfo {
  available: boolean;
  version: string;
  currentVersion: string;
  url: string;
}

export function useUpdateCheck() {
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    // Check on startup after a short delay
    const timeout = setTimeout(async () => {
      try {
        const info = await CheckForUpdate();
        if (info?.available) {
          setUpdate(info);
        }
      } catch {
        // Network error, silently ignore
      }
    }, 5000);

    // Re-check every 4 hours
    const interval = setInterval(
      async () => {
        try {
          const info = await CheckForUpdate();
          if (info?.available) {
            setUpdate(info);
            setDismissed(false);
          }
        } catch {
          // ignore
        }
      },
      4 * 60 * 60 * 1000,
    );

    return () => {
      clearTimeout(timeout);
      clearInterval(interval);
    };
  }, []);

  return {
    update: dismissed ? null : update,
    dismiss: () => setDismissed(true),
  };
}
