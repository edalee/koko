import { ChevronDown, ChevronRight, File, Folder, FolderOpen } from "lucide-react";
import { useState } from "react";

export type FileNode = {
  name: string;
  type: "file" | "folder";
  children?: FileNode[];
  content?: string;
  language?: string;
};

type FileExplorerProps = {
  files: FileNode[];
  onFileSelect: (file: FileNode) => void;
  selectedFile: FileNode | null;
};

export function FileExplorer({ files, onFileSelect, selectedFile }: FileExplorerProps) {
  return (
    <div className="h-full bg-[#1e1e1e] text-[#cccccc] overflow-auto">
      <div className="px-4 py-3 text-xs uppercase tracking-wider text-[#888888]">
        Explorer
      </div>
      <div className="px-2">
        {files.map((file, index) => (
          <FileTreeNode
            key={index}
            node={file}
            onFileSelect={onFileSelect}
            selectedFile={selectedFile}
            depth={0}
          />
        ))}
      </div>
    </div>
  );
}

type FileTreeNodeProps = {
  node: FileNode;
  onFileSelect: (file: FileNode) => void;
  selectedFile: FileNode | null;
  depth: number;
};

function FileTreeNode({ node, onFileSelect, selectedFile, depth }: FileTreeNodeProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const isSelected = selectedFile?.name === node.name;

  const handleClick = () => {
    if (node.type === "folder") {
      setIsExpanded(!isExpanded);
    } else {
      onFileSelect(node);
    }
  };

  return (
    <div>
      <div
        className={`flex items-center gap-1 px-2 py-1 cursor-pointer hover:bg-[#2a2d2e] rounded ${
          isSelected ? "bg-[#37373d]" : ""
        }`}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        onClick={handleClick}
      >
        {node.type === "folder" ? (
          <>
            {isExpanded ? (
              <ChevronDown className="size-4 shrink-0" />
            ) : (
              <ChevronRight className="size-4 shrink-0" />
            )}
            {isExpanded ? (
              <FolderOpen className="size-4 shrink-0 text-[#dcb67a]" />
            ) : (
              <Folder className="size-4 shrink-0 text-[#dcb67a]" />
            )}
          </>
        ) : (
          <>
            <div className="size-4 shrink-0" />
            <File className="size-4 shrink-0 text-[#519aba]" />
          </>
        )}
        <span className="text-sm truncate">{node.name}</span>
      </div>
      {node.type === "folder" && isExpanded && node.children && (
        <div>
          {node.children.map((child, index) => (
            <FileTreeNode
              key={index}
              node={child}
              onFileSelect={onFileSelect}
              selectedFile={selectedFile}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}
