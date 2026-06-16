// TerminalPane — xterm.js view + WebSocket pump wiring.
//
// The pane owns the xterm instance, the FitAddon, and a single TerminalClient.
// It is mounted by TerminalWindow and torn down when the window closes (the
// underlying PTY session in the hub continues to run regardless).
import { useEffect, useRef } from 'preact/hooks';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';
import { TerminalClient, type TerminalTextFrame, type ConnectionState } from '../lib/terminalClient';
import { useTerminalStore } from '../lib/terminalStore';
import { useStore } from '../lib/store';

export interface TerminalPaneProps {
  sessionId?: string;
  onRoleChange?: (role: 'writer' | 'reader' | 'none') => void;
  onStatusChange?: (state: ConnectionState) => void;
}

export function TerminalPane({ sessionId, onRoleChange, onStatusChange }: Readonly<TerminalPaneProps>) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const clientRef = useRef<TerminalClient | null>(null);
  const store = useTerminalStore();
  const { meta } = useStore();

  useEffect(() => {
    if (!containerRef.current) return;
    const term = new Terminal({
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 13,
      cursorBlink: true,
      convertEol: true,
      scrollback: 5000,
      // Start in read-only mode. The `onRole` callback below flips this
      // to false once the server confirms the writer role via the
      // `ready` frame, so the user can never type into an unattached
      // or read-only shell session.
      disableStdin: true,
      theme: {
        background: '#0e0f12',
        foreground: '#e7e9ee',
        cursor: '#a4d8ff',
        selectionBackground: '#264f78',
        black: '#0e0f12',
        red: '#e06c75',
        green: '#98c379',
        yellow: '#e5c07b',
        blue: '#61afef',
        magenta: '#c678dd',
        cyan: '#56b6c2',
        white: '#dcdfe4',
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    term.open(containerRef.current);
    try {
      fit.fit();
    } catch {
      // best effort
    }
    termRef.current = term;
    fitRef.current = fit;

    const client = new TerminalClient({
      sessionId,
      cols: term.cols,
      rows: term.rows,
      onData: (chunk) => {
        term.write(chunk);
      },
      onText: (frame: TerminalTextFrame) => {
        if (frame.type === 'exit') {
          term.writeln(`\r\n[session exited with code ${frame.code}]`);
        } else if (frame.type === 'error') {
          term.writeln(`\r\n[error] ${frame.message}`);
        }
      },
      onRole: (role) => {
        store.setRole(role);
        onRoleChange?.(role);
        if (role !== 'writer') {
          term.options.disableStdin = true;
        } else {
          term.options.disableStdin = false;
        }
      },
      onConnectionChange: (state) => {
        store.setConnection(state);
        onStatusChange?.(state);
      },
      onError: (msg) => {
        store.setLastError(msg);
      },
    });
    clientRef.current = client;
    client.start();

    const onDataDisposable = term.onData((data) => {
      const enc = new TextEncoder();
      client.sendInput(enc.encode(data));
    });
    const onResizeDisposable = term.onResize(({ cols, rows }) => {
      client.sendResize(cols, rows);
    });

    const resizeObserver = new ResizeObserver(() => {
      try {
        fit.fit();
        client.sendResize(term.cols, term.rows);
      } catch {
        // ignore
      }
    });
    resizeObserver.observe(containerRef.current);

    // Expose app name into the prompt header once ready.
    if (meta?.app?.name) {
      term.writeln(`\x1b[2m${meta.app.name} — terminal\x1b[0m`);
    }

    return () => {
      onDataDisposable.dispose();
      onResizeDisposable.dispose();
      resizeObserver.disconnect();
      client.close();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      clientRef.current = null;
    };
  }, [sessionId, meta?.app?.name]);

  return <div class="terminal-window__pane" ref={containerRef} />;
}
