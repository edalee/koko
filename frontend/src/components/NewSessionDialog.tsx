import { Folder, FolderOpen, Plus, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { PickDirectory } from "../../wailsjs/go/main/App";
import { randomName } from "../lib/names";

const RECENT_DIRS_KEY = "koko:recent-dirs";
const MAX_RECENT = 5;

function getRecentDirs(): string[] {
  try {
    return JSON.parse(localStorage.getItem(RECENT_DIRS_KEY) || "[]");
  } catch {
    return [];
  }
}

export function addRecentDir(dir: string) {
  const dirs = getRecentDirs().filter((d) => d !== dir);
  dirs.unshift(dir);
  localStorage.setItem(RECENT_DIRS_KEY, JSON.stringify(dirs.slice(0, MAX_RECENT)));
}

function shortenPath(path: string): string {
  const home = path.match(/^\/Users\/[^/]+/)?.[0];
  if (home) return path.replace(home, "~");
  return path;
}

interface NewSessionDialogProps {
  open: boolean;
  onClose: () => void;
  onCreate: (name: string, directory: string) => void;
}

type AnimState = "closed" | "open" | "closing";

export default function NewSessionDialog({ open, onClose, onCreate }: NewSessionDialogProps) {
  const [state, setState] = useState<AnimState>("closed");
  const placeholder = useMemo(() => randomName(), []);
  const [name, setName] = useState("");
  const [directory, setDirectory] = useState("");
  const [recentDirs] = useState(getRecentDirs);
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open && state === "closed") {
      setState("open");
      setName("");
      setDirectory("");
      setTimeout(() => nameRef.current?.focus(), 50);
    } else if (!open && state === "open") {
      setState("closing");
    }
  }, [open, state]);

  function handleAnimationEnd() {
    if (state === "closing") {
      setState("closed");
    }
  }

  const handleBrowse = useCallback(async () => {
    const dir = await PickDirectory();
    if (dir) setDirectory(dir);
  }, []);

  const handleCreate = useCallback(() => {
    if (!directory) return;
    const sessionName = name.trim() || placeholder;
    onCreate(sessionName, directory);
  }, [name, directory, placeholder, onCreate]);

  useEffect(() => {
    if (state !== "open") return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        onClose();
      } else if (e.key === "Enter" && directory) {
        handleCreate();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state, directory, onClose, handleCreate]);

  if (state === "closed") return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents lint/a11y/noStaticElementInteractions: backdrop */}
      <div
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm ${
          state === "closing" ? "animate-backdrop-out" : "animate-backdrop-in"
        }`}
        onClick={onClose}
      />

      <div
        className={`relative w-[480px] flex flex-col rounded-2xl border shadow-2xl ${
          state === "closing" ? "animate-overlay-out" : "animate-overlay-in"
        }`}
        style={{
          backgroundColor: "var(--color-glass)",
          borderColor: "var(--color-glass-border)",
          backdropFilter: "blur(40px)",
          WebkitBackdropFilter: "blur(40px)",
        }}
        onAnimationEnd={handleAnimationEnd}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/[0.08]">
          <div className="flex items-center gap-2.5">
            <span className="text-accent">
              <Plus className="size-4" />
            </span>
            <h2 className="text-white text-sm font-medium">New Session</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Content */}
        <div className="p-5 space-y-5">
          {/* Name input */}
          <div className="space-y-2">
            <label
              className="text-xs font-medium text-muted-foreground uppercase tracking-wider"
              htmlFor="session-name"
            >
              Session Name
            </label>
            <input
              ref={nameRef}
              id="session-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={placeholder}
              className="w-full px-3 py-2.5 text-sm bg-white/5 border border-border rounded-lg text-white placeholder:text-muted-foreground/40 outline-none focus:border-accent/50 transition-colors"
            />
          </div>

          {/* Directory picker */}
          <div className="space-y-2">
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
              Project Directory
            </span>

            {directory ? (
              <button
                type="button"
                onClick={handleBrowse}
                className="w-full flex items-center gap-3 px-3 py-2.5 text-sm bg-accent/10 border border-accent/30 rounded-lg text-white hover:bg-accent/15 transition-colors text-left"
              >
                <FolderOpen className="size-4 text-accent shrink-0" />
                <span className="truncate">{shortenPath(directory)}</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={handleBrowse}
                className="w-full flex items-center justify-center gap-2 px-3 py-3 text-sm bg-white/5 border border-border border-dashed rounded-lg text-muted-foreground hover:text-white hover:border-accent/30 hover:bg-white/[0.07] transition-colors"
              >
                <Folder className="size-4" />
                <span>Browse...</span>
              </button>
            )}

            {/* Recent directories */}
            {recentDirs.length > 0 && !directory && (
              <div className="space-y-1 pt-1">
                <span className="text-xs text-muted-foreground/60 px-1">Recent</span>
                {recentDirs.map((dir) => (
                  <button
                    type="button"
                    key={dir}
                    onClick={() => setDirectory(dir)}
                    className="w-full flex items-center gap-2.5 px-3 py-2 text-sm rounded-lg text-muted-foreground hover:text-white hover:bg-white/5 transition-colors text-left"
                  >
                    <Folder className="size-3.5 shrink-0" />
                    <span className="truncate">{shortenPath(dir)}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="px-5 py-4 border-t border-white/[0.08] flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm rounded-lg text-muted-foreground hover:text-white hover:bg-white/10 transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleCreate}
            disabled={!directory}
            className="px-4 py-2 text-sm rounded-lg font-medium transition-all disabled:opacity-30 disabled:cursor-not-allowed text-white relative overflow-hidden border-2 border-transparent"
            style={{
              background: directory
                ? "linear-gradient(var(--color-surface), var(--color-surface)) padding-box, linear-gradient(to right, #1FF2AB, #24A965) border-box"
                : undefined,
            }}
          >
            Create Session
          </button>
        </div>
      </div>
    </div>
  );
}
