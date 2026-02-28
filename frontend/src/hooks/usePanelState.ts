import { useState, useCallback, useEffect } from "react";

const STORAGE_KEY = "koko-panel-order";

export interface PanelState {
  id: string;
  expanded: boolean;
}

const defaultPanels: PanelState[] = [
  { id: "github", expanded: true },
  { id: "slack", expanded: false },
  { id: "summary", expanded: false },
];

function loadOrder(): PanelState[] {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      const ids = JSON.parse(saved) as string[];
      // Merge saved order with defaults (handle new/removed panels)
      const known = new Set(defaultPanels.map((p) => p.id));
      const ordered = ids
        .filter((id) => known.has(id))
        .map((id) => defaultPanels.find((p) => p.id === id)!);
      // Add any new panels not in saved order
      for (const p of defaultPanels) {
        if (!ids.includes(p.id)) ordered.push(p);
      }
      return ordered;
    }
  } catch {
    // ignore
  }
  return defaultPanels;
}

function saveOrder(panels: PanelState[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(panels.map((p) => p.id)));
}

export function usePanelState() {
  const [panels, setPanels] = useState<PanelState[]>(loadOrder);

  useEffect(() => {
    saveOrder(panels);
  }, [panels]);

  const togglePanel = useCallback((id: string) => {
    setPanels((prev) =>
      prev.map((p) => (p.id === id ? { ...p, expanded: !p.expanded } : p)),
    );
  }, []);

  const reorderPanels = useCallback((ids: string[]) => {
    setPanels((prev) => {
      const map = new Map(prev.map((p) => [p.id, p]));
      return ids.map((id) => map.get(id)!).filter(Boolean);
    });
  }, []);

  return { panels, togglePanel, reorderPanels };
}
