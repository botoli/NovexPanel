import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { API_BASE } from '../../../Api/api';
import { Confirm } from '../../../modals/Confirm/Confirm';
import { serverMetricsStore } from '../../../Store/ServerMetricsStore';
import { toastStore } from '../../../Store/ToastStore';
import { tokenStore } from '../../../Store/TokenStore';
import styles from './SecretsPage.module.scss';

type SecretRow = {
  id: number;
  name: string;
  type: string;
  service_scope: string;
  masked_value: string;
  expires_at?: string;
  revoked_at?: string;
  usage_count: number;
  last_used_at?: string;
  last_rotated_at?: string;
};

const SecretTypes = ['env_var', 'api_key', 'ssh_key', 'certificate', 'token'] as const;

const SecretsPage = observer(() => {
  const { id } = useParams<{ id?: string; }>();
  const fallbackServerId = serverMetricsStore.getNowServers()[0]?.id;
  const serverId = Number.isFinite(Number(id)) ? Number(id) : fallbackServerId;
  const [rows, setRows] = useState<SecretRow[]>([]);
  const [audit, setAudit] = useState<any[]>([]);
  const [name, setName] = useState('');
  const [value, setValue] = useState('');
  const [secretType, setSecretType] = useState<typeof SecretTypes[number]>('env_var');
  const [serviceScope, setServiceScope] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [revealId, setRevealId] = useState<number | null>(null);
  const [partialReveal, setPartialReveal] = useState<string>('');
  const [revokeId, setRevokeId] = useState<number | null>(null);

  const buildAuthHeaders = () => ({ Authorization: `Bearer ${tokenStore.getToken()}` });

  const load = async () => {
    if (!serverId) return;
    const [listResp, auditResp] = await Promise.all([
      fetch(`${API_BASE}/servers/${serverId}/secrets`, { headers: buildAuthHeaders() }),
      fetch(`${API_BASE}/servers/${serverId}/secrets/audit`, { headers: buildAuthHeaders() }),
    ]);
    if (listResp.ok) {
      const payload = await listResp.json();
      setRows(Array.isArray(payload) ? payload : []);
    }
    if (auditResp.ok) {
      const payload = await auditResp.json();
      setAudit(Array.isArray(payload) ? payload : []);
    }
  };

  useEffect(() => {
    void load();
  }, [serverId]);

  const createSecret = async () => {
    if (!serverId) return;
    if (!name.trim()) {
      toastStore.push('error', 'Secret name is required', 'Secrets Vault');
      return;
    }
    if (!value.trim()) {
      toastStore.push('error', 'Secret value is required', 'Secrets Vault');
      return;
    }
    const resp = await fetch(`${API_BASE}/servers/${serverId}/secrets`, {
      method: 'POST',
      headers: { ...buildAuthHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        type: secretType,
        value,
        service_scope: serviceScope,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : null,
      }),
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
    setName('');
    setValue('');
    setServiceScope('');
    setExpiresAt('');
    await load();
    toastStore.push('success', 'Secret created', 'Secrets Vault');
  };

  const rotateSecret = async (id: number) => {
    const next = window.prompt('Enter new secret value');
    if (!next || !next.trim()) return;
    const resp = await fetch(`${API_BASE}/servers/${serverId}/secrets/${id}/rotate`, {
      method: 'POST',
      headers: { ...buildAuthHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ value: next.trim() }),
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
    await load();
  };

  const revealPartial = async (id: number) => {
    const resp = await fetch(`${API_BASE}/servers/${serverId}/secrets/${id}/reveal`, {
      method: 'POST',
      headers: { ...buildAuthHeaders(), 'Content-Type': 'application/json' },
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
    setRevealId(id);
    setPartialReveal(String(payload?.value || ''));
  };

  const revokeSecret = async (id: number) => {
    const resp = await fetch(`${API_BASE}/servers/${serverId}/secrets/${id}/revoke`, {
      method: 'POST',
      headers: { ...buildAuthHeaders(), 'Content-Type': 'application/json' },
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
    await load();
  };

  const typeTone = (type: string) => {
    const value = type.toLowerCase();
    if (value.includes('token') || value.includes('api')) return styles.toneInfo;
    if (value.includes('ssh') || value.includes('cert')) return styles.toneWarn;
    return styles.toneNeutral;
  };

  if (!serverId) {
    return <div className={styles.page}>No server available for Secrets Vault.</div>;
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h2>Secrets Vault</h2>
        <button type='button' className={styles.btn} onClick={() => void load()}>Refresh</button>
      </div>

      <section className={styles.card}>
        <h3>Create secret</h3>
        <div className={styles.formRow}>
          <input
            className={styles.input}
            placeholder='Name'
            value={name}
            onChange={e => setName(e.target.value)}
          />
          <select
            className={styles.select}
            value={secretType}
            onChange={e => setSecretType(e.target.value as any)}
          >
            {SecretTypes.map(type => <option key={type} value={type}>{type}</option>)}
          </select>
          <input
            className={styles.input}
            placeholder='Service scope (optional)'
            value={serviceScope}
            onChange={e => setServiceScope(e.target.value)}
          />
          <input
            className={styles.input}
            type='datetime-local'
            value={expiresAt}
            onChange={e => setExpiresAt(e.target.value)}
          />
        </div>
        <textarea
          className={styles.editor}
          placeholder='Secret value (never fully shown later)'
          value={value}
          onChange={e => setValue(e.target.value)}
        />
        <div className={styles.row}>
          <button
            type='button'
            className={styles.btnPrimary}
            onClick={() =>
              void createSecret().catch((e) =>
                toastStore.push(
                  'error',
                  e instanceof Error ? e.message : 'Create failed',
                  'Secrets Vault',
                )
              )}
          >
            Create
          </button>
        </div>
      </section>

      <section className={styles.card}>
        <h3>Secrets</h3>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Scope</th>
                <th>Masked value</th>
                <th>Rotation</th>
                <th>Expiry</th>
                <th>Usage</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {rows.map(row => (
                <tr key={row.id} className={styles.tableRow}>
                  <td>{row.name}</td>
                  <td>
                    <span className={`${styles.typePill} ${typeTone(row.type)}`}>{row.type}</span>
                  </td>
                  <td>{row.service_scope || 'global'}</td>
                  <td>{row.masked_value}</td>
                  <td>
                    {row.last_rotated_at ? new Date(row.last_rotated_at).toLocaleString() : 'never'}
                  </td>
                  <td>{row.expires_at ? new Date(row.expires_at).toLocaleString() : '—'}</td>
                  <td>
                    {row.usage_count}
                    {row.last_used_at ? ` (${new Date(row.last_used_at).toLocaleString()})` : ''}
                  </td>
                  <td className={styles.actions}>
                    <button
                      type='button'
                      className={styles.btn}
                      onClick={() =>
                        void revealPartial(row.id).catch((e) =>
                          toastStore.push(
                            'error',
                            e instanceof Error ? e.message : 'Reveal failed',
                            'Secrets Vault',
                          )
                        )}
                    >
                      Partial reveal
                    </button>
                    <button
                      type='button'
                      className={styles.btn}
                      onClick={() =>
                        void rotateSecret(row.id).catch((e) =>
                          toastStore.push(
                            'error',
                            e instanceof Error ? e.message : 'Rotate failed',
                            'Secrets Vault',
                          )
                        )}
                    >
                      Rotate
                    </button>
                    <button
                      type='button'
                      className={styles.btnDanger}
                      onClick={() => setRevokeId(row.id)}
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
              {rows.length === 0
                ? (
                  <tr>
                    <td colSpan={8} className={styles.emptyCell}>No secrets available yet.</td>
                  </tr>
                )
                : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className={styles.card}>
        <h3>Usage mapping and access history</h3>
        <div className={styles.logs}>
          {audit.map(item => (
            <div key={item.id} className={styles.logLine}>
              {new Date(item.created_at).toLocaleString()} — {item.action} — {item.target || 'n/a'}
              {' '}
              — {item.message}
            </div>
          ))}
        </div>
      </section>

      <Confirm
        isOpen={revealId != null}
        title='Partial reveal'
        description={partialReveal ? `Secret preview: ${partialReveal}` : 'No value'}
        confirmText='Close'
        onCancel={() => {
          setRevealId(null);
          setPartialReveal('');
        }}
        onConfirm={() => {
          setRevealId(null);
          setPartialReveal('');
        }}
      />

      <Confirm
        isOpen={revokeId != null}
        title='Revoke secret'
        description={revokeId ? `Revoke secret #${revokeId}?` : ''}
        confirmText='Revoke'
        danger
        onCancel={() => setRevokeId(null)}
        onConfirm={() => {
          if (!revokeId) return;
          void revokeSecret(revokeId)
            .then(() => toastStore.push('success', 'Secret revoked', 'Secrets Vault'))
            .catch((e) =>
              toastStore.push(
                'error',
                e instanceof Error ? e.message : 'Revoke failed',
                'Secrets Vault',
              )
            )
            .finally(() => setRevokeId(null));
        }}
      />
    </div>
  );
});

export default SecretsPage;
