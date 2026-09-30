import { Settings } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { GetHiddenPRs } from "../wailsjs/go/main/ConfigService";
import { Write } from "../wailsjs/go/main/TerminalManager";
import kokoBird from "./assets/koko_bird.svg";
import ClaudeModeSwitcher from "./components/ClaudeModeSwitcher";
import CodeViewer from "./components/CodeViewer";
import ConfirmDialog from "./components/ConfirmDialog";
import OverlayPage from "./components/OverlayPage";
import PRDetailOverlay from "./components/PRDetailOverlay";
import QuickTerminal from "./components/QuickTerminal";
import RightSidebar from "./components/RightSidebar";
import SafeWorkingOverlay from "./components/SafeWorkingOverlay";
import SessionDialog from "./components/SessionDialog";
import SessionSidebar from "./components/SessionSidebar";
import SettingsPanel from "./components/SettingsPanel";
import TerminalPane from "./components/TerminalPane";
import Toolbar from "./components/Toolbar";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "./components/ui/resizable";
import WorktreeRemovalDialog from "./components/WorktreeRemovalDialog";
import { useCI } from "./hooks/useCI";
import { useCodeViewer } from "./hooks/useCodeViewer";
import { useFileChanges } from "./hooks/useFileChanges";
import { useGitHub } from "./hooks/useGitHub";
import { useKeyboardShortcuts } from "./hooks/useKeyboardShortcuts";
import { useNotifications } from "./hooks/useNotifications";
import { useOverlay } from "./hooks/useOverlay";
import { useSafeWorking } from "./hooks/useSafeWorking";
import { useSessionActivity } from "./hooks/useSessionActivity";
import { useSessionBranches } from "./hooks/useSessionBranches";
import { useSessionContext } from "./hooks/useSessionContext";
import { reloadAction, useSessionTabs } from "./hooks/useSessionTabs";
import { useSubagents } from "./hooks/useSubagents";
import { useUpdateCheck } from "./hooks/useUpdateCheck";

