import { Settings } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetHiddenPRs } from "../wailsjs/go/main/ConfigService";
import { Write } from "../wailsjs/go/main/TerminalManager";
import ClaudeModeSwitcher from "./components/ClaudeModeSwitcher";
import CodeViewer from "./components/CodeViewer";
import NewSessionDialog from "./components/NewSessionDialog";
import OverlayPage from "./components/OverlayPage";
import PRDetailOverlay from "./components/PRDetailOverlay";
import QuickTerminal from "./components/QuickTerminal";
import RightSidebar from "./components/RightSidebar";
import SafeWorkingOverlay from "./components/SafeWorkingOverlay";
import SessionSidebar from "./components/SessionSidebar";
import SettingsPanel from "./components/SettingsPanel";
import TerminalPane from "./components/TerminalPane";
import Toolbar from "./components/Toolbar";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "./components/ui/resizable";
import { useCodeViewer } from "./hooks/useCodeViewer";
import { useFileChanges } from "./hooks/useFileChanges";
import { useGitHub } from "./hooks/useGitHub";
import { useKeyboardShortcuts } from "./hooks/useKeyboardShortcuts";
import { useNotifications } from "./hooks/useNotifications";
import { useOverlay } from "./hooks/useOverlay";
import { useSafeWorking } from "./hooks/useSafeWorking";
import { useSessionActivity } from "./hooks/useSessionActivity";
import { useSessionContext } from "./hooks/useSessionContext";
import { useSessionTabs } from "./hooks/useSessionTabs";
import { useSubagents } from "./hooks/useSubagents";
import { useUpdateCheck } from "./hooks/useUpdateCheck";

