import { useEffect, useRef, useState } from "react";
import { useSessionTabs } from "./hooks/useSessionTabs";
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from "./components/ui/resizable";
import Toolbar from "./components/Toolbar";
import SessionTabs from "./components/SessionTabs";
import SessionSidebar from "./components/SessionSidebar";
import TerminalPane from "./components/TerminalPane";
import RightSidebar from "./components/RightSidebar";

export default function App() {
  const { tabs, activeTabId, createTab, closeTab, switchTab, handleSessionExit } =
    useSessionTabs();
  const [isRightSidebarOpen, setIsRightSidebarOpen] = useState(false);
  const initialized = useRef(false);

  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;
      createTab();
    }
  }, [createTab]);

  return (
    <div className="size-full flex flex-col bg-base">
      <Toolbar
        onToggleSidebar={() => setIsRightSidebarOpen(!isRightSidebarOpen)}
        isSidebarOpen={isRightSidebarOpen}
      />
      <SessionTabs
        sessions={tabs}
        activeSessionId={activeTabId}
        onSessionSelect={switchTab}
        onSessionClose={closeTab}
      />

      <div className="flex-1 flex overflow-hidden">
        <ResizablePanelGroup orientation="horizontal" className="flex-1">
          <ResizablePanel defaultSize="15" minSize="12" maxSize="25">
            <SessionSidebar
              sessions={tabs}
              activeSessionId={activeTabId}
              onSessionSelect={switchTab}
              onNewSession={createTab}
              onDeleteSession={closeTab}
            />
          </ResizablePanel>
          <ResizableHandle />
          <ResizablePanel defaultSize="85">
            <div className="h-full relative">
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
          </ResizablePanel>
        </ResizablePanelGroup>
        {isRightSidebarOpen && (
          <div className="w-[400px] border-l border-border shrink-0">
            <RightSidebar onClose={() => setIsRightSidebarOpen(false)} />
          </div>
        )}
      </div>
    </div>
  );
}
