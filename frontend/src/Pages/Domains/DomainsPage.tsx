import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { apiRequest } from '../../Api/client';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import styles from './DomainsPage.module.scss';

type Domain = { id:number; domain:string; service_name:string; port:number; tls_enabled:boolean; auto_renew:boolean; cert_status?:string; cert_expires_at?:string; validation_error?:string; issuer?:string };
type DNSDiagnostics = { a_records?: string[]; aaaa_records?: string[]; cname?: string; status?: string; expected_target?: string; message?: string };
type RenewalLog = { id:number; status:string; message:string; created_at:string; attempt?: number };

const DomainsPage = observer(() => {
  const servers = serverMetricsStore.getNowServers();
  const [serverId, setServerId] = useState<number | ''>('');
  const [domains, setDomains] = useState<Domain[]>([]);
  const [selected, setSelected] = useState<Domain | null>(null);
  const [dns, setDns] = useState<DNSDiagnostics | null>(null);
  const [renewals, setRenewals] = useState<RenewalLog[]>([]);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [tlsFilter, setTlsFilter] = useState<'all' | 'on' | 'off'>('all');

  useEffect(() => { if (!serverId && servers[0]) setServerId(servers[0].id); }, [servers, serverId]);
  useEffect(() => {
    const load = async () => {
      if (typeof serverId !== 'number') return;
      try {
        setError('');
        const list = await apiRequest<Domain[]>(`/servers/${serverId}/domains`);
        setDomains(list);
      } catch (e) {
        setError((e as Error).message);
      }
    };
    void load();
  }, [serverId]);

  const openDomain = async (item: Domain) => {
    if (typeof serverId !== 'number') return;
    setSelected(item);
    setError('');
    try {
      const [dnsDiag, renewalLogs] = await Promise.all([
        apiRequest<DNSDiagnostics>(`/servers/${serverId}/domains/${item.id}/dns`),
        apiRequest<RenewalLog[]>(`/servers/${serverId}/domains/${item.id}/renewals`)
      ]);
      setDns(dnsDiag);
      setRenewals(renewalLogs);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const runAction = async (action: 'issue' | 'renew') => {
    if (typeof serverId !== 'number' || !selected) return;
    if (!window.confirm(`${action} certificate for ${selected.domain}?`)) return;
    try {
      setError('');
      await apiRequest(`/servers/${serverId}/domains/${selected.id}/${action}`, { method: 'POST' });
      await openDomain(selected);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const filteredDomains = useMemo(() => domains.filter((item) => {
    if (tlsFilter !== 'all') {
      const isOn = tlsFilter === 'on';
      if (item.tls_enabled !== isOn) return false;
    }
    if (!query.trim()) return true;
    const q = query.toLowerCase();
    return `${item.domain} ${item.service_name} ${item.issuer || ''}`.toLowerCase().includes(q);
  }), [domains, query, tlsFilter]);

  return <div className={styles.page}><LeftPanel /><main className={styles.main}>
    <header className={styles.header}><h1>Domains & TLS</h1><select value={serverId} onChange={(e) => setServerId(Number(e.target.value))}>{servers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></header>
    {error ? <section className={styles.card}><strong>{error}</strong></section> : null}
    <section className={styles.card}><h2>Domains</h2><div className={styles.toolbar}><input className={styles.input} placeholder='Filter by domain/service' value={query} onChange={(e) => setQuery(e.target.value)} /><select value={tlsFilter} onChange={(e) => setTlsFilter(e.target.value as 'all' | 'on' | 'off')}><option value='all'>TLS all</option><option value='on'>TLS on</option><option value='off'>TLS off</option></select></div>{filteredDomains.length === 0 ? <p className={styles.empty}>No domains match the current filters.</p> : <table><thead><tr><th>Domain</th><th>Service</th><th>TLS</th><th>Expiry</th><th>Status</th><th /></tr></thead><tbody>{filteredDomains.map((d) => <tr key={d.id}><td>{d.domain}</td><td>{d.service_name}:{d.port}</td><td><span className={d.tls_enabled ? styles.ok : styles.warn}>{d.tls_enabled ? 'on' : 'off'}</span></td><td>{d.cert_expires_at ? new Date(d.cert_expires_at).toLocaleDateString() : '-'}</td><td>{d.cert_status || '-'}</td><td><button onClick={() => void openDomain(d)}>Card</button></td></tr>)}</tbody></table>}</section>
    {selected ? <section className={styles.card}><h2>Domain Card: {selected.domain}</h2><div className={styles.actions}><button type='button' onClick={() => void runAction('issue')}>Issue cert</button><button type='button' onClick={() => void runAction('renew')}>Renew cert</button><button type='button' onClick={() => void openDomain(selected)}>Refresh card</button></div><div>Connected service: {selected.service_name}:{selected.port}</div><div>Issuer: {selected.issuer || '-'}</div><div>Auto-renew: {selected.auto_renew ? 'enabled' : 'disabled'}</div><div>Validation error: {selected.validation_error || '-'}</div><h3>DNS diagnostics</h3><div>Status: {dns?.status || '-'}</div><div>Expected target: {dns?.expected_target || '-'}</div><div>A records: {dns?.a_records?.join(', ') || '-'}</div><div>AAAA records: {dns?.aaaa_records?.join(', ') || '-'}</div><div>CNAME: {dns?.cname || '-'}</div><div>Message: {dns?.message || '-'}</div><h3>Renewal logs</h3>{renewals.length === 0 ? <p className={styles.empty}>No renewal logs yet.</p> : <table><thead><tr><th>Time</th><th>Status</th><th>Attempt</th><th>Message</th></tr></thead><tbody>{renewals.slice(0, 20).map((row) => <tr key={row.id}><td>{new Date(row.created_at).toLocaleString()}</td><td>{row.status}</td><td>{row.attempt ?? '-'}</td><td>{row.message}</td></tr>)}</tbody></table>}</section> : null}
  </main></div>;
});

export default DomainsPage;
