import { Highlight, themes } from "prism-react-renderer";
import { FileNode } from "./FileExplorer";

type CodeEditorProps = {
  file: FileNode | null;
  onCodeChange: (code: string) => void;
};

export function CodeEditor({ file, onCodeChange }: CodeEditorProps) {
  if (!file || file.type !== "file") {
    return (
      <div className="h-full bg-[#1e1e1e] flex items-center justify-center text-[#888888]">
        <div className="text-center">
          <p className="text-lg">No file selected</p>
          <p className="text-sm mt-2">Select a file from the explorer to start editing</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full bg-[#1e1e1e] overflow-auto">
      <div className="min-h-full">
        <Highlight
          theme={themes.vsDark}
          code={file.content || ""}
          language={file.language || "javascript"}
        >
          {({ className, style, tokens, getLineProps, getTokenProps }) => (
            <pre className={className} style={{ ...style, margin: 0, padding: "16px" }}>
              {tokens.map((line, i) => (
                <div key={i} {...getLineProps({ line })} className="table-row">
                  <span className="table-cell text-right pr-4 select-none opacity-50 text-[#858585]">
                    {i + 1}
                  </span>
                  <span className="table-cell">
                    {line.map((token, key) => (
                      <span key={key} {...getTokenProps({ token })} />
                    ))}
                  </span>
                </div>
              ))}
            </pre>
          )}
        </Highlight>
      </div>
    </div>
  );
}
