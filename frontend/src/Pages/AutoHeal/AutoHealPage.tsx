import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import { createRule, fetchHistory, fetchRules, type AutoHealRule } from '../../Api/features/autoHeal';
import styles from './AutoHealPage.module.scss';

const defaultRule: Omit<AutoHealRule, 'id'> = { name:'', metric:'service_down_seconds', condition:'gt', threshold:30, duration_seconds:30, action:'restart', retry_limit:2, cooldown_seconds:300, enabled:true };

const AutoHealPage = observer(() => {
  const servers = serverMetricsStore.getNowServers();
  const [serverId, setServerId] = useState<number>(servers[0]?.id ?? 0);
  const [rules, setRules] = useState<AutoHealRule[]>([]);
  const [history, setHistory] = useState<Array<{ id:number; rule_name:string; status:string; message:string; created_at:string }>>([]);
  const [form, setForm] = useState(defaultRule);
  const [state, setState] = useState<'loading' | 'success' | 'error' | 'empty'>('loading');
  const [error, setError] = useState('');
  const [metricFilter, setMetricFilter] = useState('');
  const [actionFilter, setActionFilter] = useState<'all' | AutoHealRule['action']>('all');
  const [historyFilter, setHistoryFilter] = useState<'all' | 'created' | 'success' | 'failed'>('all');

  const load = async () => {
    setState('loading');
    try {
      const [r, h] = await Promise.all([fetchRules(serverId), fetchHistory(serverId)]);
      setRules(r); setHistory(h); setState(r.length ? 'success' : 'empty');
      setError('');
    } catch (e) {
      setState('error');
      setError((e as Error).message);
    }
  };

  useEffect(() => { if (serverId) void load(); }, [serverId]);

  const onCreate = async () => {
    if (!form.name.trim()) return;
    if (!window.confirm('Create auto-heal rule?')) return;
    try {
      await createRule(serverId, form);
      setForm(defaultRule);
      await load();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const filteredRules = useMemo(
    () => rules.filter((rule) => {
      if (actionFilter !== 'all' && rule.action !== actionFilter) return false;
      if (metricFilter && !rule.metric.toLowerCase().includes(metricFilter.toLowerCase())) return false;
      return true;
    }),
    [rules, actionFilter, metricFilter],
  );

  const filteredHistory = useMemo(
    () => history.filter((item) => {
      if (historyFilter === 'all') return true;
      if (historyFilter === 'created') return item.status.toLowerCase() === 'created';
      if (historyFilter === 'success') return ['ok', 'success'].includes(item.status.toLowerCase());
      return ['failed', 'error'].includes(item.status.toLowerCase());
    }),
    [history, historyFilter],
  );

  const statusClass = (status: string) => {
    const value = status.toLowerCase();
    if (['ok', 'success', 'created'].includes(value)) return styles.badgeOk;
    if (['failed', 'error'].includes(value)) return styles.badgeError;
    return styles.badgeNeutral;
  };

  return <div className={styles.page}><LeftPanel /><main className={styles.main}><header className={styles.header}><h1>Auto-Heal Rules</h1><select value={serverId} onChange={(e) => setServerId(Number(e.target.value))}>{servers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></header>
  <section className={styles.card}><h2>Rule builder</h2><div className={styles.form}><input placeholder='Rule name' value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /><input placeholder='Metric' value={form.metric} onChange={(e) => setForm({ ...form, metric: e.target.value })} /><input type='number' value={form.threshold} onChange={(e) => setForm({ ...form, threshold: Number(e.target.value) })} /><input type='number' value={form.duration_seconds} onChange={(e) => setForm({ ...form, duration_seconds: Number(e.target.value) })} /><select value={form.action} onChange={(e) => setForm({ ...form, action: e.target.value as AutoHealRule['action'] })}><option value='restart'>restart</option><option value='reload'>reload</option><option value='runbook'>runbook</option><option value='alert'>alert</option><option value='rollback_deploy'>rollback deploy</option><option value='block_traffic'>block traffic</option></select><input type='number' value={form.retry_limit} onChange={(e) => setForm({ ...form, retry_limit: Number(e.target.value) })} /><input type='number' value={form.cooldown_seconds} onChange={(e) => setForm({ ...form, cooldown_seconds: Number(e.target.value) })} /><button onClick={() => void onCreate()}>Create rule</button><button onClick={() => setForm(defaultRule)} type='button'>Reset</button><button onClick={() => void load()} type='button'>Refresh</button></div></section>
  <section className={styles.grid}><article className={styles.card}><h2>Rules list</h2><div className={styles.filters}><input placeholder='Filter by metric' value={metricFilter} onChange={(e) => setMetricFilter(e.target.value)} /><select value={actionFilter} onChange={(e) => setActionFilter(e.target.value as 'all' | AutoHealRule['action'])}><option value='all'>All actions</option><option value='restart'>restart</option><option value='reload'>reload</option><option value='runbook'>runbook</option><option value='alert'>alert</option><option value='rollback_deploy'>rollback deploy</option><option value='block_traffic'>block traffic</option></select></div>{state === 'loading' ? <p>Loading…</p> : null}{state === 'error' ? <p className={styles.errorText}>Error loading rules. {error}</p> : null}{state === 'empty' ? <p className={styles.empty}>No rules yet. Create your first policy above.</p> : null}{filteredRules.length === 0 && state === 'success' ? <p className={styles.empty}>No rules match the selected filters.</p> : null}{filteredRules.map((r) => <div key={r.id} className={styles.row}><span><strong>{r.name}</strong> · {r.metric} {r.condition} {r.threshold}</span><span className={`${styles.badge} ${r.enabled ? styles.badgeOk : styles.badgeNeutral}`}>{r.enabled ? 'enabled' : 'disabled'}</span><span>{r.action}</span><span>retries {r.retry_limit}</span><span>cooldown {r.cooldown_seconds}s</span></div>)}</article>
  <article className={styles.card}><h2>Execution history</h2><div className={styles.filters}><select value={historyFilter} onChange={(e) => setHistoryFilter(e.target.value as 'all' | 'created' | 'success' | 'failed')}><option value='all'>All statuses</option><option value='created'>created</option><option value='success'>success</option><option value='failed'>failed</option></select></div>{history.length === 0 ? <p className={styles.empty}>History is empty.</p> : filteredHistory.map((h) => <div key={h.id} className={styles.row}><span>{new Date(h.created_at).toLocaleString()}</span><span>{h.rule_name}</span><span className={`${styles.badge} ${statusClass(h.status)}`}>{h.status}</span><span>{h.message}</span></div>)}</article></section></main></div>;
});

export default AutoHealPage;
