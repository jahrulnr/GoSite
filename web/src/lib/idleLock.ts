// Idle auto-lock: warns the user, then locks the session after `lock_after_seconds`
// of no activity. Resets the timer on any user interaction while warning is up.
//
// Behaviour mirrors BangunSite: countdown toast is dismissable by movement; once
// the timer fires, the panel shows the lockscreen overlay and a fresh login is
// required.
import { useEffect, useRef } from 'preact/hooks';
import { useStore } from './store';
import { auth } from '../api/endpoints';

const ACTIVITY_EVENTS = ['mousemove', 'keydown', 'click', 'touchstart', 'scroll'] as const;
const WARNING_LEAD_SECONDS = 20; // show "locking in Ns" toast in the last 20s
const HEARTBEAT_MS = 1000;

export function useIdleLock() {
  const { user, meta, setLocked, locked, toast, dismissToast } = useStore();
  const warnToastRef = useRef<number | undefined>(undefined);
  const tickerRef = useRef<number | undefined>(undefined);

  useEffect(() => {
    const enabled = meta?.auth?.lockscreen_enabled;
    const seconds = meta?.auth?.lock_after_seconds ?? 0;
    if (!user || !enabled || locked) return;
    if (seconds <= 0) return;

    let lockAt = Date.now() + seconds * 1000;
    let lastWarnSecond = -1;

    const clearTicker = () => {
      if (tickerRef.current) {
        globalThis.clearInterval(tickerRef.current);
        tickerRef.current = undefined;
      }
    };
    const clearWarn = () => {
      if (warnToastRef.current !== undefined) {
        dismissToast(warnToastRef.current);
        warnToastRef.current = undefined;
      }
    };
    const fireLock = () => {
      clearWarn();
      clearTicker();
      auth
        .lock()
        .then(() => {
          setLocked(true);
          toast('Session locked due to inactivity', 'info');
        })
        .catch(() => {
          // re-arm the timer so a future attempt is made
          lockAt = Date.now() + seconds * 1000;
          startTicker();
        });
    };
    const startTicker = () => {
      clearTicker();
      tickerRef.current = globalThis.setInterval(() => {
        const remainingMs = lockAt - Date.now();
        if (remainingMs <= 0) {
          fireLock();
          return;
        }
        const remainingSec = Math.ceil(remainingMs / 1000);
        const inWarning = remainingSec <= WARNING_LEAD_SECONDS;
        if (inWarning) {
          if (lastWarnSecond !== remainingSec) {
            if (warnToastRef.current !== undefined) dismissToast(warnToastRef.current);
            warnToastRef.current = toast(`Locking in ${remainingSec}s — move/click to stay`, 'info');
            lastWarnSecond = remainingSec;
          }
        } else {
          clearWarn();
          lastWarnSecond = -1;
        }
      }, HEARTBEAT_MS);
    };
    const reset = () => {
      lockAt = Date.now() + seconds * 1000;
      lastWarnSecond = -1;
      clearWarn();
      startTicker();
    };

    startTicker();
    for (const ev of ACTIVITY_EVENTS) globalThis.addEventListener(ev, reset, { passive: true });

    return () => {
      clearTicker();
      clearWarn();
      for (const ev of ACTIVITY_EVENTS) globalThis.removeEventListener(ev, reset);
    };
  }, [user, meta, locked, setLocked, toast, dismissToast]);
}