export default function App() {
  const {
    tabs,
    activeTabId,
    createTab,
    closeTab,
    switchTab,
    reconnectTab,
    renameTab,
    handleSessionExit,
    history,
  } = useSessionTabs();
  // Plan 028 step 5: reconnecting a tab and opening a worktree both go
  // through the session dialog, instead of acting silently.
  const [reconnectFor, setReconnectFor] = useState<string | null>(null);
  const [dialogWorktree, setDialogWorktree] = useState<string | undefined>(undefined);
  // Remounts the dialog on each open, so it starts from that open's props.
  const [dialogKey, setDialogKey] = useState(0);
  const reconnectTarget = reconnectFor ? tabs.find((t) => t.id === reconnectFor) : undefined;
  const [isRightSidebarCollapsed, setIsRightSidebarCollapsed] = useState(true);
  const [isLeftSidebarCollapsed, setIsLeftSidebarCollapsed] = useState(false);
  const [pendingWorktreeClose, setPendingWorktreeClose] = useState<{
    tabId: string;
    worktreePath: string;
    sessionName: string;
  } | null>(null);
  const [showNewSession, setShowNewSession] = useState(false);
  const [quickTerminalTabs, setQuickTerminalTabs] = useState<Set<string>>(new Set());
  const [selectedPR, setSelectedPR] = useState<import("./types").GitHubPR | null>(null);
  const [prOverlayOpen, setPrOverlayOpen] = useState(false);
  const [hiddenPRs, setHiddenPRs] = useState<Set<string>>(new Set());

  const loadHiddenPRs = useCallback(() => {
    GetHiddenPRs().then((map) => setHiddenPRs(new Set(Object.keys(map || {}))));
  }, []);

  useEffect(() => {
    loadHiddenPRs();
  }, [loadHiddenPRs]);

  const connectedIds = tabs.filter((t) => t.connected).map((t) => t.id);
  const sessionStates = useSessionActivity(connectedIds);
  const sessionBranches = useSessionBranches(tabs.map((t) => t.directory));
  const { prs, loading, refresh } = useGitHub();
  const visiblePRCount = prs.filter((p) => !hiddenPRs.has(`${p.repo}#${p.number}`)).length;
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const {
    changes: fileChanges,
    branch,
    loading: fileChangesLoading,
    refresh: refreshFileChanges,
  } = useFileChanges(activeTab?.directory ?? null);
  const { ci, loading: ciLoading } = useCI(activeTab?.directory ?? null, branch);
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

  const dialogOpen = showNewSession || reconnectTarget !== undefined;

  // Every way into the session dialog goes through here: new, reconnect, or
  // an existing worktree.
  const openSessionDialog = useCallback((opts?: { reconnect?: string; worktree?: string }) => {
    setDialogKey((k) => k + 1);
    setReconnectFor(opts?.reconnect ?? null);
    setDialogWorktree(opts?.worktree);
    setShowNewSession(!opts?.reconnect);
  }, []);

  const closeSessionDialog = useCallback(() => {
    setShowNewSession(false);
    setReconnectFor(null);
    setDialogWorktree(undefined);
  }, []);

  // Select a tab. A disconnected one opens the session dialog to choose what
  // to reconnect into, rather than reconnecting silently (plan 028 step 5).
  const selectTab = useCallback(
    (tabId: string) => {
      switchTab(tabId);
      const tab = tabs.find((t) => t.id === tabId);
      if (tab && !tab.connected) openSessionDialog({ reconnect: tabId });
    },
    [tabs, switchTab, openSessionDialog],
  );

  // Plan 028 step 6: restart a live session in place, keeping its tab, slug
  // and conversation. Set while the "Claude isn't idle" confirmation is up.
  const [pendingReload, setPendingReload] = useState<string | null>(null);
  // Set when the confirmation opens and never cleared, so its title does not
  // change during the closing animation.
  const [reloadLabel, setReloadLabel] = useState("session");
  // Any modal up. The shortcuts wait for all of them, not just the session
  // dialog: Cmd+N behind the reload confirmation opened a second dialog, and
  // one Enter then fired both.
  const modalOpen = dialogOpen || pendingReload !== null;

  // Carry out a reload the user has settled on. A tab with no stored
  // conversation opens the picker, since reloading would have to guess (D1).
  const proceedReload = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (!tab) return;
      if (!tab.claudeSessionId) {
        openSessionDialog({ reconnect: tabId });
        return;
      }
      // A failure shows on the tab's reconnect card.
      reconnectTab(tab).catch(() => {});
    },
    [tabs, openSessionDialog, reconnectTab],
  );

  const requestReload = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (!tab) return;
      // Busy comes from terminal output, tracked here. The backend's
      // GetSessionState only knows about approvals, so it called a Claude
      // that was mid-task idle, and reload killed it without asking.
      const state = tab.connected ? (sessionStates.get(tabId) ?? "unknown") : "idle";
      if (reloadAction(tab, state) === "confirm") {
        setReloadLabel(tab.slug || tab.name);
        setPendingReload(tabId);
        return;
      }
      proceedReload(tabId);
    },
    [tabs, sessionStates, proceedReload],
  );

  // The dialog is modal, so the tab shortcuts wait while it is open. Cmd+2
  // behind a reconnect dialog used to change its target mid-way.
  const handleSwitchByIndex = useCallback(
    (index: number) => {
      if (modalOpen) return;
      if (index < tabs.length) {
        selectTab(tabs[index].id);
      }
    },
    [tabs, selectTab, modalOpen],
  );

  // When closing a session, intercept if Koko created a worktree for it
  // and ask whether to clean up. Falls through to closeTab otherwise.
  const requestCloseTab = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (tab?.worktreePath) {
        setPendingWorktreeClose({
          tabId,
          worktreePath: tab.worktreePath,
          sessionName: tab.name,
        });
        return;
      }
      closeTab(tabId);
    },
    [tabs, closeTab],
  );

  const handleCloseActive = useCallback(() => {
    if (activeTabId) requestCloseTab(activeTabId);
  }, [activeTabId, requestCloseTab]);

  // Keyed by the tab's creation time, not its session id. Reconnect and
  // reload swap the session id, which used to close the Quick Terminal and
  // orphan its shell. createdAt survives both, and is already the pane key.
  const quickKey = activeTab ? String(activeTab.createdAt) : null;
  const showQuickTerminal = quickKey ? quickTerminalTabs.has(quickKey) : false;

  const handleToggleTerminal = useCallback(() => {
    if (!quickKey) return;
    setQuickTerminalTabs((prev) => {
      const next = new Set(prev);
      if (next.has(quickKey)) {
        next.delete(quickKey);
      } else {
        next.add(quickKey);
      }
      return next;
    });
  }, [quickKey]);

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
    // Cmd+N inside an open dialog would remount it and lose what was typed.
    onNewSession: () => {
      if (!modalOpen) openSessionDialog();
    },
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
              sessionBranches={sessionBranches}
              onSessionSelect={selectTab}
              onNewSession={() => openSessionDialog()}
              onDeleteSession={requestCloseTab}
              onRenameSession={renameTab}
              onReloadSession={requestReload}
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
                        onReload={() => requestReload(tab.id)}
                      />
                      {!tab.connected && (
                        // biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: reconnect overlay
                        <div
                          className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-base/60 backdrop-blur-sm cursor-pointer"
                          onClick={() => openSessionDialog({ reconnect: tab.id })}
                        >
                          <img
                            src={kokoBird}
                            alt=""
                            className="absolute bottom-0 right-0 w-full max-h-full object-contain opacity-[0.04] pointer-events-none select-none"
                          />
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
                            {tab.reconnectError && (
                              <p className="text-xs text-error pt-1" role="alert">
                                {tab.reconnectError}
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
                  <div className="relative flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
                    <img
                      src={kokoBird}
                      alt=""
                      className="absolute bottom-0 right-0 w-full max-h-full object-contain opacity-[0.04] pointer-events-none select-none"
                    />
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
                  if (quickKey) {
                    setQuickTerminalTabs((prev) => {
                      const next = new Set(prev);
                      next.delete(quickKey);
                      return next;
                    });
                  }
                }}
                activeTabId={quickKey}
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
              ci={ci}
              ciLoading={ciLoading}
              processes={processes}
              agentCount={agentCount}
              mcpServers={mcpServers}
              agents={agents}
              commands={commands}
              contextLoading={contextLoading}
              onRefreshContext={refreshContext}
              onInjectCommand={handleInjectCommand}
              hasActiveSession={!!activeTabId}
              activeDirectory={activeTab?.directory ?? null}
              onOpenWorktreeSession={(_name, directory) => {
                // Open the dialog on this worktree, so the user can pick a
                // conversation stored for it or start a new one.
                openSessionDialog({ worktree: directory });
              }}
              onFileClick={(path, staged) => {
                if (activeTab?.directory) {
                  codeViewer.openDiff(activeTab.directory, path, staged);
                }
              }}
              onFileView={(path) => {
                if (activeTab?.directory) {
                  codeViewer.openFile(activeTab.directory, path);
                }
              }}
              prs={prs}
              prsLoading={loading}
              visiblePRCount={visiblePRCount}
              hiddenPRs={hiddenPRs}
              onRefreshPRs={refresh}
              notifications={notifications}
              notifCount={notifCount}
              notifLoading={notifLoading}
              notifFilter={notifFilter}
              onNotifFilterChange={setNotifFilter}
              onRefreshNotifications={refreshNotifications}
              onMarkNotifRead={markNotifRead}
              onMarkAllNotifRead={markAllNotifRead}
              onPRClick={(pr) => {
                setSelectedPR(pr);
                setPrOverlayOpen(true);
              }}
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
            onReposChanged={refresh}
          />
        </OverlayPage>

        <CodeViewer
          open={codeViewer.isOpen}
          file={codeViewer.file}
          rawFile={codeViewer.rawFile}
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
          prFiles={codeViewer.prFiles}
          onPRFileSelect={(path) => {
            if (codeViewer.prContext) {
              codeViewer.openPRDiff(codeViewer.prContext.repo, codeViewer.prContext.number, path);
            }
          }}
        />

        <SessionDialog
          key={dialogKey}
          open={dialogOpen}
          onClose={closeSessionDialog}
          onCreate={async (name, directory, worktreePath, resume) => {
            // Let a failure reach the dialog, which shows it and stays open.
            // The dialog reports the worktree itself, including an existing
            // one opened from the worktrees module.
            await createTab(name, directory, worktreePath, resume);
            closeSessionDialog();
          }}
          reconnect={reconnectTarget}
          onReconnect={async (tab, choice) => {
            await reconnectTab(tab, choice);
            closeSessionDialog();
          }}
          worktree={dialogWorktree}
          onOpenHeld={(tabId) => {
            closeSessionDialog();
            switchTab(tabId);
            // D2: the user chose this conversation, so a disconnected holder
            // reconnects straight into it. A failure shows on its card.
            const holder = tabs.find((t) => t.id === tabId);
            if (holder && !holder.connected) reconnectTab(holder).catch(() => {});
          }}
          history={history}
          activeDirs={tabs.map((t) => t.directory)}
          tabs={tabs}
        />

        {/* UX rule 6: reload never silently discards work in progress. */}
        <ConfirmDialog
          open={pendingReload !== null}
          title={`Reload ${reloadLabel}?`}
          message="Claude isn't idle. Reloading stops what it is doing, and anything typed but not sent is lost. The conversation itself is kept."
          confirmLabel="Reload"
          destructive
          onConfirm={() => {
            const id = pendingReload;
            setPendingReload(null);
            // Confirmed first, even for a tab with no conversation, which
            // then goes on to the picker.
            if (id) proceedReload(id);
          }}
          onCancel={() => setPendingReload(null)}
        />

        <WorktreeRemovalDialog
          open={pendingWorktreeClose !== null}
          worktreePath={pendingWorktreeClose?.worktreePath ?? ""}
          sessionName={pendingWorktreeClose?.sessionName ?? ""}
          onResolved={() => {
            const tabId = pendingWorktreeClose?.tabId;
            setPendingWorktreeClose(null);
            if (tabId) closeTab(tabId);
          }}
        />

        <PRDetailOverlay
          open={prOverlayOpen}
          prs={prs}
          selectedPR={selectedPR}
          onSelectPR={setSelectedPR}
          onClose={() => {
            setPrOverlayOpen(false);
            setSelectedPR(null);
          }}
          onRefresh={refresh}
          hiddenPRs={hiddenPRs}
          onHiddenChange={loadHiddenPRs}
          onOpenDiff={(repo, number, path, files) =>
            codeViewer.openPRDiff(repo, number, path, files)
          }
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
