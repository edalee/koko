import { Bell, GitPullRequest, MessageSquare, Settings } from "lucide-react";
import { useCallback, useState } from "react";
import ClaudeModeSwitcher from "./components/ClaudeModeSwitcher";
import GitHubPanel from "./components/GitHubPanel";
import NewSessionDialog from "./components/NewSessionDialog";
import NotificationsPanel from "./components/NotificationsPanel";
import OverlayPage from "./components/OverlayPage";
import QuickTerminal from "./components/QuickTerminal";
import RightSidebar from "./components/RightSidebar";
import SafeWorkingOverlay from "./components/SafeWorkingOverlay";
import SessionSidebar from "./components/SessionSidebar";
import SettingsPanel from "./components/SettingsPanel";
import SlackPanel from "./components/SlackPanel";
import TerminalPane from "./components/TerminalPane";
import Toolbar from "./components/Toolbar";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "./components/ui/resizable";
import { useFileChanges } from "./hooks/useFileChanges";
import { useGitHub } from "./hooks/useGitHub";
import { useKeyboardShortcuts } from "./hooks/useKeyboardShortcuts";
import { useNotifications } from "./hooks/useNotifications";
import { useOverlay } from "./hooks/useOverlay";
import { useSafeWorking } from "./hooks/useSafeWorking";
import { useSessionTabs } from "./hooks/useSessionTabs";
import { useSlack } from "./hooks/useSlack";
import { useSubagents } from "./hooks/useSubagents";
import { useUpdateCheck } from "./hooks/useUpdateCheck";

export default function App() {
  const { tabs, activeTabId, createTab, closeTab, switchTab, renameTab, handleSessionExit } =
    useSessionTabs();
  const [isRightSidebarCollapsed, setIsRightSidebarCollapsed] = useState(true);
  const [isLeftSidebarCollapsed, setIsLeftSidebarCollapsed] = useState(false);
  const [showNewSession, setShowNewSession] = useState(false);
  const [showQuickTerminal, setShowQuickTerminal] = useState(false);

  const { prs, loading, refresh } = useGitHub();
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const {
    changes: fileChanges,
    branch,
    loading: fileChangesLoading,
    refresh: refreshFileChanges,
  } = useFileChanges(activeTab?.directory ?? null);
  const { activeOverlay, toggleOverlay, closeOverlay } = useOverlay();
  const {
    messages: slackMessages,
    loading: slackLoading,
    configured: slackConfigured,
    unreadCount: slackCount,
    refresh: refreshSlack,
    openMessage: openSlackMessage,
  } = useSlack();
  const {
    notifications,
    unreadCount: notifCount,
    loading: notifLoading,
    filter: notifFilter,
    setFilter: setNotifFilter,
    refresh: refreshNotifications,
    markRead: markNotifRead,
  } = useNotifications();
  const {
    processes,
    agentCount,
    loading: processesLoading,
    refresh: refreshProcesses,
  } = useSubagents(activeTabId);
  const {
    config: safeWorkingConfig,
    updateConfig: updateSafeWorking,
    isQuietHours,
    quietResumeTime,
    isBreakTime,
    breakSecondsLeft,
    skipBreak,
  } = useSafeWorking(!!activeTabId);
  const { update, dismiss: dismissUpdate } = useUpdateCheck();

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

  const handleToggleTerminal = useCallback(() => {
    setShowQuickTerminal((prev) => !prev);
  }, []);

  useKeyboardShortcuts({
    onNewSession: () => setShowNewSession(true),
    onSwitchSession: handleSwitchByIndex,
    onCloseSession: handleCloseActive,
    onToggleTerminal: handleToggleTerminal,
  });

  return (
    <div className="size-full flex flex-col bg-base relative z-10">
      <Toolbar
        activeOverlay={activeOverlay}
        onToggleOverlay={toggleOverlay}
        githubCount={prs.length}
        slackCount={slackCount}
        notifCount={notifCount}
        update={update}
        onDismissUpdate={dismissUpdate}
      />

      <div className="flex-1 flex overflow-hidden relative">
        <ResizablePanelGroup orientation="horizontal" className="flex-1">
          <ResizablePanel
            defaultSize="15"
            minSize={isLeftSidebarCollapsed ? "4" : "14"}
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
            <div className="h-full flex flex-col">
              {/* Terminal area — shrinks when Quick Terminal is open */}
              <div className="flex-1 min-h-0 relative">
                {tabs.map((tab) => (
                  <div
                    key={tab.id}
                    className="absolute inset-0 flex flex-col"
                    style={{ display: tab.id === activeTabId ? "flex" : "none" }}
                  >
                    <div className="flex-1 min-h-0">
                      {tab.connected ? (
                        <TerminalPane
                          sessionId={tab.id}
                          active={tab.id === activeTabId}
                          onExit={() => handleSessionExit(tab.id)}
                        />
                      ) : (
                        <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
                          <p className="text-sm">Session disconnected</p>
                          <p className="text-xs text-tertiary">
                            Click to reconnect with claude --continue
                          </p>
                        </div>
                      )}
                    </div>
                    {tab.connected && <ClaudeModeSwitcher sessionId={tab.id} />}
                  </div>
                ))}
                {tabs.length === 0 && (
                  <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
                    <p className="text-sm">No active sessions</p>
                    <p className="text-xs text-tertiary">
                      Press{" "}
                      <kbd className="px-1.5 py-0.5 rounded bg-white/[0.06] border border-white/[0.08] text-[10px] font-mono text-white/70">
                        ⌘N
                      </kbd>{" "}
                      to start a new session
                    </p>
                  </div>
                )}
              </div>

              {/* Quick Terminal — takes space from the terminal above */}
              <QuickTerminal
                open={showQuickTerminal}
                onClose={() => setShowQuickTerminal(false)}
                activeTabId={activeTabId}
                directory={activeTab?.directory ?? "."}
              />
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
              processes={processes}
              agentCount={agentCount}
              processesLoading={processesLoading}
              onRefreshProcesses={refreshProcesses}
              hasActiveSession={!!activeTabId}
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
          <SlackPanel
            messages={slackMessages}
            loading={slackLoading}
            configured={slackConfigured}
            onRefresh={refreshSlack}
            onOpenMessage={openSlackMessage}
            onOpenSettings={() => toggleOverlay("settings")}
          />
        </OverlayPage>

        <OverlayPage
          open={activeOverlay === "notifications"}
          onClose={closeOverlay}
          title="Notifications"
          icon={<Bell className="size-4" />}
        >
          <NotificationsPanel
            notifications={notifications}
            loading={notifLoading}
            refresh={refreshNotifications}
            filter={notifFilter}
            onFilterChange={setNotifFilter}
            onMarkRead={markNotifRead}
          />
        </OverlayPage>

        <OverlayPage
          open={activeOverlay === "settings"}
          onClose={closeOverlay}
          title="Settings"
          icon={<Settings className="size-4" />}
        >
          <SettingsPanel
            onTokenSaved={refreshSlack}
            safeWorkingConfig={safeWorkingConfig}
            onSafeWorkingChange={updateSafeWorking}
          />
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

      <SafeWorkingOverlay
        isQuietHours={isQuietHours}
        quietResumeTime={quietResumeTime}
        isBreakTime={isBreakTime}
        breakSecondsLeft={breakSecondsLeft}
        breakTotalSeconds={safeWorkingConfig.breakMinutes * 60}
        onSkipBreak={skipBreak}
      />
    </div>
  );
}
