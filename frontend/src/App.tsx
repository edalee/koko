import { GitPullRequest, Mail, MessageSquare } from "lucide-react";
import { useCallback, useState } from "react";
import GitHubPanel from "./components/GitHubPanel";
import MailPanel, { useMailCount } from "./components/MailPanel";
import NewSessionDialog from "./components/NewSessionDialog";
import OverlayPage from "./components/OverlayPage";
import RightSidebar from "./components/RightSidebar";
import SessionSidebar from "./components/SessionSidebar";
import SlackPanel, { useSlackCount } from "./components/SlackPanel";
import TerminalPane from "./components/TerminalPane";
import Toolbar from "./components/Toolbar";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "./components/ui/resizable";
import { useFileChanges } from "./hooks/useFileChanges";
import { useGitHub } from "./hooks/useGitHub";
import { useKeyboardShortcuts } from "./hooks/useKeyboardShortcuts";
import { useOverlay } from "./hooks/useOverlay";
import { useSessionTabs } from "./hooks/useSessionTabs";

export default function App() {
  const { tabs, activeTabId, createTab, closeTab, switchTab, renameTab, handleSessionExit } =
    useSessionTabs();
  const [isRightSidebarCollapsed, setIsRightSidebarCollapsed] = useState(true);
  const [isLeftSidebarCollapsed, setIsLeftSidebarCollapsed] = useState(false);
  const [showNewSession, setShowNewSession] = useState(false);

  const { prs, loading, refresh } = useGitHub();
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const {
    changes: fileChanges,
    branch,
    loading: fileChangesLoading,
    refresh: refreshFileChanges,
  } = useFileChanges(activeTab?.directory ?? null);
  const { activeOverlay, toggleOverlay, closeOverlay } = useOverlay();
  const slackCount = useSlackCount();
  const mailCount = useMailCount();

  const handleSwitchByIndex = useCallback(
    (index: number) => {
      if (index < tabs.length) {
        switchTab(tabs[index].id);
      }
    },
    [tabs, switchTab],
  );

  const handleCloseActive = useCallback(() => {
    if (activeTabId) closeTab(activeTabId);
  }, [activeTabId, closeTab]);

  useKeyboardShortcuts({
    onNewSession: () => setShowNewSession(true),
    onSwitchSession: handleSwitchByIndex,
    onCloseSession: handleCloseActive,
  });

  return (
    <div className="size-full flex flex-col bg-base relative z-10">
      <Toolbar
        activeOverlay={activeOverlay}
        onToggleOverlay={toggleOverlay}
        githubCount={prs.length}
        slackCount={slackCount}
        mailCount={mailCount}
      />

      <div className="flex-1 flex overflow-hidden relative">
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
              onNewSession={() => setShowNewSession(true)}
              onDeleteSession={closeTab}
              onRenameSession={renameTab}
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
            minSize={isRightSidebarCollapsed ? "3" : "15"}
            maxSize={isRightSidebarCollapsed ? "3" : "35"}
          >
            <RightSidebar
              isCollapsed={isRightSidebarCollapsed}
              onToggleCollapse={() => setIsRightSidebarCollapsed(!isRightSidebarCollapsed)}
              fileChanges={fileChanges}
              branch={branch}
              fileChangesLoading={fileChangesLoading}
              onRefreshFileChanges={refreshFileChanges}
            />
          </ResizablePanel>
        </ResizablePanelGroup>

        {/* Floating Overlays */}
        <OverlayPage
          open={activeOverlay === "github"}
          onClose={closeOverlay}
          title="Pull Requests"
          icon={<GitPullRequest className="size-4" />}
        >
          <GitHubPanel prs={prs} loading={loading} refresh={refresh} />
        </OverlayPage>

        <OverlayPage
          open={activeOverlay === "slack"}
          onClose={closeOverlay}
          title="Slack Messages"
          icon={<MessageSquare className="size-4" />}
        >
          <SlackPanel />
        </OverlayPage>

        <OverlayPage
          open={activeOverlay === "mail"}
          onClose={closeOverlay}
          title="Mail"
          icon={<Mail className="size-4" />}
        >
          <MailPanel />
        </OverlayPage>

        <NewSessionDialog
          open={showNewSession}
          onClose={() => setShowNewSession(false)}
          onCreate={(name, directory) => {
            createTab(name, directory);
            setShowNewSession(false);
          }}
        />
      </div>
    </div>
  );
}
