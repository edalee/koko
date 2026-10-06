import { useCallback, useEffect, useState } from "react";

export type OverlayModule = "settings";

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
        return;
      }
      // Cmd+, opens and closes Settings, the usual Mac shortcut.
      if (e.key === "," && (e.metaKey || e.ctrlKey) && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        toggleOverlay("settings");
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeOverlay, closeOverlay, toggleOverlay]);

  return { activeOverlay, toggleOverlay, closeOverlay };
}
