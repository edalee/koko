import { X } from "lucide-react";
import { FileNode } from "./FileExplorer";

type TabBarProps = {
  openFiles: FileNode[];
  activeFile: FileNode | null;
  onTabClick: (file: FileNode) => void;
  onTabClose: (file: FileNode) => void;
};

export function TabBar({ 
  openFiles, 
  activeFile, 
  onTabClick, 
  onTabClose,
}: TabBarProps) {
  if (openFiles.length === 0) {
    return null;
  }

  return (
    <div className="flex bg-[#252526] border-b border-[#1e1e1e] overflow-x-auto">
      {openFiles.map((file, index) => (
        <div
          key={`file-${index}`}
          className={`flex items-center gap-2 px-3 py-2 border-r border-[#1e1e1e] cursor-pointer hover:bg-[#2a2d2e] min-w-0 ${
            activeFile?.name === file.name ? "bg-[#1e1e1e]" : ""
          }`}
          onClick={() => onTabClick(file)}
        >
          <span className="text-sm text-[#cccccc] truncate max-w-[150px]">
            {file.name}
          </span>
          <button
            className="shrink-0 hover:bg-[#3e3e42] rounded p-0.5"
            onClick={(e) => {
              e.stopPropagation();
              onTabClose(file);
            }}
          >
            <X className="size-3 text-[#cccccc]" />
          </button>
        </div>
      ))}
    </div>
  );
}