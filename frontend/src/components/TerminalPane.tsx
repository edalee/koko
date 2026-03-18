import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import { Terminal } from "@xterm/xterm";
import { useEffect, useRef } from "react";
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
  const onExitRef = useRef(onExit);
  onExitRef.current = onExit;

  // Use refs so handlers created once always access current values
  // without recreating the terminal when props change during reconnect.
  const sessionIdRef = useRef(sessionId);
  sessionIdRef.current = sessionId;
  const activeRef = useRef(active);
  activeRef.current = active;

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
    term.loadAddon(fit);
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

    // User input → PTY (uses ref so it always targets current session)
    const onDataDisposable = term.onData((data: string) => {
      Write(sessionIdRef.current, btoa(data));
    });

    const onBinaryDisposable = term.onBinary((data: string) => {
      const bytes = new Uint8Array(data.length);
      for (let i = 0; i < data.length; i++) {
        bytes[i] = data.charCodeAt(i);
      }
      Write(sessionIdRef.current, btoa(String.fromCharCode(...bytes)));
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

  return (
    <div
      ref={containerRef}
      className="h-full w-full p-1"
      style={{
        visibility: active ? "visible" : "hidden",
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
      }}
    />
  );
}
