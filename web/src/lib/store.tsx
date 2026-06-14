// Global app context: authenticated user, backend UI meta, lock state, and toasts.
// Stateless wrt the backend — user/meta are fetched fresh on boot and after login.
import { createContext } from 'preact';
import { useCallback, useContext, useMemo, useRef, useState } from 'preact/hooks';
import type { ComponentChildren } from 'preact';
import type { UiMetaResponse, User } from '../api/types';

export interface Toast {
  id: number;
  message: string;
  kind: 'info' | 'error';
}

export interface AppStore {
  user: User | undefined;
  meta: UiMetaResponse | undefined;
  locked: boolean;
  setUser: (u: User | undefined) => void;
  setMeta: (m: UiMetaResponse | undefined) => void;
  setLocked: (v: boolean) => void;
  toasts: Toast[];
  toast: (message: string, kind?: Toast['kind']) => void;
  dismissToast: (id: number) => void;
}

const Ctx = createContext<AppStore | null>(null);

export function AppProvider({ children }: Readonly<{ children: ComponentChildren }>) {
  const [user, setUser] = useState<User>();
  const [meta, setMeta] = useState<UiMetaResponse>();
  const [locked, setLocked] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(1);

  const dismissToast = useCallback((id: number) => {
    setToasts((list) => list.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback(
    (message: string, kind: Toast['kind'] = 'info') => {
      const id = nextId.current++;
      setToasts((list) => [...list, { id, message, kind }]);
      globalThis.setTimeout(() => dismissToast(id), kind === 'error' ? 6000 : 3500);
    },
    [dismissToast],
  );

  const value = useMemo<AppStore>(
    () => ({ user, meta, locked, setUser, setMeta, setLocked, toasts, toast, dismissToast }),
    [user, meta, locked, toasts, toast, dismissToast],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useStore(): AppStore {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error('useStore must be used within AppProvider');
  return ctx;
}
