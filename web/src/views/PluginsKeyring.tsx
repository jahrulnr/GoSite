import { useMemo, useState } from 'preact/hooks';
import { plugins } from '../api/endpoints';
import type { PluginKeyringEntry } from '../api/types';
import { IconPlus, IconShield, IconTrash } from '../components/Icons';
import { Badge, EmptyState, Field, InlineNotice, Modal, Spinner } from '../components/Ui';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function truncateKey(value: string, max = 24) {
  if (value.length <= max) return value;
  return `${value.slice(0, max - 1)}…`;
}

function groupByVendor(keys: PluginKeyringEntry[]) {
  const map = new Map<string, PluginKeyringEntry[]>();
  for (const key of keys) {
    map.set(key.vendor, [...(map.get(key.vendor) ?? []), key]);
  }
  return [...map.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function AddKeyModal({ onClose, onAdded }: Readonly<{ onClose: () => void; onAdded: () => void }>) {
  const { meta, toast } = useStore();
  const add = useAction(plugins.addKeyringEntry);
  const [vendor, setVendor] = useState('');
  const [keyId, setKeyId] = useState('');
  const [publicKey, setPublicKey] = useState('');

  const submit = async (event: Event) => {
    event.preventDefault();
    try {
      await add.run({ vendor: vendor.trim(), keyId: keyId.trim(), publicKey: publicKey.trim() });
      toast('Trusted key added');
      onAdded();
      onClose();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  return (
    <Modal
      title="Add trusted key"
      onClose={onClose}
      footer={
        <>
          {add.error && <InlineNotice kind="danger">{humanizeError(add.error, meta)}</InlineNotice>}
          <button type="button" class="btn ghost" onClick={onClose}>Cancel</button>
          <button type="submit" form="plugin-keyring-add" class="btn primary" disabled={add.loading}>
            {add.loading ? <Spinner /> : <><IconPlus /> Add key</>}
          </button>
        </>
      }
    >
      <form id="plugin-keyring-add" onSubmit={submit}>
        <InlineNotice kind="info">
          Keys verify Ed25519 signatures on plugin artifacts in strict trust mode.
          Revoking a key blocks new installs; already-enabled plugins keep running.
        </InlineNotice>
        <Field label="Vendor" hint="Publisher namespace, e.g. acme">
          <input class="input" value={vendor} onInput={(e) => setVendor((e.target as HTMLInputElement).value)} required />
        </Field>
        <Field label="Key ID" hint="Stable id referenced in manifest signatures">
          <input class="input mono" value={keyId} onInput={(e) => setKeyId((e.target as HTMLInputElement).value)} required />
        </Field>
        <Field label="Public key" hint="Base64-encoded Ed25519 public key">
          <textarea
            class="textarea mono"
            rows={4}
            value={publicKey}
            onInput={(e) => setPublicKey((e.target as HTMLTextAreaElement).value)}
            required
          />
        </Field>
      </form>
    </Modal>
  );
}

export function PluginsKeyringPanel() {
  const { meta, toast } = useStore();
  const state = useAsync(() => plugins.listKeyring());
  const revoke = useAction(plugins.revokeKeyringEntry);
  const [addOpen, setAddOpen] = useState(false);
  const groups = useMemo(() => groupByVendor(state.data ?? []), [state.data]);

  const onRevoke = async (entry: PluginKeyringEntry) => {
    if (!globalThis.confirm(`Revoke ${entry.vendor}/${entry.keyId}? New installs signed with this key will be rejected.`)) {
      return;
    }
    try {
      await revoke.run(entry.vendor, entry.keyId);
      toast('Key revoked');
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  if (state.loading && !state.data) {
    return <div class="dim" style="padding:24px;"><Spinner /> Loading keyring…</div>;
  }
  if (state.error) {
    return <InlineNotice kind="danger">{humanizeError(state.error, meta)}</InlineNotice>;
  }

  return (
    <>
      <div class="row between wrap" style="margin-bottom:16px;">
        <InlineNotice kind="info">
          <IconShield width={16} height={16} style="vertical-align:-3px;margin-right:6px;" />
          Trusted vendor keys for signed plugin installs. Grouped by publisher.
        </InlineNotice>
        <button type="button" class="btn primary" onClick={() => setAddOpen(true)}>
          <IconPlus /> Add key
        </button>
      </div>
      {groups.length === 0 ? (
        <EmptyState
          title="No trusted keys"
          hint="Add a vendor public key to require signed artifacts in strict trust mode."
        />
      ) : (
        <div class="plugin-keyring-groups">
          {groups.map(([vendor, keys]) => (
            <section key={vendor} class="plugin-keyring-vendor">
              <h3 class="mono">{vendor}</h3>
              <div class="table-wrap">
                <table class="table compact">
                  <thead>
                    <tr>
                      <th>Key ID</th>
                      <th>Public key</th>
                      <th>Status</th>
                      <th class="right">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {keys.map((entry) => (
                      <tr key={`${entry.vendor}-${entry.keyId}`}>
                        <td class="mono">{entry.keyId}</td>
                        <td class="mono dim">{truncateKey(entry.publicKey, 32)}</td>
                        <td>
                          {entry.revokedAt ? (
                            <Badge kind="off">revoked</Badge>
                          ) : (
                            <Badge kind="ok">active</Badge>
                          )}
                        </td>
                        <td class="right">
                          {!entry.revokedAt && (
                            <button
                              type="button"
                              class="btn sm ghost danger"
                              disabled={revoke.loading}
                              onClick={() => void onRevoke(entry)}
                            >
                              <IconTrash /> Revoke
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ))}
        </div>
      )}
      {addOpen && <AddKeyModal onClose={() => setAddOpen(false)} onAdded={state.reload} />}
    </>
  );
}
