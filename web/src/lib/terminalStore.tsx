// Floating terminal store.
//
// State machine for the topbar-popup xterm window:
//   - idle: the popup is closed. The active session id (if any) is persisted
//     in localStorage so a subsequent click can resume it.
//   - open: the popup is visible. Exactly one session is active per window
//     and is either the resumed id (if the backend still has it) or a new
//     id chosen at open time.
//
// Resume is *only* triggered when the user clicks the topbar icon — never
// on page load. The store validates the persisted id against the backend
// `GET /terminal/sessions` endpoint and falls back to a new session if the
// persisted one has been swept or never existed.
import { createContext } from 'preact';
import { useCallback, useContext, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { ComponentChildren } from 'preact';
import { terminalApi, type TerminalSession } from '../api/endpoints';
import type { ConnectionState } from './terminalClient';

export type { ConnectionState } from './terminalClient';

const STORAGE_KEY = 'gosite.terminal.v1';
const STORAGE_VERSION = 1 as const;

export interface TerminalPopupGeometry {
  x: number;
  y: number;
  w: number;
  h: number;
  maximized: boolean;
  minimized: boolean;
}

export interface TerminalStoreState {
  open: boolean;
  minimized: boolean;
  maximized: boolean;
  activeSessionId: string | null;
  role: 'writer' | 'reader' | 'none';
  connection: ConnectionState;
  sessions: TerminalSession[];
  popup: TerminalPopupGeometry;
  lastError: string | null;
}

export interface TerminalStore extends TerminalStoreState {
  openTerminal: (sessionId?: string) => Promise<void>;
  closeTerminal: () => void;
  toggleTerminal: (sessionId?: string) => Promise<void>;
  setSession: (id: string) => void;
  setRole: (role: TerminalStoreState['role']) => void;
  setConnection: (state: TerminalStoreState['connection']) => void;
  setLastError: (msg: string | null) => void;
  setGeometry: (g: Partial<TerminalPopupGeometry>) => void;
  setMinimized: (v: boolean) => void;
  setMaximized: (v: boolean) => void;
  refreshSessions: () => Promise<void>;
}

const Ctx = createContext<TerminalStore | null>(null);

const DEFAULT_GEOMETRY: TerminalPopupGeometry = {
  x: 80,
  y: 80,
  w: 720,
  h: 420,
  maximized: false,
  minimized: false,
};

interface PersistedState {
  v: typeof STORAGE_VERSION;
  popup: TerminalPopupGeometry;
  activeSessionId: string | null;
}

function loadPersisted(): PersistedState {
  try {
    const raw = globalThis.localStorage?.getItem(STORAGE_KEY);
    if (!raw) return defaultPersisted();
    const parsed = JSON.parse(raw) as Partial<PersistedState>;
    if (parsed.v !== STORAGE_VERSION) return defaultPersisted();
    return {
      v: STORAGE_VERSION,
      popup: { ...DEFAULT_GEOMETRY, ...(parsed.popup ?? {}) },
      activeSessionId: parsed.activeSessionId ?? null,
    };
  } catch {
    return defaultPersisted();
  }
}

function defaultPersisted(): PersistedState {
  return {
    v: STORAGE_VERSION,
    popup: { ...DEFAULT_GEOMETRY },
    activeSessionId: null,
  };
}

function savePersisted(state: PersistedState) {
  try {
    globalThis.localStorage?.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // best effort; localStorage may be disabled in private mode.
  }
}

export function TerminalProvider({ children }: Readonly<{ children: ComponentChildren }>) {
  const persistedRef = useRef<PersistedState>(defaultPersisted());
  // Initial state is read lazily so SSR/Node test environments without
  // localStorage do not crash. The first effect below hydrates the
  // persisted value.
  const [state, setState] = useState<TerminalStoreState>(() => ({
    open: false,
    minimized: false,
    maximized: persistedRef.current.popup.maximized,
    activeSessionId: null,
    role: 'none',
    connection: 'idle',
    sessions: [],
    popup: { ...DEFAULT_GEOMETRY },
    lastError: null,
  }));

  // Hydrate from localStorage on mount.
  useEffect(() => {
    const p = loadPersisted();
    persistedRef.current = p;
    setState((prev) => ({
      ...prev,
      activeSessionId: p.activeSessionId,
      popup: p.popup,
      maximized: p.popup.maximized,
    }));
  }, []);

  const persist = useCallback((next: Partial<TerminalStoreState>) => {
    setState((prev) => {
      const merged = { ...prev, ...next };
      const persisted: PersistedState = {
        v: STORAGE_VERSION,
        popup: merged.popup,
        activeSessionId: merged.activeSessionId,
      };
      savePersisted(persisted);
      persistedRef.current = persisted;
      return merged;
    });
  }, []);

  const setSession = useCallback((id: string) => persist({ activeSessionId: id }), [persist]);
  const setRole = useCallback((role: TerminalStoreState['role']) => setState((p) => ({ ...p, role })), []);
  const setConnection = useCallback(
    (connection: TerminalStoreState['connection']) => setState((p) => ({ ...p, connection })),
    [],
  );
  const setLastError = useCallback((lastError: string | null) => setState((p) => ({ ...p, lastError })), []);

  const setGeometry = useCallback(
    (g: Partial<TerminalPopupGeometry>) => {
      setState((prev) => {
        const next: TerminalStoreState = { ...prev, popup: { ...prev.popup, ...g } };
        savePersisted({
          v: STORAGE_VERSION,
          popup: next.popup,
          activeSessionId: next.activeSessionId,
        });
        return next;
      });
    },
    [],
  );

  const setMinimized = useCallback(
    (minimized: boolean) => {
      setState((prev) => {
        const next: TerminalStoreState = { ...prev, minimized };
        if (!minimized) next.maximized = false;
        savePersisted({
          v: STORAGE_VERSION,
          popup: { ...next.popup, minimized, maximized: next.maximized },
          activeSessionId: next.activeSessionId,
        });
        return next;
      });
    },
    [],
  );

  const setMaximized = useCallback(
    (maximized: boolean) => {
      setState((prev) => {
        const next: TerminalStoreState = { ...prev, maximized };
        savePersisted({
          v: STORAGE_VERSION,
          popup: { ...next.popup, maximized },
          activeSessionId: next.activeSessionId,
        });
        return next;
      });
    },
    [],
  );

  const refreshSessions = useCallback(async () => {
    try {
      const sessions = await terminalApi.list();
      setState((prev) => ({ ...prev, sessions }));
    } catch (err) {
      // Silent: the store will retry on the next refresh trigger.
      // eslint-disable-next-line no-console
      console.warn('terminal: list sessions failed', err);
    }
  }, []);

  const openTerminal = useCallback(
    async (sessionId?: string) => {
      let id = sessionId ?? persistedRef.current.activeSessionId ?? null;
      if (id) {
        try {
          const sessions = await terminalApi.list();
          const known = sessions.find((s) => s.id === id);
          if (!known) {
            // Either swept, lost on server restart, or never existed.
            id = null;
          }
        } catch {
          // Backend unreachable — fall through to a fresh session.
          id = null;
        }
      }
      setState((prev) => ({
        ...prev,
        open: true,
        minimized: false,
        activeSessionId: id,
        connection: id ? 'connecting' : 'connecting',
        lastError: null,
      }));
      if (!id) {
        // Mark as not-yet-assigned; the WS handler will fill it in once
        // the server creates a session and the ready frame arrives.
        persist({ activeSessionId: '' });
      } else {
        persist({ activeSessionId: id });
      }
    },
    [persist],
  );

  const closeTerminal = useCallback(() => {
    setState((prev) => ({ ...prev, open: false, connection: 'idle' }));
  }, []);

  const toggleTerminal = useCallback(
    async (sessionId?: string) => {
      setState((prev) => {
        if (prev.open) {
          // Window is open — closing it keeps the session alive in the
          // hub. The store state remains valid; we just hide the popup.
          return { ...prev, open: false, connection: 'idle' };
        }
        return prev; // openTerminal below will set open: true
      });
      if (!state.open) {
        await openTerminal(sessionId);
      }
    },
    [state.open, openTerminal],
  );

  const value = useMemo<TerminalStore>(
    () => ({
      ...state,
      openTerminal,
      closeTerminal,
      toggleTerminal,
      setSession,
      setRole,
      setConnection,
      setLastError,
      setGeometry,
      setMinimized,
      setMaximized,
      refreshSessions,
    }),
    [
      state,
      openTerminal,
      closeTerminal,
      toggleTerminal,
      setSession,
      setRole,
      setConnection,
      setLastError,
      setGeometry,
      setMinimized,
      setMaximized,
      refreshSessions,
    ],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useTerminalStore(): TerminalStore {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error('useTerminalStore must be used within TerminalProvider');
  return ctx;
}

// Helper for components that want to test the store without mounting the
// provider (used by unit tests).
export function __resetPersistedStateForTests() {
  try {
    globalThis.localStorage?.removeItem(STORAGE_KEY);
  } catch {
    // ignore
  }
}
