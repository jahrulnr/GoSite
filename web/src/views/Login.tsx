// Login + lockscreen. All copy/options come from the backend ui/meta + auth metadata.
import { useState } from 'preact/hooks';
import { auth, ui } from '../api/endpoints';
import { mergePanelMeta } from '../lib/meta';
import { useAction } from '../lib/hooks';
import { useStore } from '../lib/store';
import { ApiError } from '../api/client';
import { initials } from '../lib/format';
import { Spinner } from '../components/Ui';
import { IconLock, IconLogout } from '../components/Icons';

export function Login() {
  const { meta, setUser, setLocked, setMeta, toast } = useStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [remember, setRemember] = useState(false);
  const { run, loading, error } = useAction(auth.login);

  const app = meta?.app;
  const authMeta = meta?.auth;

  const onSubmit = async (e: Event) => {
    e.preventDefault();
    try {
      const res = await run(email, password, remember);
      if (res?.user) {
        setUser(res.user);
        setLocked(false);
        // Session cookie is set by login; refresh ui/meta so files.roots and nav load.
        try {
          const fresh = await ui.meta();
          setMeta(mergePanelMeta(fresh, meta?.auth));
        } catch {
          /* keep existing partial meta from BootGate */
        }
        toast(`Welcome back, ${res.user.name}`);
      }
    } catch {
      /* surfaced via error */
    }
  };

  return (
    <div class="login-screen">
      <div class="app-bg-glow" />
      <form class="login-card" onSubmit={onSubmit}>
        <div class="brand-logo">{app?.logo_letter ?? 'G'}</div>
        <h2>{app?.name ?? 'GoSite'}</h2>
        <p class="sub">{authMeta?.login_hint ?? 'Sign in to your control panel'}</p>

        <label class="field">
          <span>Email</span>
          <input
            class="input"
            type="email"
            autocomplete="username"
            placeholder={authMeta?.login_email_placeholder}
            value={email}
            onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
            required
          />
        </label>

        <label class="field">
          <span>Password</span>
          <input
            class="input"
            type="password"
            autocomplete="current-password"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            required
          />
        </label>

        {authMeta?.remember_me && (
          <label class="row" style="gap:8px;margin-bottom:14px;cursor:pointer;font-size:13px;color:var(--text-muted);">
            <input
              type="checkbox"
              checked={remember}
              onChange={(e) => setRemember((e.target as HTMLInputElement).checked)}
            />
            <span>Remember me</span>
          </label>
        )}

        {error && (
          <p class="field-error" style="margin:-4px 0 12px;">
            {error instanceof ApiError ? error.message : 'Login failed'}
          </p>
        )}

        <button type="submit" class="btn primary" style="width:100%;justify-content:center;padding:10px;" disabled={loading}>
          {loading ? <Spinner /> : <><IconLogout /> Sign in</>}
        </button>
      </form>
    </div>
  );
}

export function Lockscreen() {
  const { user, meta, setLocked, toast } = useStore();
  const [password, setPassword] = useState('');
  const { run, loading, error } = useAction(auth.unlock);

  const onSubmit = async (e: Event) => {
    e.preventDefault();
    try {
      const res = await run(password);
      if (res) {
        setLocked(false);
        setPassword('');
        toast('Unlocked');
      }
    } catch {
      /* surfaced via error */
    }
  };

  return (
    <div class="login-screen">
      <div class="app-bg-glow" />
      <form class="login-card" onSubmit={onSubmit}>
        <div class="avatar" style="width:56px;height:56px;font-size:20px;margin:0 auto 14px;border-radius:16px;">
          {initials(user?.name)}
        </div>
        <h2>{user?.name ?? 'Locked'}</h2>
        <p class="sub">{meta?.app?.name ?? 'GoSite'} is locked</p>

        <label class="field">
          <span>Password</span>
          <input
            class="input"
            type="password"
            autocomplete="current-password"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            required
          />
        </label>

        {error && (
          <p class="field-error" style="margin:-4px 0 12px;">
            {error instanceof ApiError ? error.message : 'Unlock failed'}
          </p>
        )}

        <button type="submit" class="btn primary" style="width:100%;justify-content:center;padding:10px;" disabled={loading}>
          {loading ? <Spinner /> : <><IconLock /> Unlock</>}
        </button>
      </form>
    </div>
  );
}
