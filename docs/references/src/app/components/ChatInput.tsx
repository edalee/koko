import { Send } from "lucide-react";
import { useState } from "react";
import { Button } from "./ui/button";
import { Textarea } from "./ui/textarea";

type ChatInputProps = {
  onSendMessage: (content: string) => void;
};

export function ChatInput({ onSendMessage }: ChatInputProps) {
  const [message, setMessage] = useState("");

  const handleSend = () => {
    if (message.trim()) {
      onSendMessage(message);
      setMessage("");
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="border-t border-white/10 bg-[#252526] p-4">
      <div className="max-w-4xl mx-auto">
        <div className="relative">
          <Textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type your message to Claude..."
            className="min-h-[60px] max-h-[200px] pr-12 bg-white/5 border-white/10 text-white placeholder:text-gray-500 resize-none"
          />
          <div className="absolute right-2 bottom-2">
            <Button
              onClick={handleSend}
              disabled={!message.trim()}
              size="sm"
              className="bg-gradient-to-r from-[#1FF2AB] to-[#24A965] hover:from-[#1FF2AB]/90 hover:to-[#24A965]/90 text-[#4A1A33] disabled:opacity-50"
            >
              <Send className="size-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}