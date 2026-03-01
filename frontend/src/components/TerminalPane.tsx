import { useEffect, useRef, useCallback } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  Write,
  Resize,
} from "../../wailsjs/go/main/TerminalManager";
import "@xterm/xterm/css/xterm.css";

interface TerminalPaneProps {
  sessionId: string;
  active: boolean;
  onExit?: () => void;
}

export default function TerminalPane({
  sessionId,
  active,
  onExit,
}: TerminalPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);

  const initTerminal = useCallback(() => {
    const container = containerRef.current;
    if (!container || termRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily:
        'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
      theme: {
        background: "#1F3860",
        foreground: "#C2E3EA",
        cursor: "#8A4460",
        selectionBackground: "#72869C40",
        black: "#1F3860",
        red: "#c4544e",
        green: "#57a86b",
        yellow: "#c4a14e",
        blue: "#5b8fc9",
        magenta: "#a86bc4",
        cyan: "#5bc4c4",
        white: "#C2E3EA",
        brightBlack: "#72869C",
        brightRed: "#d4746e",
        brightGreen: "#77c88b",
        brightYellow: "#d4c16e",
        brightBlue: "#7bafe9",
        brightMagenta: "#c88be4",
        brightCyan: "#7be4e4",
        brightWhite: "#ffffff",
      },
      allowProposedApi: true,
    });

    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());

    term.open(container);

    try {
      term.loadAddon(new WebglAddon());
    } catch {
      // WebGL not available
    }

    fit.fit();
    termRef.current = term;
    fitRef.current = fit;

    // PTY output → terminal
    const cleanupData = EventsOn(
      `pty:data:${sessionId}`,
      (encoded: string) => {
        const bytes = Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
        term.write(bytes);
      },
    );

    // PTY exit
    const cleanupExit = EventsOn(`pty:exit:${sessionId}`, () => {
      onExit?.();
    });

    // User input → PTY
    const onDataDisposable = term.onData((data: string) => {
      const encoded = btoa(data);
      Write(sessionId, encoded);
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
  }, [sessionId, onExit]);

  useEffect(() => {
    initTerminal();
    return () => {
      cleanupRef.current?.();
      cleanupRef.current = null;
    };
  }, [initTerminal]);

  // Refit when becoming active
  useEffect(() => {
    if (active && fitRef.current) {
      // Small delay to ensure container dimensions are settled
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
