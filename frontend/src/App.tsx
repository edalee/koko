import { useEffect, useRef, useState } from "react";
import RightSidebar from "./components/RightSidebar";
import SessionSidebar from "./components/SessionSidebar";
import TerminalPane from "./components/TerminalPane";
import Toolbar from "./components/Toolbar";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "./components/ui/resizable";
import { useSessionTabs } from "./hooks/useSessionTabs";

export default function App() {
  const { tabs, activeTabId, createTab, closeTab, switchTab, handleSessionExit } = useSessionTabs();
  const [isRightSidebarCollapsed, setIsRightSidebarCollapsed] = useState(true);
  const [isLeftSidebarCollapsed, setIsLeftSidebarCollapsed] = useState(false);
  const initialized = useRef(false);

  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;
      createTab();
    }
  }, [createTab]);

  return (
    <div className="size-full flex flex-col bg-base">
      <Toolbar />

      <div className="flex-1 flex overflow-hidden">
        <ResizablePanelGroup orientation="horizontal" className="flex-1">
          <ResizablePanel
            defaultSize="15"
            minSize={isLeftSidebarCollapsed ? "4" : "12"}
            maxSize={isLeftSidebarCollapsed ? "4" : "25"}
          >
            <SessionSidebar
              sessions={tabs}
              activeSessionId={activeTabId}
              onSessionSelect={switchTab}
              onNewSession={createTab}
              onDeleteSession={closeTab}
              isCollapsed={isLeftSidebarCollapsed}
              onToggleCollapse={() => setIsLeftSidebarCollapsed(!isLeftSidebarCollapsed)}
            />
          </ResizablePanel>
          <ResizableHandle />
          <ResizablePanel defaultSize="66">
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
          <ResizableHandle />
          <ResizablePanel
            defaultSize="19"
            minSize={isRightSidebarCollapsed ? "4" : "15"}
            maxSize={isRightSidebarCollapsed ? "4" : "35"}
          >
            <RightSidebar
              isCollapsed={isRightSidebarCollapsed}
              onToggleCollapse={() => setIsRightSidebarCollapsed(!isRightSidebarCollapsed)}
            />
          </ResizablePanel>
        </ResizablePanelGroup>
      </div>
    </div>
  );
}
