import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { API_BASE } from '../../../Api/api';
import { Confirm } from '../../../modals/Confirm/Confirm';
import { useCurrentServer } from '../../../Store/ServerStore';
import { toastStore } from '../../../Store/ToastStore';
import { tokenStore } from '../../../Store/TokenStore';
import styles from './ServicesPage.module.scss';

type Provider = 'systemd' | 'supervisor' | 'docker-compose';

type ServiceRow = {
  name: string;
  status: string;
  cpu: number;
  ram: number;
  uptime: string;
  ports: string[];
  process_id: number;
  restart_count: number;
  last_crash: string;
  linked_domains: string[];
  autorestart_policy: string;
  health_status: string;
  crash_reason: string;
};

const ServicesPage = observer(() => {
  const { serverId } = useCurrentServer();
  const [provider, setProvider] = useState<Provider>('systemd');
  const [services, setServices] = useState<ServiceRow[]>([]);
  const [selected, setSelected] = useState<ServiceRow | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [graphNodes, setGraphNodes] = useState<string[]>([]);
  const [history, setHistory] = useState<string[]>([]);
  const [audit, setAudit] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [confirm, setConfirm] = useState<
    { action: 'stop' | 'restart' | 'reload'; service: string; } | null
  >(null);

  const buildAuthHeaders = () => ({ Authorization: `Bearer ${tokenStore.getToken()}` });

  const loadServices = async (silent = false) => {
    if (!Number.isFinite(serverId)) return;
    if (!silent) setLoading(true);
    try {
      const response = await fetch(
        `${API_BASE}/servers/${serverId}/services?provider=${provider}`,
        { headers: buildAuthHeaders() },
      );
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const payload = await response.json();
      const list: ServiceRow[] = Array.isArray(payload?.services) ? payload.services as ServiceRow[] : [];
      setServices(list);
      const nextSelected = list.find((item: ServiceRow) => item.name === selected?.name) || list[0] || null;
      setSelected(nextSelected);
      if (!nextSelected) {
        setLogs([]);
        setHistory([]);
      }
    } catch (e) {
      setServices([]);
      setSelected(null);
      toastStore.push(
        'error',
        e instanceof Error ? e.message : 'Failed to load services',
        'Service Manager',
      );
    } finally {
      if (!silent) setLoading(false);
    }
  };

  const loadAudit = async () => {
    if (!Number.isFinite(serverId)) return;
    const response = await fetch(`${API_BASE}/servers/${serverId}/services/audit?limit=50`, {
      headers: buildAuthHeaders(),
    });
    if (!response.ok) return;
    const payload = await response.json();
    setAudit(Array.isArray(payload) ? payload : []);
  };

  const loadDependencies = async () => {
    if (!Number.isFinite(serverId)) return;
    const response = await fetch(
      `${API_BASE}/servers/${serverId}/services/dependencies?provider=${provider}`,
      { headers: buildAuthHeaders() },
    );
    if (!response.ok) return;
    const payload = await response.json();
    setGraphNodes(Array.isArray(payload?.nodes) ? payload.nodes : []);
  };

  const loadLogsAndHistory = async (serviceName: string) => {
    if (!Number.isFinite(serverId)) return;
    const [logsResp, historyResp] = await Promise.all([
      fetch(
        `${API_BASE}/servers/${serverId}/services/logs?provider=${provider}&service=${
          encodeURIComponent(serviceName)
        }&lines=200`,
        { headers: buildAuthHeaders() },
      ),
      fetch(
        `${API_BASE}/servers/${serverId}/services/restart-history?provider=${provider}&service=${
          encodeURIComponent(serviceName)
        }`,
        { headers: buildAuthHeaders() },
      ),
    ]);
    if (logsResp.ok) {
      const payload = await logsResp.json();
      setLogs(Array.isArray(payload?.logs) ? payload.logs : []);
    } else {
      setLogs([]);
    }
    if (historyResp.ok) {
      const payload = await historyResp.json();
      setHistory(Array.isArray(payload?.history) ? payload.history : []);
    } else {
      setHistory([]);
    }
  };

  useEffect(() => {
    void loadServices();
    void loadDependencies();
    void loadAudit();
    const id = setInterval(() => {
      void loadServices(true);
    }, 3500);
    return () => clearInterval(id);
  }, [serverId, provider]);

  useEffect(() => {
    if (!selected) return;
    void loadLogsAndHistory(selected.name);
  }, [selected?.name, provider]);

  const serviceAction = async (
    serviceName: string,
    action: 'start' | 'stop' | 'restart' | 'reload',
  ) => {
    if (!Number.isFinite(serverId)) return;
    const response = await fetch(`${API_BASE}/servers/${serverId}/services/action`, {
      method: 'POST',
      headers: { ...buildAuthHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider,
        service: serviceName,
        action,
        graceful: action === 'restart' || action === 'reload',
      }),
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    await Promise.all([loadServices(true), loadAudit()]);
    if (selected?.name === serviceName) {
      await loadLogsAndHistory(serviceName);
    }
  };

  if (!Number.isFinite(serverId)) return null;

  const statusTone = (value: string) => {
    const normalized = (value || '').toLowerCase();
    if (['running', 'active', 'ok', 'healthy'].includes(normalized)) return styles.toneSuccess;
    if (['failed', 'error', 'dead', 'unhealthy', 'crashed'].includes(normalized)) {
      return styles.toneDanger;
    }
    if (['reloading', 'starting', 'stopping'].includes(normalized)) return styles.toneWarn;
    return styles.toneNeutral;
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h2>Service Manager</h2>
        <div className={styles.actions}>
          <select
            value={provider}
            className={styles.select}
            onChange={e => setProvider(e.target.value as Provider)}
          >
            <option value='systemd'>systemd</option>
            <option value='supervisor'>supervisor</option>
            <option value='docker-compose'>docker-compose</option>
          </select>
          <button type='button' className={styles.btn} onClick={() => void loadServices()}>
            Refresh
          </button>
        </div>
      </div>

      <div className={styles.layout}>
        <section className={styles.card}>
          <h3>Services</h3>
          {loading ? <p>Loading...</p> : null}
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Health</th>
                  <th>Uptime</th>
                </tr>
              </thead>
              <tbody>
                {services.map(service => (
                  <tr
                    key={service.name}
                    onClick={() => setSelected(service)}
                    className={`${styles.tableRow} ${
                      selected?.name === service.name ? styles.activeRow : ''
                    }`}
                  >
                    <td>{service.name}</td>
                    <td>
                      <span className={`${styles.statusPill} ${statusTone(service.status)}`}>
                        {service.status}
                      </span>
                    </td>
                    <td>
                      <span
                        className={`${styles.statusPill} ${
                          statusTone(service.health_status || service.status)
                        }`}
                      >
                        {service.health_status || service.status}
                      </span>
                    </td>
                    <td>{service.uptime || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className={styles.card}>
          <h3>Service Card</h3>
          {selected
            ? (
              <div className={styles.metrics}>
                <div className={styles.metricItem}>
                  <span>Status</span>
                  <span className={styles.metricValue}>{selected.status}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>CPU</span>
                  <span className={styles.metricValue}>{selected.cpu ?? 0}%</span>
                </div>
                <div className={styles.metricItem}>
                  <span>RAM</span>
                  <span className={styles.metricValue}>{selected.ram ?? 0}%</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Uptime</span>
                  <span className={styles.metricValue}>{selected.uptime || '—'}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Ports</span>
                  <span className={styles.metricValue}>
                    {(selected.ports || []).join(', ') || '—'}
                  </span>
                </div>
                <div className={styles.metricItem}>
                  <span>Process ID</span>
                  <span className={styles.metricValue}>{selected.process_id || '—'}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Restart count</span>
                  <span className={styles.metricValue}>{selected.restart_count || 0}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Last crash</span>
                  <span className={styles.metricValue}>{selected.last_crash || '—'}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Crash reason</span>
                  <span className={styles.metricValue}>{selected.crash_reason || '—'}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Autorestart</span>
                  <span className={styles.metricValue}>{selected.autorestart_policy || '—'}</span>
                </div>
                <div className={styles.metricItem}>
                  <span>Domains</span>
                  <span className={styles.metricValue}>
                    {(selected.linked_domains || []).join(', ') || '—'}
                  </span>
                </div>
              </div>
            )
            : <p>Select service</p>}
          <div className={styles.row}>
            <button
              type='button'
              className={styles.btn}
              disabled={!selected}
              onClick={() => selected && serviceAction(selected.name, 'start')}
            >
              Start
            </button>
            <button
              type='button'
              className={styles.btn}
              disabled={!selected}
              onClick={() => selected && setConfirm({ action: 'stop', service: selected.name })}
            >
              Stop
            </button>
            <button
              type='button'
              className={styles.btn}
              disabled={!selected}
              onClick={() => selected && setConfirm({ action: 'restart', service: selected.name })}
            >
              Restart
            </button>
            <button
              type='button'
              className={styles.btn}
              disabled={!selected}
              onClick={() => selected && setConfirm({ action: 'reload', service: selected.name })}
            >
              Reload
            </button>
          </div>
        </section>
      </div>

      <div className={styles.layout}>
        <section className={styles.card}>
          <h3>Logs</h3>
          <div className={styles.logs}>
            {logs.map((line, idx) => <div key={idx} className={styles.logLine}>{line}</div>)}
          </div>
        </section>
        <section className={styles.card}>
          <h3>Restart History</h3>
          <div className={styles.logs}>
            {history.map((line, idx) => <div key={idx} className={styles.logLine}>{line}</div>)}
          </div>
        </section>
      </div>

      <div className={styles.layout}>
        <section className={styles.card}>
          <h3>Dependencies Graph</h3>
          <div className={styles.logs}>
            {graphNodes.map((node, idx) => <div key={idx} className={styles.logLine}>{node}</div>)}
          </div>
        </section>
        <section className={styles.card}>
          <h3>Action Logs / Audit Trail</h3>
          <div className={styles.logs}>
            {audit.map(item => (
              <div key={item.id} className={styles.logLine}>
                [{item.provider}] {item.service_name} {item.action}{' '}
                {item.success ? 'ok' : `failed: ${item.error_message || 'error'}`}
              </div>
            ))}
          </div>
        </section>
      </div>

      <Confirm
        isOpen={confirm != null}
        title={confirm ? `${confirm.action} service` : ''}
        description={confirm
          ? `Are you sure you want to ${confirm.action} ${confirm.service}?`
          : ''}
        confirmText={confirm?.action || 'Confirm'}
        danger={confirm?.action === 'stop'}
        onCancel={() => setConfirm(null)}
        onConfirm={async () => {
          if (!confirm) return;
          try {
            await serviceAction(confirm.service, confirm.action);
            toastStore.push('success', `${confirm.action} requested`, 'Service Manager');
          } catch (e) {
            toastStore.push(
              'error',
              e instanceof Error ? e.message : 'Action failed',
              'Service Manager',
            );
          } finally {
            setConfirm(null);
          }
        }}
      />
    </div>
  );
});

export default ServicesPage;
