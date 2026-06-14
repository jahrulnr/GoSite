import { useEffect, useState } from 'preact/hooks';
import { nginx } from '../api/endpoints';
import { IconEdit, IconRefresh, IconShield } from '../components/Icons';
import { EmptyState, ErrorState, Loading, Spinner } from '../components/Ui';
import { Card, Page } from '../components/Layout';
import { useAction, useAsync } from '../lib/hooks';

export function NginxView() {
  const defaultConfig = useAsync(() => nginx.getDefault());
  const globalConfig = useAsync(() => nginx.getGlobal());
  const [tab, setTab] = useState<'default' | 'global'>('default');
  const [defaultText, setDefaultText] = useState('');
  const [globalText, setGlobalText] = useState('');
  const test = useAction(() => nginx.test(tab === 'default' ? defaultText : globalText));
  const saveDefault = useAction(() => nginx.updateDefault(defaultText));
  const saveGlobal = useAction(() => nginx.updateGlobal(globalText));
  const reload = useAction(nginx.reload);
  const activeText = tab === 'default' ? defaultText : globalText;
  const activeState = tab === 'default' ? defaultConfig : globalConfig;
  const setActiveText = tab === 'default' ? setDefaultText : setGlobalText;

  useEffect(() => {
    if (defaultConfig.data !== undefined) setDefaultText(defaultConfig.data);
  }, [defaultConfig.data]);

  useEffect(() => {
    if (globalConfig.data !== undefined) setGlobalText(globalConfig.data);
  }, [globalConfig.data]);

  const save = async () => {
    if (tab === 'default') await saveDefault.run();
    else await saveGlobal.run();
  };

  return (
    <Page
      title="Nginx"
      subtitle="Edit default server or global nginx configuration"
      eyebrow="config"
      actions={
        <>
          <button type="button" class="btn" onClick={() => test.run()}>{test.loading ? <Spinner /> : <><IconShield /> Test</>}</button>
          <button type="button" class="btn" onClick={save}>{saveDefault.loading || saveGlobal.loading ? <Spinner /> : <><IconEdit /> Save</>}</button>
          <button type="button" class="btn primary" onClick={() => reload.run()}>{reload.loading ? <Spinner /> : <><IconRefresh /> Reload</>}</button>
        </>
      }
    >
      <div class="tabs">
        <button type="button" class={tab === 'default' ? 'active' : ''} onClick={() => setTab('default')}>Default server</button>
        <button type="button" class={tab === 'global' ? 'active' : ''} onClick={() => setTab('global')}>Global config</button>
      </div>
      {activeState.loading && activeState.data === undefined ? (
        <Card title={tab === 'default' ? 'Default server' : 'Global config'}>
          <Loading label="Loading nginx config" />
        </Card>
      ) : activeState.error ? (
        <Card title={tab === 'default' ? 'Default server' : 'Global config'}>
          <ErrorState error={activeState.error} onRetry={activeState.reload} />
        </Card>
      ) : (
        <Card title={tab === 'default' ? 'default.conf' : 'nginx.conf'} actions={<span class="dim mono" style="font-size:11px;">{activeText.length} chars</span>}>
          {!activeText.trim() && (
            <EmptyState title="No configuration loaded" hint="Paste or type nginx configuration in the editor below." />
          )}
          <textarea
            class="textarea config-editor"
            value={activeText}
            placeholder="# nginx configuration block"
            onInput={(e) => setActiveText((e.target as HTMLTextAreaElement).value)}
          />
        </Card>
      )}
      {test.error && <ErrorState error={test.error} />}
      {saveDefault.error && <ErrorState error={saveDefault.error} />}
      {saveGlobal.error && <ErrorState error={saveGlobal.error} />}
      {reload.error && <ErrorState error={reload.error} />}
    </Page>
  );
}
