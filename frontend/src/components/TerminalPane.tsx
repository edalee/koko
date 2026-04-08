import { FitAddon } from "@xterm/addon-fit";
import { SearchAddon } from "@xterm/addon-search";
import { SerializeAddon } from "@xterm/addon-serialize";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import { Terminal } from "@xterm/xterm";
import { ChevronDown, ChevronUp, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { ReplayBuffer, Resize, Write } from "../../wailsjs/go/main/TerminalManager";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";
import "@xterm/xterm/css/xterm.css";

interface TerminalPaneProps {
  sessionId: string;
  active: boolean;
  onExit?: () => void;
}

export default function TerminalPane({ sessionId, active, onExit }: TerminalPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const serializeRef = useRef<SerializeAddon | null>(null);
  const searchRef = useRef<SearchAddon | null>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const onExitRef = useRef(onExit);
  onExitRef.current = onExit;

  // Use refs so handlers created once always access current values
  // without recreating the terminal when props change during reconnect.
  const sessionIdRef = useRef(sessionId);
  sessionIdRef.current = sessionId;
  const activeRef = useRef(active);
  activeRef.current = active;

  // Context menu state
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number } | null>(null);

  // Search state
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResult, setSearchResult] = useState<string | null>(null);

  const openSearch = useCallback(() => {
    setSearchOpen(true);
    setTimeout(() => searchInputRef.current?.focus(), 0);
  }, []);

  const closeSearch = useCallback(() => {
    setSearchOpen(false);
    setSearchQuery("");
    setSearchResult(null);
    searchRef.current?.clearDecorations();
    termRef.current?.focus();
  }, []);

  const doSearch = useCallback((query: string, direction: "next" | "prev" = "next") => {
    const addon = searchRef.current;
    if (!addon || !query) {
      setSearchResult(null);
      addon?.clearDecorations();
      return;
    }
    const opts = { regex: false, caseSensitive: false, incremental: direction === "next" };
    const found =
      direction === "next" ? addon.findNext(query, opts) : addon.findPrevious(query, opts);
    setSearchResult(found ? null : "No results");
  }, []);

  const copyPlainText = useCallback(() => {
    const term = termRef.current;
    const serialize = serializeRef.current;
    if (!term?.hasSelection() || !serialize) return;
    const plainText = cleanTerminalText(term.getSelection());
    const html = serialize.serializeAsHTML({ onlySelection: true, includeGlobalBackground: false });
    navigator.clipboard.write([
      new ClipboardItem({
        "text/plain": new Blob([plainText], { type: "text/plain" }),
        "text/html": new Blob([html], { type: "text/html" }),
      }),
    ]);
  }, []);

  const copyAsMarkdown = useCallback(() => {
    const term = termRef.current;
    const serialize = serializeRef.current;
    if (!term?.hasSelection() || !serialize) return;
    const html = serialize.serializeAsHTML({ onlySelection: true });
    navigator.clipboard.writeText(htmlToMarkdown(html));
  }, []);

  // Create terminal once (stable across reconnects)
  useEffect(() => {
    const container = containerRef.current;
    if (!container || termRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      scrollback: 5000,
      scrollOnUserInput: true,
      smoothScrollDuration: 0,
      fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
      theme: {
        background: "#0f1117",
        foreground: "#ebebeb",
        cursor: "#1FF2AB",
        selectionBackground: "rgba(255, 255, 255, 0.1)",
        black: "#0f1117",
        red: "#f44747",
        green: "#6a9955",
        yellow: "#d7ba7d",
        blue: "#569cd6",
        magenta: "#c586c0",
        cyan: "#4ec9b0",
        white: "#cccccc",
        brightBlack: "#808080",
        brightRed: "#f44747",
        brightGreen: "#6a9955",
        brightYellow: "#d7ba7d",
        brightBlue: "#569cd6",
        brightMagenta: "#c586c0",
        brightCyan: "#4ec9b0",
        brightWhite: "#ffffff",
      },
      allowProposedApi: true,
    });

    const fit = new FitAddon();
    const serialize = new SerializeAddon();
    const search = new SearchAddon();
    term.loadAddon(fit);
    term.loadAddon(serialize);
    term.loadAddon(search);
    term.loadAddon(
      new WebLinksAddon((_event, uri) => {
        BrowserOpenURL(uri);
      }),
    );

    term.open(container);

    try {
      const webgl = new WebglAddon();
      webgl.onContextLoss(() => webgl.dispose());
      term.loadAddon(webgl);
    } catch {
      // WebGL not available, DOM renderer is fine
    }

    fit.fit();
    termRef.current = term;
    fitRef.current = fit;
    serializeRef.current = serialize;
    searchRef.current = search;

    // --- Copy handlers ---
    // Cmd+C: copy selection as trimmed plain text + HTML for rich paste
    // Cmd+Shift+C: copy selection as Markdown
    term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.type === "keydown" && e.metaKey && e.shiftKey && e.code === "KeyC") {
        copyAsMarkdown();
        return false;
      }
      if (e.type === "keydown" && e.metaKey && !e.shiftKey && e.code === "KeyF") {
        openSearch();
        return false;
      }
      return true;
    });

    const onCopy = (e: ClipboardEvent) => {
      if (!term.hasSelection()) return;
      e.preventDefault();
      const plainText = cleanTerminalText(term.getSelection());
      const html = serialize.serializeAsHTML({
        onlySelection: true,
        includeGlobalBackground: false,
      });
      e.clipboardData?.setData("text/plain", plainText);
      e.clipboardData?.setData("text/html", html);
    };
    container.addEventListener("copy", onCopy);

    // Right-click context menu
    const onContextMenu = (e: MouseEvent) => {
      e.preventDefault();
      setCtxMenu({ x: e.clientX, y: e.clientY });
    };
    container.addEventListener("contextmenu", onContextMenu);

    // User input → PTY (uses ref so it always targets current session)
    // Uses TextEncoder for UTF-8 safety — btoa() fails on characters > U+00FF
    // (e.g. em dashes, smart quotes, non-Latin text).
    const onDataDisposable = term.onData((data: string) => {
      Write(sessionIdRef.current, utf8ToBase64(data));
    });

    const onBinaryDisposable = term.onBinary((data: string) => {
      const bytes = new Uint8Array(data.length);
      for (let i = 0; i < data.length; i++) {
        bytes[i] = data.charCodeAt(i);
      }
      Write(sessionIdRef.current, uint8ToBase64(bytes));
    });

    // Resize — debounce to avoid rapid SIGWINCH during drag.
    let resizeTimer: ReturnType<typeof setTimeout> | null = null;
    const observer = new ResizeObserver(() => {
      if (!activeRef.current) return; // skip hidden tabs
      if (resizeTimer) clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => {
        if (fitRef.current && termRef.current) {
          const dims = fitRef.current.proposeDimensions();
          if (dims && (dims.cols !== termRef.current.cols || dims.rows !== termRef.current.rows)) {
            fitRef.current.fit();
            Resize(sessionIdRef.current, dims.cols, dims.rows);
          }
        }
      }, 50);
    });
    observer.observe(container);

    return () => {
      container.removeEventListener("copy", onCopy);
      container.removeEventListener("contextmenu", onContextMenu);
      onDataDisposable.dispose();
      onBinaryDisposable.dispose();
      if (resizeTimer) clearTimeout(resizeTimer);
      observer.disconnect();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
  }, []); // stable — never recreated

  // Subscribe to PTY events — re-subscribes when sessionId changes (reconnect).
  useEffect(() => {
    const term = termRef.current;
    if (!term) return;

    const cleanupData = EventsOn(`pty:data:${sessionId}`, (encoded: string) => {
      const bytes = Uint8Array.from(atob(encoded), (c: string) => c.charCodeAt(0));
      term.write(bytes);
    });

    const cleanupExit = EventsOn(`pty:exit:${sessionId}`, () => {
      term.writeln("\r\n\x1b[31m[Session ended]\x1b[0m");
      onExitRef.current?.();
    });

    // Initial resize for this session
    if (fitRef.current) {
      const dims = fitRef.current.proposeDimensions();
      if (dims) {
        Resize(sessionId, dims.cols, dims.rows);
      }
    }

    // Replay buffered output from this session
    ReplayBuffer(sessionId).then((encoded) => {
      if (encoded) {
        const bytes = Uint8Array.from(atob(encoded), (c: string) => c.charCodeAt(0));
        term.write(bytes);
      }
    });

    return () => {
      cleanupData();
      cleanupExit();
    };
  }, [sessionId]);

  // Focus terminal when becoming active.
  useEffect(() => {
    if (active && termRef.current) {
      termRef.current.focus();
    }
  }, [active]);

  // Close context menu on any click or keypress
  useEffect(() => {
    if (!ctxMenu) return;
    const dismiss = () => setCtxMenu(null);
    document.addEventListener("click", dismiss);
    document.addEventListener("keydown", dismiss);
    return () => {
      document.removeEventListener("click", dismiss);
      document.removeEventListener("keydown", dismiss);
    };
  }, [ctxMenu]);

  const hasSelection = termRef.current?.hasSelection() ?? false;

  return (
    <div
      className="h-full w-full"
      style={{
        visibility: active ? "visible" : "hidden",
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
      }}
    >
      <div ref={containerRef} className="h-full w-full p-1" />
      {searchOpen && active && (
        <div className="absolute top-2 right-4 z-40 flex items-center gap-1.5 rounded-lg border border-white/[0.08] bg-[rgba(15,17,23,0.95)] px-2 py-1.5 shadow-lg">
          <input
            ref={searchInputRef}
            type="text"
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              doSearch(e.target.value);
            }}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                closeSearch();
              } else if (e.key === "Enter" && e.shiftKey) {
                doSearch(searchQuery, "prev");
              } else if (e.key === "Enter") {
                doSearch(searchQuery, "next");
              }
            }}
            placeholder="Search..."
            className="w-48 bg-transparent text-xs text-white outline-none placeholder:text-white/30"
          />
          {searchResult && <span className="text-[10px] text-red-400">{searchResult}</span>}
          <button
            type="button"
            onClick={() => doSearch(searchQuery, "prev")}
            className="rounded p-0.5 text-white/50 hover:bg-white/[0.06] hover:text-white"
            title="Previous (Shift+Enter)"
          >
            <ChevronUp className="size-3.5" />
          </button>
          <button
            type="button"
            onClick={() => doSearch(searchQuery, "next")}
            className="rounded p-0.5 text-white/50 hover:bg-white/[0.06] hover:text-white"
            title="Next (Enter)"
          >
            <ChevronDown className="size-3.5" />
          </button>
          <button
            type="button"
            onClick={closeSearch}
            className="rounded p-0.5 text-white/50 hover:bg-white/[0.06] hover:text-white"
            title="Close (Esc)"
          >
            <X className="size-3.5" />
          </button>
        </div>
      )}
      {ctxMenu && active && (
        <ContextMenu
          x={ctxMenu.x}
          y={ctxMenu.y}
          hasSelection={hasSelection}
          onCopy={() => {
            copyPlainText();
            setCtxMenu(null);
          }}
          onCopyMarkdown={() => {
            copyAsMarkdown();
            setCtxMenu(null);
          }}
          onPaste={async () => {
            const text = await navigator.clipboard.readText();
            if (text && termRef.current) Write(sessionIdRef.current, utf8ToBase64(text));
            setCtxMenu(null);
          }}
          onSelectAll={() => {
            termRef.current?.selectAll();
            setCtxMenu(null);
          }}
          onClear={() => {
            termRef.current?.clear();
            setCtxMenu(null);
          }}
        />
      )}
    </div>
  );
}

