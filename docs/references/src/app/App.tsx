import { useState } from "react";
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from "./components/ui/resizable";
import { SessionSidebar } from "./components/SessionSidebar";
import { ChatInterface } from "./components/ChatInterface";
import { FileNode } from "./components/FileExplorer";
import { Session, ChatSession, Message } from "./types";
import { CollapsibleTerminal } from "./components/CollapsibleTerminal";
import { Toolbar } from "./components/Toolbar";
import { TabBar } from "./components/TabBar";
import { RightSidebar } from "./components/RightSidebar";
import { RightSidebarToggle } from "./components/RightSidebarToggle";

const mockFiles: FileNode[] = [
  {
    name: "src",
    type: "folder",
    children: [
      {
        name: "App.tsx",
        type: "file",
        language: "tsx",
        content: `import React from 'react';
import { Header } from './components/Header';
import { MainContent } from './components/MainContent';

function App() {
  return (
    <div className="app">
      <Header title="My Application" />
      <MainContent />
    </div>
  );
}

export default App;`,
      },
      {
        name: "index.tsx",
        type: "file",
        language: "tsx",
        content: `import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);`,
      },
    ],
  },
  {
    name: "package.json",
    type: "file",
    language: "json",
    content: `{
  "name": "my-app",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  }
}`,
  },
];

const mockChatSessions: ChatSession[] = [
  {
    id: "chat-1",
    name: "React Best Practices",
    messages: [
      {
        id: "msg-1",
        role: "user",
        content: "What are the best practices for React in 2026?",
        timestamp: "10:30 AM",
      },
      {
        id: "msg-2",
        role: "assistant",
        content: "Here are key React best practices for 2026:\n\n1. Use Server Components for better performance\n2. Leverage concurrent features like useTransition\n3. Keep components focused and small\n4. Use TypeScript for type safety\n5. Implement proper error boundaries",
        timestamp: "10:31 AM",
      },
    ],
  },
  {
    id: "chat-2",
    name: "App.tsx",
    messages: [
      {
        id: "msg-3",
        role: "user",
        content: "Create a React App component with header and main content",
        timestamp: "9:15 AM",
      },
      {
        id: "msg-4",
        role: "assistant",
        content: "Here's a React App component:\n\n```tsx\nimport React from 'react';\nimport { Header } from './components/Header';\nimport { MainContent } from './components/MainContent';\n\nfunction App() {\n  return (\n    <div className=\"app\">\n      <Header title=\"My Application\" />\n      <MainContent />\n    </div>\n  );\n}\n\nexport default App;\n```",
        timestamp: "9:16 AM",
      },
    ],
  },
];

const mockSessions: Session[] = [
  {
    id: "1",
    title: "React App Project",
    createdAt: "2026-03-01T10:30:00",
    files: mockFiles,
    chatSessions: mockChatSessions,
    activeFile: null,
    activeChatSession: mockChatSessions[1], // Set App.tsx chat as active
  },
  {
    id: "2",
    title: "TypeScript Utils",
    createdAt: "2026-02-28T14:15:00",
    files: [
      {
        name: "utils.ts",
        type: "file",
        language: "typescript",
        content: `export const formatDate = (date: Date): string => {
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });
};`,
      },
    ],
    chatSessions: [],
    activeFile: null,
    activeChatSession: null,
  },
];

