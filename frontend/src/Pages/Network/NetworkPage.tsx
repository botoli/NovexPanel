import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { apiRequest } from '../../Api/client';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import styles from './NetworkPage.module.scss';

type PortItem = { proto?: string; port?: number; process?: string; state?: string; address?: string };
type FirewallRule = { id:number; provider:string; direction:string; action:string; protocol:string; port:string; source:string; destination:string; comment:string; enabled:boolean; created_at:string };
type AlertItem = { key:string; message:string; level:string; count?:number; ip?:string };

type TrafficResponse = { points: Array<{ timestamp:string; rx:number; tx:number }>; anomalies: Array<{ timestamp:string; rx:number; tx:number }>; summary?:{ peak_rx:number; peak_tx:number } };

const NetworkPage = observer(() => {
  const [serverId, setServerId] = useState<number | ''>('');
  const [ports, setPorts] = useState<PortItem[]>([]);
  const [rules, setRules] = useState<FirewallRule[]>([]);
  const [alerts, setAlerts] = useState<AlertItem[]>([]);
  const [traffic, setTraffic] = useState<TrafficResponse>({ points: [], anomalies: [] });
  const [blocked, setBlocked] = useState<Array<{ id:number; ip:string; reason:string; created_at:string }>>([]);
  const [provider, setProvider] = useState<'ufw' | 'iptables'>('ufw');
  const [loading, setLoading] = useState(false);

  const servers = serverMetricsStore.getNowServers();

  const load = async (id: number) => {
    setLoading(true);
    try {
      const [openPorts, fw, netAlerts, netTraffic, blockedHistory] = await Promise.all([
        apiRequest<{ ports: PortItem[] }>(`/servers/${id}/network/ports`),
        apiRequest<{ managed_rules: FirewallRule[] }>(`/servers/${id}/network/firewall?provider=${provider}`),
        apiRequest<{ alerts: AlertItem[] }>(`/servers/${id}/network/alerts`),
        apiRequest<TrafficResponse>(`/servers/${id}/network/traffic`),
        apiRequest<Array<{ id:number; ip:string; reason:string; created_at:string }>>(`/servers/${id}/network/firewall/blocked`)
      ]);
      setPorts(openPorts.ports || []);
      setRules(fw.managed_rules || []);
      setAlerts(netAlerts.alerts || []);
      setTraffic(netTraffic || { points: [], anomalies: [] });
      setBlocked(blockedHistory || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!serverId && servers[0]) setServerId(servers[0].id);
  }, [servers, serverId]);

  useEffect(() => {
    if (typeof serverId === 'number') void load(serverId);
  }, [serverId, provider]);

  const createQuickRule = async (action: 'allow' | 'deny', port: number) => {
    if (typeof serverId !== 'number') return;
    if (!window.confirm(`Confirm firewall change: ${action.toUpperCase()} tcp/${port}?`)) return;
    await apiRequest(`/servers/${serverId}/network/firewall/quick-rule`, {
      method: 'POST',
      body: JSON.stringify({ provider, direction: 'inbound', action, protocol: 'tcp', port, source: 'any', destination: 'any', comment: `quick-${action}-${port}` })
    });
    await load(serverId);
  };

  const rollback = async () => {
    if (typeof serverId !== 'number') return;
    if (!window.confirm('Rollback managed firewall rules to previous snapshot?')) return;
    await apiRequest(`/servers/${serverId}/network/firewall/rollback?provider=${provider}`, { method: 'POST' });
    await load(serverId);
  };

  const anomalyCount = useMemo(() => traffic.anomalies?.length || 0, [traffic.anomalies]);

  return <div className={styles.page}><LeftPanel /><main className={styles.main}>
    <header className={styles.header}><h1>Network & Firewall</h1><div className={styles.controls}>
      <select value={serverId} onChange={(e) => setServerId(Number(e.target.value))}>{servers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select>
      <select value={provider} onChange={(e) => setProvider(e.target.value as 'ufw' | 'iptables')}><option value='ufw'>UFW</option><option value='iptables'>iptables</option></select>
      <button type='button' onClick={() => void rollback()}>Rollback rules</button>
    </div></header>
    <section className={styles.grid}>
      <article className={styles.card}><h2>Open Ports</h2><table><thead><tr><th>Port</th><th>Proto</th><th>Process</th><th>State</th><th /></tr></thead><tbody>{ports.map((p, i) => <tr key={i}><td>{p.port ?? '-'}</td><td>{p.proto ?? '-'}</td><td>{p.process ?? '-'}</td><td>{p.state ?? '-'}</td><td><button onClick={() => void createQuickRule('deny', Number(p.port || 0))}>Block</button></td></tr>)}</tbody></table></article>
      <article className={styles.card}><h2>Firewall Rules</h2><table><thead><tr><th>Action</th><th>Dir</th><th>Proto</th><th>Port</th><th>Source</th><th>Enabled</th></tr></thead><tbody>{rules.map((r) => <tr key={r.id}><td>{r.action}</td><td>{r.direction}</td><td>{r.protocol}</td><td>{r.port}</td><td>{r.source}</td><td>{r.enabled ? 'yes' : 'no'}</td></tr>)}</tbody></table><div className={styles.quickBtns}><button onClick={() => void createQuickRule('allow', 80)}>Allow 80</button><button onClick={() => void createQuickRule('allow', 443)}>Allow 443</button><button onClick={() => void createQuickRule('deny', 22)}>Deny 22</button></div></article>
      <article className={styles.card}><h2>Alerts</h2><p>Inbound anomalies: {anomalyCount}</p>{alerts.map((a) => <div key={a.key} className={styles.alert}>{a.level}: {a.message}</div>)}</article>
      <article className={styles.card}><h2>Traffic Timeline</h2><div className={styles.timeline}>{traffic.points.slice(-24).map((p) => <div key={p.timestamp}>{new Date(p.timestamp).toLocaleTimeString()} RX {Math.round(p.rx)} TX {Math.round(p.tx)}</div>)}</div></article>
      <article className={styles.card}><h2>Blocked IP history</h2>{blocked.slice(0, 20).map((b) => <div key={b.id}>{b.ip} — {b.reason}</div>)}</article>
    </section>{loading ? <div className={styles.loading}>Loading…</div> : null}
  </main></div>;
});

export default NetworkPage;
