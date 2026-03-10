import { useCallback, useEffect, useState } from "react";

export type OverlayModule = "github" | "slack" | "notifications" | "settings";

export function useOverlay() {
  const [activeOverlay, setActiveOverlay] = useState<OverlayModule | null>(null);

  const toggleOverlay = useCallback((module: OverlayModule) => {
    setActiveOverlay((prev) => (prev === module ? null : module));
  }, []);

  const closeOverlay = useCallback(() => {
    setActiveOverlay(null);
  }, []);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape" && activeOverlay) {
        closeOverlay();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeOverlay, closeOverlay]);

  return { activeOverlay, toggleOverlay, closeOverlay };
}