export default function App() {
  const {
    tabs,
    activeTabId,
    createTab,
    closeTab,
    switchTab,
    renameTab,
    handleSessionExit,
    history,
  } = useSessionTabs();
  const [isRightSidebarCollapsed, setIsRightSidebarCollapsed] = useState(true);
  const [isLeftSidebarCollapsed, setIsLeftSidebarCollapsed] = useState(false);
  const [showNewSession, setShowNewSession] = useState(false);
  const [quickTerminalTabs, setQuickTerminalTabs] = useState<Set<string>>(new Set());
  const [selectedPR, setSelectedPR] = useState<import("./types").GitHubPR | null>(null);
  const [hiddenPRs, setHiddenPRs] = useState<Set<string>>(new Set());

  const loadHiddenPRs = useCallback(() => {
    GetHiddenPRs().then((map) => setHiddenPRs(new Set(Object.keys(map || {}))));
  }, []);

  useEffect(() => {
    loadHiddenPRs();
  }, [loadHiddenPRs]);

  const connectedIds = tabs.filter((t) => t.connected).map((t) => t.id);
  const sessionStates = useSessionActivity(connectedIds);
  const { prs, loading, refresh } = useGitHub();
  const visiblePRCount = prs.filter((p) => !hiddenPRs.has(`${p.repo}#${p.number}`)).length;
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const {
    changes: fileChanges,
    branch,
    loading: fileChangesLoading,
    refresh: refreshFileChanges,
  } = useFileChanges(activeTab?.directory ?? null);
  const codeViewer = useCodeViewer();
  const { activeOverlay, toggleOverlay, closeOverlay } = useOverlay();
  const {
    notifications,
    unreadCount: notifCount,
    loading: notifLoading,
    filter: notifFilter,
    setFilter: setNotifFilter,
    refresh: refreshNotifications,
    markRead: markNotifRead,
    markAllRead: markAllNotifRead,
  } = useNotifications();
  const { processes, agentCount } = useSubagents(activeTabId);
  const {
    mcpServers,
    agents,
    commands,
    loading: contextLoading,
    refresh: refreshContext,
  } = useSessionContext(activeTab?.directory ?? null);
  const {
    config: safeWorkingConfig,
    updateConfig: updateSafeWorking,
    isQuietHours,
    quietResumeTime,
    isBreakTime,
    breakSecondsLeft,
    skipBreak,
    delayQuietHours,
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

  const showQuickTerminal = activeTabId ? quickTerminalTabs.has(activeTabId) : false;

  const handleToggleTerminal = useCallback(() => {
    if (!activeTabId) return;
    setQuickTerminalTabs((prev) => {
      const next = new Set(prev);
      if (next.has(activeTabId)) {
        next.delete(activeTabId);
      } else {
        next.add(activeTabId);
      }
      return next;
    });
  }, [activeTabId]);

  const handleInjectCommand = useCallback(
    (command: string) => {
      if (activeTabId) {
        const bytes = new TextEncoder().encode(command);
        let binary = "";
        for (let i = 0; i < bytes.length; i++) {
          binary += String.fromCharCode(bytes[i]);
        }
        Write(activeTabId, btoa(binary));
      }
    },
    [activeTabId],
  );

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
              sessionStates={sessionStates}
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
                    key={tab.createdAt}
                    className="absolute inset-0 flex flex-col"
                    style={{
                      visibility: tab.id === activeTabId ? "visible" : "hidden",
                      zIndex: tab.id === activeTabId ? 1 : 0,
                    }}
                  >
                    <div className="flex-1 min-h-0 relative">
                      <TerminalPane
                        sessionId={tab.id}
                        active={tab.id === activeTabId}
                        onExit={() => handleSessionExit(tab.id)}
                      />
                      {!tab.connected && (
                        // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: reconnect overlay
                        <div
                          className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-base/60 backdrop-blur-sm cursor-pointer"
                          onClick={() => switchTab(tab.id)}
                        >
                          <div className="max-w-sm w-full mx-auto glass-card rounded-xl p-5 border border-white/[0.08] space-y-3">
                            <div className="flex items-center gap-2">
                              <span className="size-2 rounded-full bg-white/20" />
                              <h3 className="text-sm text-white font-medium truncate">
                                {tab.name}
                              </h3>
                            </div>
                            <p className="text-xs text-muted-foreground truncate">
                              {tab.directory.replace(/^\/Users\/[^/]+/, "~")}
                            </p>
                            {tab.lastMsg && (
                              <p className="text-xs text-white/40 italic line-clamp-3 leading-relaxed">
                                {tab.lastMsg}
                              </p>
                            )}
                            <p className="text-xs text-accent pt-1">Click to reconnect</p>
                          </div>
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
                onClose={() => {
                  if (activeTabId) {
                    setQuickTerminalTabs((prev) => {
                      const next = new Set(prev);
                      next.delete(activeTabId);
                      return next;
                    });
                  }
                }}
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
              mcpServers={mcpServers}
              agents={agents}
              commands={commands}
              contextLoading={contextLoading}
              onRefreshContext={refreshContext}
              onInjectCommand={handleInjectCommand}
              hasActiveSession={!!activeTabId}
              onFileClick={(path, staged) => {
                if (activeTab?.directory) {
                  codeViewer.openDiff(activeTab.directory, path, staged);
                }
              }}
              prs={prs}
              prsLoading={loading}
              visiblePRCount={visiblePRCount}
              onRefreshPRs={refresh}
              notifications={notifications}
              notifCount={notifCount}
              notifLoading={notifLoading}
              notifFilter={notifFilter}
              onNotifFilterChange={setNotifFilter}
              onRefreshNotifications={refreshNotifications}
              onMarkNotifRead={markNotifRead}
              onMarkAllNotifRead={markAllNotifRead}
              onPRClick={setSelectedPR}
            />
          </ResizablePanel>
        </ResizablePanelGroup>

        {/* Floating Overlays */}
        <OverlayPage
          open={activeOverlay === "settings"}
          onClose={closeOverlay}
          title="Settings"
          icon={<Settings className="size-4" />}
        >
          <SettingsPanel
            safeWorkingConfig={safeWorkingConfig}
            onSafeWorkingChange={updateSafeWorking}
          />
        </OverlayPage>

        <CodeViewer
          open={codeViewer.isOpen}
          file={codeViewer.file}
          loading={codeViewer.loading}
          viewMode={codeViewer.viewMode}
          filePath={codeViewer.filePath}
          staged={codeViewer.staged}
          fileChanges={fileChanges}
          onClose={codeViewer.close}
          onSetViewMode={codeViewer.setViewMode}
          onFileSelect={(path, isStaged) => {
            if (activeTab?.directory) {
              codeViewer.openDiff(activeTab.directory, path, isStaged);
            }
          }}
        />

        <NewSessionDialog
          open={showNewSession}
          onClose={() => setShowNewSession(false)}
          onCreate={(name, directory) => {
            createTab(name, directory);
            setShowNewSession(false);
          }}
          history={history}
          activeDirs={tabs.map((t) => t.directory)}
        />

        <PRDetailOverlay
          open={!!selectedPR}
          prs={prs}
          selectedPR={selectedPR}
          onSelectPR={setSelectedPR}
          onClose={() => setSelectedPR(null)}
          onRefresh={refresh}
          hiddenPRs={hiddenPRs}
          onHiddenChange={loadHiddenPRs}
        />
      </div>

      <SafeWorkingOverlay
        isQuietHours={isQuietHours}
        quietResumeTime={quietResumeTime}
        isBreakTime={isBreakTime}
        breakSecondsLeft={breakSecondsLeft}
        breakTotalSeconds={safeWorkingConfig.breakMinutes * 60}
        onSkipBreak={skipBreak}
        onDelayQuietHours={delayQuietHours}
      />
    </div>
  );
}
