import { Session, ChatSession } from "../types";
import { Code2, MessageSquare } from "lucide-react";
import { CodeEditor } from "./CodeEditor";
import { ChatView } from "./ChatView";
import { ChatInput } from "./ChatInput";

type ChatInterfaceProps = {
  session: Session | null;
  onCodeChange: (code: string) => void;
  onSendMessage: (content: string) => void;
};

export function ChatInterface({ session, onCodeChange, onSendMessage }: ChatInterfaceProps) {
  if (!session) {
    return (
      <div className="h-full flex items-center justify-center bg-[#1e1e1e]">
        <div className="text-center">
          <div className="mb-6 flex justify-center">
            <div className="p-6 bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 rounded-full">
              <Code2 className="size-16 text-[#1FF2AB]" />
            </div>
          </div>
          <h2 className="text-3xl text-white mb-3">KõKõ Editor</h2>
          <p className="text-gray-400 max-w-md mx-auto">
            Select a session from the sidebar or create a new one to start coding
          </p>
        </div>
      </div>
    );
  }

  // Show chat view if there's an active chat session
  if (session.activeChatSession) {
    return (
      <div className="h-full flex flex-col bg-[#1e1e1e]">
        <div className="flex-1 overflow-hidden">
          <ChatView chatSession={session.activeChatSession} />
        </div>
        <ChatInput onSendMessage={onSendMessage} />
      </div>
    );
  }

  // Show code editor if there's an active file
  if (session.activeFile) {
    return (
      <div className="h-full bg-[#1e1e1e]">
        <CodeEditor file={session.activeFile} onCodeChange={onCodeChange} />
      </div>
    );
  }

  // Empty state
  return (
    <div className="h-full flex items-center justify-center bg-[#1e1e1e]">
      <div className="text-center">
        <div className="flex gap-4 justify-center mb-6">
          <div className="p-6 bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 rounded-full">
            <Code2 className="size-12 text-[#1FF2AB]" />
          </div>
          <div className="p-6 bg-gradient-to-r from-[#1FF2AB]/20 to-[#24A965]/20 rounded-full">
            <MessageSquare className="size-12 text-[#1FF2AB]" />
          </div>
        </div>
        <h2 className="text-xl text-white mb-3">No file or chat selected</h2>
        <p className="text-gray-400 max-w-md mx-auto">
          Open a file from the explorer or start a chat conversation
        </p>
      </div>
    </div>
  );
}