// --- Context menu ---

interface CtxMenuProps {
  x: number;
  y: number;
  hasSelection: boolean;
  onCopy: () => void;
  onCopyMarkdown: () => void;
  onPaste: () => void;
  onSelectAll: () => void;
  onClear: () => void;
}

function ContextMenu({
  x,
  y,
  hasSelection,
  onCopy,
  onCopyMarkdown,
  onPaste,
  onSelectAll,
  onClear,
}: CtxMenuProps) {
  const items: {
    label: string;
    shortcut?: string;
    action: () => void;
    disabled?: boolean;
    separator?: boolean;
  }[] = [
    { label: "Copy", shortcut: "⌘C", action: onCopy, disabled: !hasSelection },
    { label: "Copy as Markdown", shortcut: "⇧⌘C", action: onCopyMarkdown, disabled: !hasSelection },
    { label: "Paste", shortcut: "⌘V", action: onPaste },
    { label: "---", action: () => {}, separator: true },
    { label: "Select All", shortcut: "⌘A", action: onSelectAll },
    { label: "Clear Terminal", action: onClear },
  ];

  return (
    <div
      className="fixed z-50 min-w-[180px] rounded-lg border border-glass-border bg-[rgba(15,17,23,0.92)] glass-overlay shadow-xl py-1 text-[12px]"
      style={{ left: x, top: y }}
    >
      {items.map((item) =>
        item.separator ? (
          <div key={item.label} className="my-1 border-t border-white/[0.06]" />
        ) : (
          <button
            key={item.label}
            type="button"
            disabled={item.disabled}
            className="flex w-full items-center px-3 py-1.5 text-left text-white/80 hover:bg-white/[0.06] disabled:opacity-30 disabled:pointer-events-none"
            onMouseDown={(e) => {
              e.preventDefault();
              item.action();
            }}
          >
            {item.label}
            {item.shortcut && (
              <span className="ml-auto pl-4 text-[10px] text-white/30">{item.shortcut}</span>
            )}
          </button>
        ),
      )}
    </div>
  );
}

