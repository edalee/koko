import { Terminal as TerminalIcon } from "lucide-react";

type TerminalProps = {
  output: string[];
};

export function Terminal({ output }: TerminalProps) {
  return (
    <div className="h-full bg-[#1e1e1e] flex flex-col">
      <div className="flex items-center gap-2 px-4 py-2 bg-[#252526] border-b border-[#1e1e1e]">
        <TerminalIcon className="size-4 text-[#cccccc]" />
        <span className="text-sm text-[#cccccc]">Terminal</span>
      </div>
      <div className="flex-1 overflow-auto p-4 font-mono text-sm">
        {output.map((line, index) => (
          <div key={index} className="text-[#cccccc] mb-1">
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}
