import { nginx } from '../api/endpoints';
import { IconEdit, IconRefresh, IconShield } from '../components/Icons';
import { CodeEditor } from '../components/CodeEditor';
import { ErrorState, Loading, Spinner } from '../components/Ui';
import { Card, Page } from '../components/Layout';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { validateDefaultNginx, validateGlobalNginx } from '../lib/nginxValidate';
import { useStore } from '../lib/store';
import { useEffect, useState } from 'preact/hooks';

export function NginxView() {
  const { toast } = useStore();
  const defaultConfig = useAsync(() => nginx.getDefault());
  const globalConfig = useAsync(() => nginx.getGlobal());
  const [tab, setTab] = useState<'default' | 'global'>('default');
  const [defaultText, setDefaultText] = useState('');
  const [globalText, setGlobalText] = useState('');
  const test = useAction(() => nginx.test(tab === 'default' ? defaultText : globalText, tab));
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

  const onTest = async () => {
    try {
      await test.run();
      toast('Configuration syntax OK');
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  const save = async () => {
    try {
      if (tab === 'default') await saveDefault.run();
      else await saveGlobal.run();
      toast(tab === 'default' ? 'default.conf saved' : 'nginx.conf saved');
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  const onReload = async () => {
    try {
      await reload.run();
      toast('Nginx reloaded');
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  return (
    <Page
      title="Nginx"
      subtitle="Edit default server or global nginx configuration"
      eyebrow="config"
      actions={
        <>
          <button type="button" class="btn" onClick={onTest}>{test.loading ? <Spinner /> : <><IconShield /> Test</>}</button>
          <button type="button" class="btn" onClick={save}>{saveDefault.loading || saveGlobal.loading ? <Spinner /> : <><IconEdit /> Save</>}</button>
          <button type="button" class="btn primary" onClick={onReload}>{reload.loading ? <Spinner /> : <><IconRefresh /> Reload</>}</button>
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
          <CodeEditor
            key={tab}
            value={activeText}
            onChange={setActiveText}
            language="nginx"
            className="config-editor"
            placeholder="# nginx configuration"
            onValidate={tab === 'default' ? validateDefaultNginx : validateGlobalNginx}
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
