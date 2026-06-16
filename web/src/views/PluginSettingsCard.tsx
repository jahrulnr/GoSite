import { plugins } from '../api/endpoints';
import { AsyncView, Badge, InlineNotice, KeyValue } from '../components/Ui';
import { navigate } from '../lib/router';
import { useAsync } from '../lib/hooks';

function tokenBadge(configured: boolean) {
  return configured ? <Badge kind="ok">configured</Badge> : <Badge kind="off">not set</Badge>;
}

export function PluginSettingsCard() {
  const state = useAsync(() => plugins.installSettings());

  return (
    <AsyncView state={state} loadingLabel="Loading plugin settings">
      {(settings) => (
        <div class="col" style="gap:12px;">
          <InlineNotice kind="info">
            Public GitHub repos need no token. For private repos, set <span class="mono">GITHUB_TOKEN</span> in
            the host environment (e.g. docker compose env file) or install plugins via Artifact upload.
          </InlineNotice>
          <div class="grid cols-2" style="gap:12px;">
            <KeyValue label="Remote install" mono>{settings.remote_install_enabled ? 'enabled' : 'disabled'}</KeyValue>
            <KeyValue label="Trust mode" mono>{settings.trust_mode}</KeyValue>
            <KeyValue label="GitHub token" mono>{tokenBadge(settings.github_token_configured)}</KeyValue>
            <KeyValue label="GitLab token" mono>{tokenBadge(settings.gitlab_token_configured)}</KeyValue>
            <KeyValue label="Allow unsigned" mono>{settings.allow_unsigned ? 'yes' : 'no'}</KeyValue>
          </div>
          {settings.allowed_hosts.length > 0 && (
            <>
              <div class="divider" />
              <KeyValue label="Allowed fetch hosts" mono>{settings.allowed_hosts.join(', ')}</KeyValue>
            </>
          )}
          <div class="row wrap" style="margin-top:4px;">
            <button type="button" class="btn sm ghost" onClick={() => navigate('/plugins')}>Open registry</button>
            <button type="button" class="btn sm ghost" onClick={() => navigate('/plugins/keyring')}>Manage keyring</button>
          </div>
        </div>
      )}
    </AsyncView>
  );
}
