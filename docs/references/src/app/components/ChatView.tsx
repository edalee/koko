import { useEffect, useRef } from "react";
import { ChatSession } from "../types";

type ChatViewProps = {
  chatSession: ChatSession;
};

export function ChatView({ chatSession }: ChatViewProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [chatSession.messages]);

  return (
    <div className="h-full bg-[#1e1e1e] overflow-auto">
      <div className="max-w-4xl mx-auto px-8 py-8 space-y-8 font-mono text-sm">
        {chatSession.messages.map((message, index) => {
          const isUser = message.role === "user";
          return (
            <div key={message.id} className="space-y-2">
              <div className="text-xs text-gray-500 uppercase tracking-wide">
                {isUser ? "User" : "Assistant"}
              </div>
              <div className="text-gray-200 leading-relaxed whitespace-pre-wrap">
                {message.content}
              </div>
              {index < chatSession.messages.length - 1 && (
                <div className="border-b border-white/10 mt-6" />
              )}
            </div>
          );
        })}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
}