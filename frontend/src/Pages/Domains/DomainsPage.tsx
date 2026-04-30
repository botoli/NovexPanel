import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { apiRequest } from '../../Api/client';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import styles from './DomainsPage.module.scss';

type Domain = { id:number; domain:string; service_name:string; port:number; tls_enabled:boolean; auto_renew:boolean; cert_status?:string; cert_expires_at?:string; validation_error?:string; issuer?:string };

const DomainsPage = observer(() => {
  const servers = serverMetricsStore.getNowServers();
  const [serverId, setServerId] = useState<number | ''>('');
  const [domains, setDomains] = useState<Domain[]>([]);
  const [selected, setSelected] = useState<Domain | null>(null);
  const [dns, setDns] = useState<any>(null);
  const [renewals, setRenewals] = useState<any[]>([]);

  useEffect(() => { if (!serverId && servers[0]) setServerId(servers[0].id); }, [servers, serverId]);
  useEffect(() => {
    const load = async () => {
      if (typeof serverId !== 'number') return;
      const list = await apiRequest<Domain[]>(`/servers/${serverId}/domains`);
      setDomains(list);
    };
    void load();
  }, [serverId]);

  const openDomain = async (item: Domain) => {
    if (typeof serverId !== 'number') return;
    setSelected(item);
    const [dnsDiag, renewalLogs] = await Promise.all([
      apiRequest(`/servers/${serverId}/domains/${item.id}/dns`),
      apiRequest<any[]>(`/servers/${serverId}/domains/${item.id}/renewals`)
    ]);
    setDns(dnsDiag);
    setRenewals(renewalLogs);
  };

  return <div className={styles.page}><LeftPanel /><main className={styles.main}>
    <header className={styles.header}><h1>Domains & TLS</h1><select value={serverId} onChange={(e) => setServerId(Number(e.target.value))}>{servers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></header>
    <section className={styles.card}><h2>Domains</h2><table><thead><tr><th>Domain</th><th>Service</th><th>TLS</th><th>Expiry</th><th>Status</th><th /></tr></thead><tbody>{domains.map((d) => <tr key={d.id}><td>{d.domain}</td><td>{d.service_name}:{d.port}</td><td>{d.tls_enabled ? 'on' : 'off'}</td><td>{d.cert_expires_at ? new Date(d.cert_expires_at).toLocaleDateString() : '-'}</td><td>{d.cert_status || '-'}</td><td><button onClick={() => void openDomain(d)}>Card</button></td></tr>)}</tbody></table></section>
    {selected ? <section className={styles.card}><h2>Domain Card: {selected.domain}</h2><div>Connected service: {selected.service_name}:{selected.port}</div><div>Issuer: {selected.issuer || '-'}</div><div>Auto-renew: {selected.auto_renew ? 'enabled' : 'disabled'}</div><div>Validation error: {selected.validation_error || '-'}</div><h3>DNS diagnostics</h3><pre>{JSON.stringify(dns, null, 2)}</pre><h3>Renewal logs</h3><pre>{JSON.stringify(renewals.slice(0, 20), null, 2)}</pre></section> : null}
  </main></div>;
});

export default DomainsPage;
