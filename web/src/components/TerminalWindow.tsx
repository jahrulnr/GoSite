// TerminalWindow — floating, draggable, resizable, minimizable popup that
// hosts a TerminalPane. The window is mounted at the App root so it survives
// route changes. Closing the window leaves the underlying PTY session alive
// in the hub; reopening resumes it via the session id stored in
// localStorage by the terminalStore.
import { useEffect, useRef, useState } from 'preact/hooks';
import { TerminalPane } from './TerminalPane';
import { useTerminalStore } from '../lib/terminalStore';
import type { ConnectionState } from '../lib/terminalClient';
import { terminalApi } from '../api/endpoints';
import { IconClose, IconMinus, IconSquare, IconTerminal, IconTrash } from './Icons';

const RESIZE_DIRS = ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'] as const;
type ResizeDir = (typeof RESIZE_DIRS)[number];

export function TerminalWindow() {
  const store = useTerminalStore();
  const windowRef = useRef<HTMLDivElement | null>(null);
  const dragOrigin = useRef<{ x: number; y: number; startX: number; startY: number; startW: number; startH: number; dir: ResizeDir | null } | null>(null);
  const [statusOverride, setStatusOverride] = useState<ConnectionState | null>(null);

  useEffect(() => {
    if (!store.open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        // First Esc minimizes, second closes. This matches common
        // desktop terminal behavior.
        if (store.minimized) {
          store.closeTerminal();
        } else {
          store.setMinimized(true);
        }
      }
    };
    globalThis.addEventListener?.('keydown', onKey);
    return () => globalThis.removeEventListener?.('keydown', onKey);
  }, [store.open, store.minimized]);

  if (!store.open) return null;

  const onTitlePointerDown = (e: PointerEvent) => {
    if (store.maximized) return;
    if ((e.target as HTMLElement).closest('button')) return;
    dragOrigin.current = {
      x: e.clientX,
      y: e.clientY,
      startX: store.popup.x,
      startY: store.popup.y,
      startW: store.popup.w,
      startH: store.popup.h,
      dir: null,
    };
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  };

  const onResizePointerDown = (e: PointerEvent, dir: ResizeDir) => {
    if (store.maximized) return;
    e.stopPropagation();
    dragOrigin.current = {
      x: e.clientX,
      y: e.clientY,
      startX: store.popup.x,
      startY: store.popup.y,
      startW: store.popup.w,
      startH: store.popup.h,
      dir,
    };
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: PointerEvent) => {
    const o = dragOrigin.current;
    if (!o) return;
    if (o.dir === null) {
      const dx = e.clientX - o.x;
      const dy = e.clientY - o.y;
      store.setGeometry({ x: clamp(o.startX + dx, 0, globalThis.innerWidth - 120), y: clamp(o.startY + dy, 0, globalThis.innerHeight - 40) });
    } else {
      const dx = e.clientX - o.x;
      const dy = e.clientY - o.y;
      let x = o.startX;
      let y = o.startY;
      let w = o.startW;
      let h = o.startH;
      if (o.dir.includes('e')) w = Math.max(320, o.startW + dx);
      if (o.dir.includes('s')) h = Math.max(200, o.startH + dy);
      if (o.dir.includes('w')) {
        const newW = Math.max(320, o.startW - dx);
        x = o.startX + (o.startW - newW);
        w = newW;
      }
      if (o.dir.includes('n')) {
        const newH = Math.max(200, o.startH - dy);
        y = o.startY + (o.startH - newH);
        h = newH;
      }
      store.setGeometry({ x, y, w, h });
    }
  };

  const onPointerUp = () => {
    dragOrigin.current = null;
  };

  const stateAttr = store.minimized ? 'minimized' : store.maximized ? 'maximized' : 'normal';
  const status = statusOverride ?? (store.connection === 'connected' ? 'connected'
    : store.connection === 'reconnecting' ? 'reconnecting'
    : store.connection === 'error' ? 'error'
    : store.connection === 'connecting' ? 'connecting'
    : 'idle');

  return (
    <div
      class="terminal-window"
      data-state={stateAttr}
      ref={windowRef}
      style={{
        left: store.maximized ? undefined : `${store.popup.x}px`,
        top: store.maximized ? undefined : `${store.popup.y}px`,
        width: store.maximized ? undefined : `${store.popup.w}px`,
        height: store.minimized ? undefined : (store.maximized ? undefined : `${store.popup.h}px`),
      }}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
    >
      <div class="terminal-window__titlebar" onPointerDown={onTitlePointerDown}>
        <span class="terminal-window__title">
          <IconTerminal />
          <strong>Terminal</strong>
          <span class="terminal-window__role" data-role={store.role}>{store.role}</span>
        </span>
        <div class="terminal-window__actions">
          <button class="terminal-window__action" type="button" title="Minimize" onClick={() => store.setMinimized(true)}>
            <IconMinus />
          </button>
          <button class="terminal-window__action" type="button" title={store.maximized ? 'Restore' : 'Maximize'} onClick={() => store.setMaximized(!store.maximized)}>
            <IconSquare />
          </button>
          <button class="terminal-window__action" type="button" title="Kill session" onClick={async () => {
            const sid = store.activeSessionId;
            if (sid) {
              try {
                await terminalApi.kill(sid);
              } catch {
                // best effort
              }
            }
            store.closeTerminal();
            store.setSession('');
            setStatusOverride(null);
          }}>
            <IconTrash />
          </button>
          <button class="terminal-window__action" type="button" title="Close" onClick={() => store.closeTerminal()}>
            <IconClose />
          </button>
        </div>
      </div>

      <div class="terminal-window__layout" data-rail="false">
        <div class="terminal-window__body">
          {store.minimized ? null : (
            <TerminalPane
              sessionId={store.activeSessionId ?? undefined}
              onRoleChange={(role) => store.setRole(role)}
              onStatusChange={(s) => setStatusOverride(s)}
            />
          )}
          <div class="terminal-window__status" data-state={status}>
            {status}
            {store.activeSessionId ? ` · ${store.activeSessionId.slice(0, 8)}` : ''}
          </div>
        </div>
      </div>

      {store.minimized || store.maximized
        ? null
        : RESIZE_DIRS.map((d) => (
            <span
              key={d}
              class="terminal-window__resize"
              data-dir={d}
              onPointerDown={(e) => onResizePointerDown(e, d)}
            />
          ))}
    </div>
  );
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}
