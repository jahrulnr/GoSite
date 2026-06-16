import { useState } from 'preact/hooks';
import { rootHealth, settings } from '../api/endpoints';
import { AsyncView, Badge, ErrorState, Field, KeyValue, Spinner } from '../components/Ui';
import { Card, Page } from '../components/Layout';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

export function SettingsView() {
  const { user, setUser, toast } = useStore();
  const [name, setName] = useState(user?.name ?? '');
  const [email, setEmail] = useState(user?.email ?? '');
  const [password, setPassword] = useState('');
  const save = useAction(settings.updateProfile);
  const health = useAsync(rootHealth);

  const submit = async (event: Event) => {
    event.preventDefault();
    try {
      const res = await save.run({ name, email, password: password || undefined });
      if (res?.user) setUser(res.user);
      setPassword('');
      toast('Profile updated');
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  return (
    <Page title="Settings" subtitle="Manage your account and inspect panel health" eyebrow="config">
      <div class="grid cols-2">
        <Card title="Profile">
          <form onSubmit={submit}>
            <Field label="Name"><input class="input" value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} /></Field>
            <Field label="Email"><input class="input" type="email" value={email} onInput={(e) => setEmail((e.target as HTMLInputElement).value)} /></Field>
            <Field label="New password" hint="Leave blank to keep the current password">
              <input class="input" type="password" value={password} onInput={(e) => setPassword((e.target as HTMLInputElement).value)} />
            </Field>
            {save.error && <ErrorState error={save.error} />}
            <div class="row" style="margin-top:6px;">
              <button type="submit" class="btn primary">{save.loading ? <Spinner /> : 'Save profile'}</button>
            </div>
          </form>
        </Card>
        <Card title="Health">
          <AsyncView state={health}>
            {(data) => {
              const status = typeof data === 'object' && data && 'status' in data
                ? String((data as { status: string }).status)
                : 'unknown';
              const ok = status === 'ok';
              return (
                <div class="col">
                  <div class="row" style="gap:10px;">
                    <Badge kind={ok ? 'ok' : 'warn'}>{ok ? 'Healthy' : status}</Badge>
                    <span class="dim">{ok ? 'API is responding normally.' : 'Service reported a non-ok status.'}</span>
                  </div>
                  <div class="divider" />
                  <KeyValue label="Endpoint" mono>/health</KeyValue>
                  <KeyValue label="Auth" mono>{user?.email}</KeyValue>
                </div>
              );
            }}
          </AsyncView>
        </Card>
      </div>
    </Page>
  );
}
