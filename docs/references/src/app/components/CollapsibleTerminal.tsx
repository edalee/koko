import { Terminal as TerminalIcon, ChevronUp, ChevronDown, X } from "lucide-react";
import { useState } from "react";

type CollapsibleTerminalProps = {
  output: string[];
  onClear: () => void;
};

export function CollapsibleTerminal({ output, onClear }: CollapsibleTerminalProps) {
  const [isCollapsed, setIsCollapsed] = useState(false);

  return (
    <div className={`bg-[#1e1e1e] border-t border-white/10 transition-all ${isCollapsed ? "h-10" : "h-full"}`}>
      <div 
        className="flex items-center justify-between px-4 py-2 bg-[#252526] border-b border-white/10 cursor-pointer hover:bg-[#2a2d2e] transition-colors"
        onClick={() => setIsCollapsed(!isCollapsed)}
      >
        <div className="flex items-center gap-2">
          <TerminalIcon className="size-4 text-[#1FF2AB]" />
          <span className="text-sm text-white">Terminal</span>
        </div>
        <div className="flex items-center gap-2">
          {!isCollapsed && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onClear();
              }}
              className="p-1 hover:bg-white/10 rounded transition-colors"
            >
              <X className="size-4 text-gray-400" />
            </button>
          )}
          <button className="p-1 hover:bg-white/10 rounded transition-colors">
            {isCollapsed ? (
              <ChevronUp className="size-4 text-gray-400" />
            ) : (
              <ChevronDown className="size-4 text-gray-400" />
            )}
          </button>
        </div>
      </div>
      {!isCollapsed && (
        <div className="h-[calc(100%-42px)] overflow-auto p-4 font-mono text-sm">
          {output.map((line, index) => (
            <div key={index} className="text-[#cccccc] mb-1">
              {line}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}