/** Clean terminal text: trim trailing whitespace and collapse runs of 2+ spaces
 *  (xterm pads lines to the terminal width with spaces). */
function cleanTerminalText(text: string): string {
  return text
    .split("\n")
    .map((line) => line.trimEnd().replace(/ {2,}/g, " "))
    .join("\n");
}

/** Convert xterm HTML output to Markdown. */
function htmlToMarkdown(html: string): string {
  const doc = new DOMParser().parseFromString(html, "text/html");
  const lines: string[] = [];

  function walk(node: Node): string {
    if (node.nodeType === Node.TEXT_NODE) {
      return node.textContent ?? "";
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return "";

    const el = node as HTMLElement;
    const tag = el.tagName.toLowerCase();

    // Collect children text
    let inner = "";
    for (const child of el.childNodes) {
      inner += walk(child);
    }

    // Map terminal styling to Markdown
    const style = el.getAttribute("style") ?? "";
    const isBold = style.includes("font-weight: bold") || tag === "b" || tag === "strong";
    const isItalic = style.includes("font-style: italic") || tag === "i" || tag === "em";

    if (isBold && isItalic) return `***${inner}***`;
    if (isBold) return `**${inner}**`;
    if (isItalic) return `*${inner}*`;

    // Line breaks
    if (tag === "br") return "\n";
    if (tag === "div" || tag === "p") {
      lines.push(inner);
      return "";
    }

    return inner;
  }

  const body = doc.body;
  let remaining = "";
  for (const child of body.childNodes) {
    remaining += walk(child);
  }
  if (remaining) lines.push(remaining);

  return lines
    .map((line) => line.trimEnd())
    .join("\n")
    .trim();
}

/** Encode a UTF-8 string to base64 (safe for Unicode — btoa fails on chars > U+00FF). */
function utf8ToBase64(str: string): string {
  const bytes = new TextEncoder().encode(str);
  return uint8ToBase64(bytes);
}

/** Encode a Uint8Array to base64. */
function uint8ToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}
