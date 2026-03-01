import { useEffect, useRef } from "react";
import { useSessionTabs } from "./hooks/useSessionTabs";
import { useGitHub } from "./hooks/useGitHub";
import { usePanelState } from "./hooks/usePanelState";
import TitleBar from "./components/TitleBar";
import SessionTabBar from "./components/SessionTabBar";
import TerminalPane from "./components/TerminalPane";
import PanelDock from "./components/PanelDock";
import GitHubPanel from "./components/GitHubPanel";
import SlackPanel from "./components/SlackPanel";
import SummaryPanel from "./components/SummaryPanel";

export default function App() {
  const { tabs, activeTabId, createTab, closeTab, switchTab, handleSessionExit } =
    useSessionTabs();
  const { prs } = useGitHub();
  const { panels, togglePanel, reorderPanels } = usePanelState();
  const initialized = useRef(false);

  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;
      createTab();
    }
  }, [createTab]);

  // Mock slack count
  const slackCount = 6;

  return (
    <div
      className="flex h-screen w-screen flex-col"
      style={{ background: 'linear-gradient(180deg, oklch(0.10 0.02 260), oklch(0.18 0.04 260))' }}
    >
      <TitleBar>
        <SessionTabBar
          tabs={tabs}
          activeTabId={activeTabId}
          onSwitch={switchTab}
          onClose={closeTab}
          onCreate={createTab}
        />
      </TitleBar>

      <div className="flex flex-1 min-h-0">
        {/* Terminal panes */}
        <div className="flex-1 min-w-0 relative p-2">
          {tabs.map((tab) => (
            <div
              key={tab.id}
              className="absolute inset-0"
              style={{ display: tab.id === activeTabId ? "block" : "none" }}
            >
              <TerminalPane
                sessionId={tab.id}
                active={tab.id === activeTabId}
                onExit={() => handleSessionExit(tab.id)}
              />
            </div>
          ))}
          {tabs.length === 0 && (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              <p>No active sessions</p>
            </div>
          )}
        </div>

        {/* Panel dock */}
        <PanelDock
          panels={panels}
          onReorder={reorderPanels}
          renderPanel={(panel) => {
            switch (panel.id) {
              case "github":
                return (
                  <GitHubPanel
                    expanded={panel.expanded}
                    onToggle={() => togglePanel(panel.id)}
                  />
                );
              case "slack":
                return (
                  <SlackPanel
                    expanded={panel.expanded}
                    onToggle={() => togglePanel(panel.id)}
                  />
                );
              case "summary":
                return (
                  <SummaryPanel
                    expanded={panel.expanded}
                    onToggle={() => togglePanel(panel.id)}
                    prCount={prs.length}
                    slackCount={slackCount}
                  />
                );
              default:
                return null;
            }
          }}
        />
      </div>
    </div>
  );
}
