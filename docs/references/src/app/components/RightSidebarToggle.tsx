import { ChevronLeft, ChevronRight } from "lucide-react";

type RightSidebarToggleProps = {
  isOpen: boolean;
  onToggle: () => void;
};

export function RightSidebarToggle({ isOpen, onToggle }: RightSidebarToggleProps) {
  return (
    <div className="h-full w-12 bg-[#252526] border-l border-white/10 flex flex-col items-center justify-end pb-3">
      <button
        onClick={onToggle}
        className="p-1.5 hover:bg-white/10 rounded transition-colors"
        title={isOpen ? "Collapse sidebar" : "Expand sidebar"}
      >
        {isOpen ? (
          <ChevronRight className="size-4 text-gray-400" />
        ) : (
          <ChevronLeft className="size-4 text-gray-400" />
        )}
      </button>
    </div>
  );
}
