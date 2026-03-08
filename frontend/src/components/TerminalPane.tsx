import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import { Terminal } from "@xterm/xterm";
import { useEffect, useRef } from "react";
import { Resize, Write } from "../../wailsjs/go/main/TerminalManager";
import { EventsOn } from "../../wailsjs/runtime/runtime";
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
  const cleanupRef = useRef<(() => void) | null>(null);
  const onExitRef = useRef(onExit);
  onExitRef.current = onExit;

  // Initialize terminal once per sessionId — onExit is accessed via ref
  // to avoid re-creating the terminal on every parent render.
  useEffect(() => {
    const container = containerRef.current;
    if (!container || termRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
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
    term.loadAddon(new WebLinksAddon());

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

    // PTY output → terminal
    const cleanupData = EventsOn(`pty:data:${sessionId}`, (encoded: string) => {
      const bytes = Uint8Array.from(atob(encoded), (c: string) => c.charCodeAt(0));
      term.write(bytes);
    });

    // PTY exit
    const cleanupExit = EventsOn(`pty:exit:${sessionId}`, () => {
      term.writeln("\r\n\x1b[31m[Session ended]\x1b[0m");
      onExitRef.current?.();
    });

    // User input → PTY
    const onDataDisposable = term.onData((data: string) => {
      Write(sessionId, btoa(data));
    });

    // Binary data → PTY
    const onBinaryDisposable = term.onBinary((data: string) => {
      const bytes = new Uint8Array(data.length);
      for (let i = 0; i < data.length; i++) {
        bytes[i] = data.charCodeAt(i);
      }
      Write(sessionId, btoa(String.fromCharCode(...bytes)));
    });

    // Resize
    const observer = new ResizeObserver(() => {
      if (fitRef.current) {
        fitRef.current.fit();
        const dims = fitRef.current.proposeDimensions();
        if (dims) {
          Resize(sessionId, dims.cols, dims.rows);
        }
      }
    });
    observer.observe(container);

    // Initial resize
    const dims = fit.proposeDimensions();
    if (dims) {
      Resize(sessionId, dims.cols, dims.rows);
    }

    cleanupRef.current = () => {
      cleanupData();
      cleanupExit();
      onDataDisposable.dispose();
      onBinaryDisposable.dispose();
      observer.disconnect();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };

    return () => {
      cleanupRef.current?.();
      cleanupRef.current = null;
    };
  }, [sessionId]);

  // Refit when becoming active
  useEffect(() => {
    if (active && fitRef.current) {
      requestAnimationFrame(() => {
        fitRef.current?.fit();
      });
    }
  }, [active]);

  // Focus terminal when active
  useEffect(() => {
    if (active && termRef.current) {
      termRef.current.focus();
    }
  }, [active]);

  return (
    <div
      ref={containerRef}
      className="h-full w-full p-1"
      style={{ display: active ? "block" : "none" }}
    />
  );
}
