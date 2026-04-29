import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import { API_BASE } from '../../../Api/api';
import { useCurrentServer } from '../../../Store/ServerStore';
import { tokenStore } from '../../../Store/TokenStore';
import { toastStore } from '../../../Store/ToastStore';
import { Confirm } from '../../../modals/Confirm/Confirm';
import styles from './FilesPage.module.scss';

type FileItem = { name: string; path: string; type: 'file' | 'dir'; };

const providerByPath = (path: string): string => {
  const v = path.toLowerCase();
  if (v.includes('nginx')) return 'nginx';
  if (v.includes('docker-compose') || v.endsWith('.yml') || v.endsWith('.yaml')) return 'docker-compose';
  if (v.includes('/systemd/') || v.endsWith('.service')) return 'systemd';
  if (v.endsWith('.env')) return 'env';
  if (v.endsWith('.json')) return 'json';
  if (v.endsWith('.toml')) return 'toml';
  if (v.endsWith('.conf')) return 'conf';
  return 'generic';
};

const FilesPage = observer(() => {
  const { serverId } = useCurrentServer();
  const [currentPath, setCurrentPath] = useState('/etc');
  const [items, setItems] = useState<FileItem[]>([]);
  const [selectedFile, setSelectedFile] = useState<string>('');
  const [originalContent, setOriginalContent] = useState('');
  const [editedContent, setEditedContent] = useState('');
  const [checksum, setChecksum] = useState('');
  const [provider, setProvider] = useState('generic');
  const [validation, setValidation] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);
  const [audit, setAudit] = useState<any[]>([]);
  const [lockToken, setLockToken] = useState('');
  const [confirmRollback, setConfirmRollback] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);

  const authHeaders = useMemo(() => ({ Authorization: `Bearer ${tokenStore.getToken()}` }), []);

  const loadTree = async (path = currentPath) => {
    if (!Number.isFinite(serverId)) return;
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/tree?path=${encodeURIComponent(path)}`, { headers: authHeaders });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const payload = await resp.json();
    setCurrentPath(payload.path || path);
    setItems(Array.isArray(payload.items) ? payload.items : []);
  };

  const lockFile = async (path: string) => {
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/lock`, {
      method: 'POST',
      headers: { ...authHeaders, 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const payload = await resp.json();
    setLockToken(payload.lock_token || '');
  };

  const loadFile = async (path: string) => {
    if (!Number.isFinite(serverId)) return;
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/content?path=${encodeURIComponent(path)}`, { headers: authHeaders });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const payload = await resp.json();
    setSelectedFile(path);
    setOriginalContent(String(payload.content || ''));
    setEditedContent(String(payload.content || ''));
    setChecksum(String(payload.checksum || ''));
    setProvider(providerByPath(path));
    await lockFile(path);
  };

  const loadHistory = async () => {
    if (!selectedFile) return;
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/history?path=${encodeURIComponent(selectedFile)}`, { headers: authHeaders });
    if (!resp.ok) return;
    const payload = await resp.json();
    setHistory(Array.isArray(payload) ? payload : []);
  };

  const loadAudit = async () => {
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/audit`, { headers: authHeaders });
    if (!resp.ok) return;
    const payload = await resp.json();
    setAudit(Array.isArray(payload) ? payload : []);
  };

  useEffect(() => {
    if (!Number.isFinite(serverId)) return;
    void loadTree('/etc');
    void loadAudit();
  }, [serverId]);

  useEffect(() => {
    if (!selectedFile) return;
    void loadHistory();
  }, [selectedFile]);

  const validate = async () => {
    if (!selectedFile) return;
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/validate`, {
      method: 'POST',
      headers: { ...authHeaders, 'Content-Type': 'application/json' },
      body: JSON.stringify({ provider, path: selectedFile, content: editedContent }),
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
    setValidation(payload);
  };

  const apply = async () => {
    if (!selectedFile) return;
    setLoading(true);
    try {
      const resp = await fetch(`${API_BASE}/servers/${serverId}/files/apply`, {
        method: 'POST',
        headers: { ...authHeaders, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider,
          path: selectedFile,
          content: editedContent,
          expected_hash: checksum,
          lock_token: lockToken,
        }),
      });
      const payload = await resp.json().catch(() => ({}));
      if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
      setOriginalContent(editedContent);
      setChecksum(String(payload.checksum || checksum));
      setValidation(payload.validation || validation);
      toastStore.push('success', 'File applied successfully', 'File Ops');
      await Promise.all([loadHistory(), loadAudit()]);
    } finally {
      setLoading(false);
    }
  };

  const rollback = async (versionId: number) => {
    const resp = await fetch(`${API_BASE}/servers/${serverId}/files/rollback`, {
      method: 'POST',
      headers: { ...authHeaders, 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: selectedFile, version_id: versionId }),
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(payload?.error || `HTTP ${resp.status}`);
    await loadFile(selectedFile);
    await Promise.all([loadHistory(), loadAudit()]);
    toastStore.push('success', 'Rollback applied', 'File Ops');
  };

  const unlock = async () => {
    if (!selectedFile || !lockToken) return;
    await fetch(`${API_BASE}/servers/${serverId}/files/unlock`, {
      method: 'POST',
      headers: { ...authHeaders, 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: selectedFile, lock_token: lockToken }),
    });
    setLockToken('');
  };

  useEffect(() => () => { void unlock(); }, [selectedFile, lockToken]);

  if (!Number.isFinite(serverId)) return null;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h2>File Ops</h2>
        <div className={styles.row}>
          <input className={styles.input} value={currentPath} onChange={e => setCurrentPath(e.target.value)} />
          <button type='button' className={styles.btn} onClick={() => void loadTree(currentPath)}>Browse</button>
        </div>
      </div>

      <div className={styles.layout}>
        <section className={styles.card}>
          <h3>File Browser</h3>
          <div className={styles.list}>
            {items.map(item => (
              <button
                key={item.path}
                type='button'
                className={styles.listItem}
                onClick={() => {
                  if (item.type === 'dir') {
                    void loadTree(item.path);
                  } else {
                    void loadFile(item.path).catch((e) => toastStore.push('error', e instanceof Error ? e.message : 'Open file failed', 'File Ops'));
                  }
                }}
              >
                {item.type === 'dir' ? 'DIR' : 'FILE'} {item.name}
              </button>
            ))}
          </div>
        </section>

        <section className={styles.card}>
          <h3>Editor</h3>
          <div className={styles.row}>
            <label>Provider</label>
            <select value={provider} className={styles.select} onChange={e => setProvider(e.target.value)}>
              <option value='generic'>generic</option>
              <option value='nginx'>nginx</option>
              <option value='docker-compose'>docker-compose</option>
              <option value='systemd'>systemd</option>
              <option value='env'>env</option>
              <option value='yaml'>yaml</option>
              <option value='json'>json</option>
              <option value='toml'>toml</option>
              <option value='conf'>conf</option>
            </select>
            <button type='button' className={styles.btn} disabled={!selectedFile} onClick={() => void validate().catch((e) => toastStore.push('error', e instanceof Error ? e.message : 'Validation failed', 'File Ops'))}>Validate</button>
            <button type='button' className={styles.btnPrimary} disabled={!selectedFile || loading} onClick={() => void apply().catch((e) => toastStore.push('error', e instanceof Error ? e.message : 'Apply failed', 'File Ops'))}>
              {loading ? 'Applying...' : 'Apply'}
            </button>
          </div>
          <div className={styles.diffGrid}>
            <div>
              <div className={styles.label}>Before</div>
              <pre className={styles.code}>{originalContent}</pre>
            </div>
            <div>
              <div className={styles.label}>After</div>
              <textarea className={styles.editor} value={editedContent} onChange={e => setEditedContent(e.target.value)} />
            </div>
          </div>
        </section>
      </div>

      <div className={styles.layout}>
        <section className={styles.card}>
          <h3>Validation Panel</h3>
          <pre className={styles.code}>{JSON.stringify(validation, null, 2)}</pre>
        </section>
        <section className={styles.card}>
          <h3>History Timeline</h3>
          <div className={styles.list}>
            {history.map(row => (
              <button key={row.id} type='button' className={styles.listItem} onClick={() => setConfirmRollback(row.id)}>
                v{row.id} {new Date(row.created_at).toLocaleString()} {row.checksum.slice(0, 8)}
              </button>
            ))}
          </div>
        </section>
      </div>

      <section className={styles.card}>
        <h3>Audit Logs</h3>
        <div className={styles.list}>
          {audit.map(item => (
            <div key={item.id} className={styles.listItem}>
              [{item.provider}] {item.path} {item.action} {item.success ? 'ok' : `failed: ${item.message || 'error'}`}
            </div>
          ))}
        </div>
      </section>

      <Confirm
        isOpen={confirmRollback != null}
        title='Rollback file'
        description={confirmRollback ? `Rollback to version #${confirmRollback}?` : ''}
        confirmText='Rollback'
        danger
        onCancel={() => setConfirmRollback(null)}
        onConfirm={async () => {
          if (!confirmRollback) return;
          try {
            await rollback(confirmRollback);
          } catch (e) {
            toastStore.push('error', e instanceof Error ? e.message : 'Rollback failed', 'File Ops');
          } finally {
            setConfirmRollback(null);
          }
        }}
      />
    </div>
  );
});

export default FilesPage;