export default function App() {
  const [sessions, setSessions] = useState<Session[]>(mockSessions);
  const [activeSession, setActiveSession] = useState<Session | null>(mockSessions[0]);
  const [openSessions, setOpenSessions] = useState<Session[]>([mockSessions[0]]);
  const [openFiles, setOpenFiles] = useState<FileNode[]>([]);
  const [terminalOutput, setTerminalOutput] = useState<string[]>([
    "$ Welcome to KõKõ Terminal",
    "$ Ready for commands",
  ]);
  const [terminalHeight, setTerminalHeight] = useState(200);
  const [terminalCollapsed, setTerminalCollapsed] = useState(false);
  const [isRightSidebarOpen, setIsRightSidebarOpen] = useState(false);
  const [isLeftSidebarCollapsed, setIsLeftSidebarCollapsed] = useState(false);

  const handleNewSession = () => {
    const newSession: Session = {
      id: Date.now().toString(),
      title: "New Session",
      createdAt: new Date().toISOString(),
      files: [],
      chatSessions: [],
      activeFile: null,
      activeChatSession: null,
    };
    setSessions([newSession, ...sessions]);
    setActiveSession(newSession);
    if (!openSessions.find((s) => s.id === newSession.id)) {
      setOpenSessions([...openSessions, newSession]);
    }
  };

  const handleSessionSelect = (session: Session) => {
    setActiveSession(session);
    if (!openSessions.find((s) => s.id === session.id)) {
      setOpenSessions([...openSessions, session]);
    }
    setOpenFiles(session.activeFile ? [session.activeFile] : []);
  };

  const handleDeleteSession = (sessionId: string) => {
    setSessions(sessions.filter((s) => s.id !== sessionId));
    setOpenSessions(openSessions.filter((s) => s.id !== sessionId));
    if (activeSession?.id === sessionId) {
      const remaining = openSessions.filter((s) => s.id !== sessionId);
      setActiveSession(remaining.length > 0 ? remaining[0] : null);
    }
  };

  const handleSessionClose = (sessionId: string) => {
    const newOpenSessions = openSessions.filter((s) => s.id !== sessionId);
    setOpenSessions(newOpenSessions);
    if (activeSession?.id === sessionId) {
      setActiveSession(newOpenSessions.length > 0 ? newOpenSessions[0] : null);
    }
  };

  const handleFileSelect = (file: FileNode) => {
    if (activeSession) {
      const updatedSession = { ...activeSession, activeFile: file, activeChatSession: null };
      setActiveSession(updatedSession);
      setSessions(sessions.map((s) => (s.id === activeSession.id ? updatedSession : s)));
      if (!openFiles.find((f) => f.name === file.name)) {
        setOpenFiles([...openFiles, file]);
      }
    }
  };

  const handleChatSelect = (chat: ChatSession) => {
    if (activeSession) {
      // If it's a new chat, add it to the session
      let updatedChatSessions = activeSession.chatSessions;
      if (!activeSession.chatSessions.find((c) => c.id === chat.id)) {
        updatedChatSessions = [...activeSession.chatSessions, chat];
      }
      
      const updatedSession = {
        ...activeSession,
        chatSessions: updatedChatSessions,
        activeChatSession: chat,
        activeFile: null,
      };
      setActiveSession(updatedSession);
      setSessions(sessions.map((s) => (s.id === activeSession.id ? updatedSession : s)));
    }
  };

  const handleTabClick = (file: FileNode) => {
    if (activeSession) {
      const updatedSession = { ...activeSession, activeFile: file, activeChatSession: null };
      setActiveSession(updatedSession);
      setSessions(sessions.map((s) => (s.id === activeSession.id ? updatedSession : s)));
    }
  };

  const handleTabClose = (file: FileNode) => {
    const newOpenFiles = openFiles.filter((f) => f.name !== file.name);
    setOpenFiles(newOpenFiles);
    if (activeSession?.activeFile?.name === file.name) {
      const newActiveFile = newOpenFiles.length > 0 ? newOpenFiles[newOpenFiles.length - 1] : null;
      const updatedSession = { ...activeSession, activeFile: newActiveFile };
      setActiveSession(updatedSession);
      setSessions(sessions.map((s) => (s.id === activeSession.id ? updatedSession : s)));
    }
  };

  const handleSendMessage = (content: string) => {
    if (!activeSession?.activeChatSession) return;

    const userMessage: Message = {
      id: `msg-${Date.now()}`,
      role: "user",
      content,
      timestamp: new Date().toLocaleTimeString("en-US", {
        hour: "numeric",
        minute: "2-digit",
      }),
    };

    const assistantMessage: Message = {
      id: `msg-${Date.now() + 1}`,
      role: "assistant",
      content:
        "I'm a demo response! In a real application, this would be connected to Claude's API to provide intelligent responses to your questions.",
      timestamp: new Date().toLocaleTimeString("en-US", {
        hour: "numeric",
        minute: "2-digit",
      }),
    };

    const updatedChat = {
      ...activeSession.activeChatSession,
      messages: [...activeSession.activeChatSession.messages, userMessage, assistantMessage],
      name:
        activeSession.activeChatSession.messages.length === 0
          ? content.slice(0, 40) + (content.length > 40 ? "..." : "")
          : activeSession.activeChatSession.name,
    };

    const updatedChatSessions = activeSession.chatSessions.map((c) =>
      c.id === updatedChat.id ? updatedChat : c
    );

    const updatedSession = {
      ...activeSession,
      chatSessions: updatedChatSessions,
      activeChatSession: updatedChat,
    };

    setActiveSession(updatedSession);
    setSessions(sessions.map((s) => (s.id === activeSession.id ? updatedSession : s)));

    setTerminalOutput([
      ...terminalOutput,
      `$ Message sent to Claude chat: "${updatedChat.name}"`,
    ]);
  };

  return (
    <div className="size-full flex flex-col bg-[#1e1e1e]">
      <Toolbar />
      <div className="flex-1 flex overflow-hidden relative">
        <ResizablePanelGroup direction="horizontal" className="flex-1">
          <ResizablePanel defaultSize={15} minSize={12} maxSize={25}>
            <SessionSidebar
              sessions={sessions}
              activeSession={activeSession}
              onSessionSelect={handleSessionSelect}
              onNewSession={handleNewSession}
              onDeleteSession={handleDeleteSession}
              isCollapsed={isLeftSidebarCollapsed}
              onToggleCollapse={() => setIsLeftSidebarCollapsed(!isLeftSidebarCollapsed)}
            />
          </ResizablePanel>
          <ResizableHandle className="w-[1px] bg-white/10 hover:bg-[#1FF2AB]/50 transition-colors" />
          <ResizablePanel defaultSize={85}>
            <div className="h-full flex flex-col relative">
              <div 
                className="flex-1 flex flex-col overflow-hidden"
                style={{ 
                  marginBottom: terminalCollapsed ? 42 : terminalHeight 
                }}
              >
                <TabBar
                  openFiles={openFiles}
                  activeFile={activeSession?.activeFile || null}
                  onTabClick={handleTabClick}
                  onTabClose={handleTabClose}
                />
                <div className="flex-1 overflow-hidden">
                  <ChatInterface
                    session={activeSession}
                    onCodeChange={() => {}}
                    onSendMessage={handleSendMessage}
                  />
                </div>
              </div>
              <div 
                className="absolute bottom-0 left-0 right-0 border-t border-white/10"
                style={{ height: terminalCollapsed ? 42 : terminalHeight }}
              >
                <CollapsibleTerminal
                  output={terminalOutput}
                  onClear={() => setTerminalOutput(["$ Terminal cleared"])}
                />
              </div>
            </div>
          </ResizablePanel>
        </ResizablePanelGroup>
        
        {/* Always visible right bar with toggle */}
        <RightSidebarToggle 
          isOpen={isRightSidebarOpen}
          onToggle={() => setIsRightSidebarOpen(!isRightSidebarOpen)}
        />
        
        {/* Right sidebar content */}
        {isRightSidebarOpen && (
          <div className="absolute right-12 top-0 bottom-0 w-[400px] border-l border-white/10 bg-[#1e1e1e] z-10">
            <RightSidebar
              isOpen={isRightSidebarOpen}
              onClose={() => setIsRightSidebarOpen(false)}
              activeSession={activeSession}
              onFileSelect={handleFileSelect}
              onChatSelect={handleChatSelect}
              selectedFile={activeSession?.activeFile || null}
            />
          </div>
        )}
      </div>
    </div>
  );
}
