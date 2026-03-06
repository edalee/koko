import { FolderTree, GitPullRequest, MessageSquare, Mail, X } from "lucide-react";
import { useState } from "react";
import { FileExplorer, FileNode } from "./FileExplorer";
import { Session, ChatSession } from "../types";
import { Button } from "./ui/button";

type RightSidebarProps = {
  isOpen: boolean;
  onClose: () => void;
  activeSession: Session | null;
  onFileSelect: (file: FileNode) => void;
  onChatSelect: (chat: ChatSession) => void;
  selectedFile: FileNode | null;
};

type SidebarModule = "explorer" | "github" | "slack" | "email";

export function RightSidebar({ isOpen, onClose, activeSession, onFileSelect, onChatSelect, selectedFile }: RightSidebarProps) {
  const [activeModule, setActiveModule] = useState<SidebarModule>("explorer");

  if (!isOpen) return null;

  return (
    <div className="h-full flex bg-[#1e1e1e] border-l border-white/10">
      {/* Module Icons */}
      <div className="w-12 bg-[#252526] border-r border-white/10 flex flex-col items-center py-3 gap-2">
        <button
          onClick={() => setActiveModule("explorer")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "explorer"
              ? "bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 text-[#1FF2AB]"
              : "text-gray-400 hover:text-white hover:bg-white/5"
          }`}
          title="Explorer"
        >
          <FolderTree className="size-5" />
        </button>
        <button
          onClick={() => setActiveModule("github")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "github"
              ? "bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 text-[#1FF2AB]"
              : "text-gray-400 hover:text-white hover:bg-white/5"
          }`}
          title="GitHub"
        >
          <GitPullRequest className="size-5" />
        </button>
        <button
          onClick={() => setActiveModule("slack")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "slack"
              ? "bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 text-[#1FF2AB]"
              : "text-gray-400 hover:text-white hover:bg-white/5"
          }`}
          title="Slack"
        >
          <MessageSquare className="size-5" />
        </button>
        <button
          onClick={() => setActiveModule("email")}
          className={`p-2 rounded-lg transition-colors ${
            activeModule === "email"
              ? "bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 text-[#1FF2AB]"
              : "text-gray-400 hover:text-white hover:bg-white/5"
          }`}
          title="Email"
        >
          <Mail className="size-5" />
        </button>
        
        <div className="flex-1" />
        
        <button
          onClick={onClose}
          className="p-2 rounded-lg text-gray-400 hover:text-white hover:bg-white/5 transition-colors"
          title="Close"
        >
          <X className="size-5" />
        </button>
      </div>

      {/* Module Content */}
      <div className="flex-1 min-w-[300px] flex flex-col">
        {activeModule === "explorer" && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="border-b border-white/10 p-3">
              <h3 className="text-white text-sm mb-3">Files</h3>
              <div className="flex-1 overflow-auto">
                <FileExplorer
                  files={activeSession?.files || []}
                  onFileSelect={onFileSelect}
                  selectedFile={selectedFile}
                />
              </div>
            </div>
            <div className="flex-1 p-3 overflow-auto">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-white text-sm">Claude Chats</h3>
                <Button
                  size="sm"
                  onClick={() => {
                    const newChat: ChatSession = {
                      id: Date.now().toString(),
                      name: "New Chat",
                      messages: [],
                    };
                    onChatSelect(newChat);
                  }}
                  className="bg-gradient-to-r from-[#1FF2AB] to-[#24A965] hover:from-[#1FF2AB]/90 hover:to-[#24A965]/90 text-[#4A1A33] h-7 text-xs"
                >
                  + New Chat
                </Button>
              </div>
              <div className="space-y-2">
                {activeSession?.chatSessions.map((chat) => (
                  <div
                    key={chat.id}
                    onClick={() => onChatSelect(chat)}
                    className="p-3 bg-white/5 rounded-lg border border-white/10 hover:bg-white/10 transition-colors cursor-pointer"
                  >
                    <div className="flex items-start gap-2">
                      <MessageSquare className="size-4 text-[#1FF2AB] mt-0.5 shrink-0" />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm text-white truncate">{chat.name}</p>
                        <p className="text-xs text-gray-400">{chat.messages.length} messages</p>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
        
        {activeModule === "github" && (
          <div className="h-full p-4">
            <h3 className="text-white mb-4">Pull Requests</h3>
            <div className="space-y-3">
              {[1, 2, 3].map((pr) => (
                <div key={pr} className="p-3 bg-white/5 rounded-lg border border-white/10 hover:bg-white/10 transition-colors cursor-pointer">
                  <div className="flex items-start gap-3">
                    <GitPullRequest className="size-4 text-[#1FF2AB] mt-0.5" />
                    <div>
                      <p className="text-sm text-white mb-1">feat: Add new component #{pr}23</p>
                      <p className="text-xs text-gray-400">opened 2 hours ago by user{pr}</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
        
        {activeModule === "slack" && (
          <div className="h-full p-4">
            <h3 className="text-white mb-4">Slack Messages</h3>
            <div className="space-y-3">
              {[1, 2, 3].map((msg) => (
                <div key={msg} className="p-3 bg-white/5 rounded-lg border border-white/10 hover:bg-white/10 transition-colors cursor-pointer">
                  <div className="flex items-start gap-3">
                    <div className="size-8 rounded bg-gradient-to-r from-[#1FF2AB] to-[#24A965] flex items-center justify-center text-[#4A1A33] text-xs font-bold">
                      U{msg}
                    </div>
                    <div className="flex-1">
                      <p className="text-sm text-white mb-1">User {msg}</p>
                      <p className="text-xs text-gray-400">Hey, can you review this?</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
        
        {activeModule === "email" && (
          <div className="h-full p-4">
            <h3 className="text-white mb-4">Emails</h3>
            <div className="space-y-3">
              {[1, 2, 3].map((email) => (
                <div key={email} className="p-3 bg-white/5 rounded-lg border border-white/10 hover:bg-white/10 transition-colors cursor-pointer">
                  <div className="flex items-start gap-3">
                    <Mail className="size-4 text-[#1FF2AB] mt-0.5" />
                    <div className="flex-1">
                      <p className="text-sm text-white mb-1">Project Update #{email}</p>
                      <p className="text-xs text-gray-400">from: team@example.com</p>
                      <p className="text-xs text-gray-500 mt-1">Meeting scheduled for tomorrow...</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}