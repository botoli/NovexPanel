import { Icon } from '@iconify/react';

import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { refreshStore } from '../../../Store/RefreshStore';
import { toastStore } from '../../../Store/ToastStore';
import { API_BASE } from '../../../Api/api';
import { Confirm } from '../../../modals/Confirm/Confirm';
import { tokenStore } from '../../../Store/TokenStore';
import styles from './ProcessesPage.module.scss';

type ProcessTone = 'calm' | 'watch' | 'hot';

type ProcessRow = {
  pid: number;
  name: string;
  cpu: number;
  mem: number;
  state?: string;
  user?: string;
  uptime?: number;
  threads?: number;
  ppid?: number;
  has_children?: boolean;
  start_time?: string;
  type?: 'system' | 'user' | string;
};

type ProcessAction = 'stop' | 'restart' | 'kill';

const getProcessTone = (cpu: number, mem: number): ProcessTone => {
  if (cpu >= 70 || mem >= 70) return 'hot';
  if (cpu >= 35 || mem >= 35) return 'watch';
  return 'calm';
};

const formatUptime = (seconds?: number) => {
  if (!seconds || !Number.isFinite(seconds) || seconds <= 0) return '—';
  const s = Math.floor(seconds);
  const days = Math.floor(s / 86400);
  const hours = Math.floor((s % 86400) / 3600);
  const minutes = Math.floor((s % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
};

const isStoppedHeuristic = (state?: string) => {
  const s = (state || '').toLowerCase();
  return s.includes('z') || s.includes('stop') || s.includes('dead') || s.includes('idle');
};

const TONE_LABEL: Record<ProcessTone, string> = {
  calm: 'Stable',
  watch: 'Watch',
  hot: 'High',
};

const TONE_CLASS: Record<ProcessTone, string> = {
  calm: styles.toneCalm,
  watch: styles.toneWatch,
  hot: styles.toneHot,
};

const ProcessesPage = observer(() => {
  const [searchText, setSearchText] = useState<string>('');
  const [pidQuery, setPidQuery] = useState<string>('');
  const [filterType, setFilterType] = useState<'all' | 'system' | 'user'>('all');
  const [filterState, setFilterState] = useState<'all' | 'active' | 'stopped'>('all');
  const [sortKey, setSortKey] = useState<'cpu' | 'mem' | 'name' | 'uptime'>('cpu');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const [viewMode, setViewMode] = useState<'list' | 'tree'>('tree');
  const [page, setPage] = useState(1);
  const pageSize = 100;
  const [confirm, setConfirm] = useState<{ action: ProcessAction; pid: number; name: string; } | null>(null);

  const { id } = useParams<{ id?: string; }>();
  const serverId = id ? Number(id) : Number.NaN;

  const [rows, setRows] = useState<ProcessRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchProcesses = useCallback(async (opts?: { silent?: boolean; }) => {
    const silent = Boolean(opts?.silent);
    try {
      if (!silent) setLoading(true);
      setError(null);
      const resp = await fetch(`${API_BASE}/servers/${serverId}/processes?limit=500`, {
        headers: { Authorization: `Bearer ${tokenStore.getToken()}` },
      });
      if (!resp.ok) {
        let msg = `HTTP ${resp.status}`;
        try {
          const body = await resp.json();
          if (body?.error) msg = String(body.error);
        } catch {
          // ignore
        }
        throw new Error(msg);
      }
      const data = await resp.json();
      const list = Array.isArray(data?.processes) ? (data.processes as ProcessRow[]) : [];
      setRows(list);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to load processes';
      setError(msg);
      if (!silent) toastStore.push('error', msg, 'Processes');
    } finally {
      if (!silent) setLoading(false);
    }
  }, [serverId]);

  useEffect(() => {
    if (!Number.isFinite(serverId)) return;
    void fetchProcesses();
    const interval = setInterval(() => fetchProcesses({ silent: true }), 4000);
    return () => clearInterval(interval);
  }, [fetchProcesses, serverId]);

  useEffect(() => {
    if (!Number.isFinite(serverId)) return;
    void fetchProcesses();
  }, [refreshStore.refreshKey, fetchProcesses, serverId]);

  const filteredProcesses = useMemo(() => {
    let list = rows;

    const pidFilter = pidQuery.trim();
    if (pidFilter) {
      const n = Number(pidFilter);
      if (Number.isFinite(n)) {
        list = list.filter(p => p.pid === n);
      }
    }

    const q = searchText.trim().toLowerCase();
    if (q) {
      list = list.filter(p => (p.name || '').toLowerCase().includes(q));
    }

    if (filterType !== 'all') {
      list = list.filter(p => (p.type || 'user') === filterType);
    }

    if (filterState !== 'all') {
      list = list.filter(p => (filterState === 'stopped' ? isStoppedHeuristic(p.state) : !isStoppedHeuristic(p.state)));
    }

    const sorted = [...list].sort((a, b) => {
      const dir = sortDir === 'asc' ? 1 : -1;
      const get = (p: ProcessRow) => {
        if (sortKey === 'cpu') return p.cpu ?? 0;
        if (sortKey === 'mem') return p.mem ?? 0;
        if (sortKey === 'uptime') return p.uptime ?? 0;
        return (p.name ?? '').toLowerCase();
      };
      const av = get(a);
      const bv = get(b);
      if (typeof av === 'string' && typeof bv === 'string') return av.localeCompare(bv) * dir;
      return ((av as number) - (bv as number)) * dir;
    });

    return sorted;
  }, [rows, pidQuery, searchText, filterType, filterState, sortKey, sortDir]);

  const tree = useMemo(() => {
    const byPid = new Map<number, ProcessRow>();
    const children = new Map<number, ProcessRow[]>();
    const roots: ProcessRow[] = [];

    for (const p of filteredProcesses) {
      byPid.set(p.pid, p);
    }

    for (const p of filteredProcesses) {
      const parentPid = typeof p.ppid === 'number' ? p.ppid : 0;
      if (parentPid > 0 && byPid.has(parentPid)) {
        if (!children.has(parentPid)) children.set(parentPid, []);
        children.get(parentPid)!.push(p);
      } else {
        roots.push(p);
      }
    }

    for (const [pid, list] of children) {
      list.sort((a, b) => (a.pid - b.pid));
      children.set(pid, list);
    }

    roots.sort((a, b) => (a.pid - b.pid));
    return { roots, children };
  }, [filteredProcesses]);

  const [expanded, setExpanded] = useState<Set<number>>(() => new Set());

  useEffect(() => {
    // Auto-expand first level if we are in tree mode.
    if (viewMode !== 'tree') return;
    const next = new Set<number>();
    for (const p of tree.roots) {
      if (p.has_children) next.add(p.pid);
    }
    setExpanded(next);
  }, [tree.roots, viewMode]);

  const flattenedTreeItems = useMemo(() => {
    if (viewMode !== 'tree') return filteredProcesses.map(p => ({ p, depth: 0 }));
    const out: Array<{ p: ProcessRow; depth: number; }> = [];
    const visit = (p: ProcessRow, depth: number) => {
      out.push({ p, depth });
      const kids = tree.children.get(p.pid) || [];
      if (kids.length === 0) return;
      if (!expanded.has(p.pid)) return;
      for (const c of kids) visit(c, depth + 1);
    };
    for (const r of tree.roots) visit(r, 0);
    return out;
  }, [tree, expanded, filteredProcesses, viewMode]);

  const pagedProcesses = useMemo(() => {
    const total = flattenedTreeItems.length;
    const pageCount = Math.max(1, Math.ceil(total / pageSize));
    const safePage = Math.min(page, pageCount);
    const start = (safePage - 1) * pageSize;
    return {
      page: safePage,
      pageCount,
      items: flattenedTreeItems.slice(start, start + pageSize),
      total,
    };
  }, [flattenedTreeItems, page]);

  if (!Number.isFinite(serverId)) {
    return <div className={styles.stateMessage}>Invalid server id</div>;
  }
  const hasProcesses = pagedProcesses.items.length > 0;

  return (
    <section className={styles.processes}>
      <header className={styles.header}>
        <div className={styles.heading}>
          <h2 className={styles.title}>
            <Icon icon='mdi:format-list-bulleted-square' className={styles.titleIcon} />
            Processes
          </h2>

          <p className={styles.subtitle}>Live list of processes running on the server.</p>
        </div>

        <div className={styles.toolbar}>
          <label className={styles.search} aria-label='Filter processes by name'>
            <Icon icon='mdi:magnify' />
            <input
              type='text'
              placeholder='Filter by name...'
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
            />
          </label>

          <label className={styles.search} aria-label='Filter processes by PID'>
            <Icon icon='mdi:numeric' />
            <input
              type='text'
              placeholder='PID...'
              value={pidQuery}
              onChange={(e) => setPidQuery(e.target.value)}
              inputMode='numeric'
            />
          </label>

          <select
            value={filterType}
            onChange={(e) => setFilterType(e.target.value as any)}
            className={styles.refreshBtn}
            aria-label='Filter by type'
          >
            <option value='all'>All</option>
            <option value='user'>User</option>
            <option value='system'>System</option>
          </select>

          <select
            value={filterState}
            onChange={(e) => setFilterState(e.target.value as any)}
            className={styles.refreshBtn}
            aria-label='Filter by state'
          >
            <option value='all'>All states</option>
            <option value='active'>Active</option>
            <option value='stopped'>Stopped</option>
          </select>

          <select
            value={`${sortKey}:${sortDir}`}
            onChange={(e) => {
              const [k, d] = e.target.value.split(':');
              setSortKey(k as any);
              setSortDir(d as any);
            }}
            className={styles.refreshBtn}
            aria-label='Sort processes'
          >
            <option value='cpu:desc'>CPU (high)</option>
            <option value='cpu:asc'>CPU (low)</option>
            <option value='mem:desc'>Memory (high)</option>
            <option value='mem:asc'>Memory (low)</option>
            <option value='uptime:desc'>Uptime (long)</option>
            <option value='uptime:asc'>Uptime (short)</option>
            <option value='name:asc'>Name (A-Z)</option>
            <option value='name:desc'>Name (Z-A)</option>
          </select>

          <button
            type='button'
            className={styles.refreshBtn}
            onClick={() => setViewMode(m => (m === 'tree' ? 'list' : 'tree'))}
            aria-label='Toggle tree view'
          >
            <Icon icon={viewMode === 'tree' ? 'mdi:file-tree' : 'mdi:view-list'} />
            {viewMode === 'tree' ? 'Tree' : 'List'}
          </button>

          <button
            type='button'
            className={styles.refreshBtn}
            disabled={refreshStore.refreshing || loading}
            onClick={() => {
              void refreshStore.run(async () => {
                await fetchProcesses();
              }, 'server').catch((err) => {
                toastStore.push('error', err instanceof Error ? err.message : 'Refresh failed', 'Processes');
              });
            }}
          >
            <Icon icon='mdi:refresh' />
            {refreshStore.refreshing || loading ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>
      </header>

      <div className={styles.metaRow}>
        <span className={styles.metaPill}>
          <Icon icon='mdi:layers-triple-outline' />
          {pagedProcesses.total} total
        </span>
        <span className={styles.metaHint}>
          Showing {pagedProcesses.items.length} / {pagedProcesses.total}. Page {pagedProcesses.page}/{pagedProcesses.pageCount}.
        </span>
      </div>

      <div className={styles.tableCard}>
        {error ? <div className={styles.stateMessage}>{error}</div> : null}
        {hasProcesses
          ? (
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.pidCol}>PID</th>
                  <th>Name</th>
                  <th className={styles.numCol}>CPU</th>
                  <th className={styles.numCol}>MEM</th>
                  <th className={styles.stateCol}>State</th>
                  <th>User</th>
                  <th className={styles.numCol}>Uptime</th>
                  <th className={styles.numCol}>Threads</th>
                  <th className={styles.pidCol}>PPID</th>
                  <th className={styles.actionCol}>Action</th>
                </tr>
              </thead>
              <tbody>
                {pagedProcesses.items.map(({ p: process, depth }) => {
                  const tone = getProcessTone(process.cpu, process.mem);
                  const hasChildren = Boolean(process.has_children) || (tree.children.get(process.pid)?.length ?? 0) > 0;
                  const isExpanded = expanded.has(process.pid);
                  const isSystem = (process.type || 'user') === 'system';
                  const cpuHot = (process.cpu ?? 0) >= 70;
                  const memHot = (process.mem ?? 0) >= 70;

                  return (
                    <tr key={process.pid}>
                      <td className={styles.pidCell}>{process.pid}</td>
                      <td className={styles.nameCell}>
                        <div className={styles.nameTreeCell}>
                          {viewMode === 'tree'
                            ? (
                              <span className={styles.treeIndent} style={{ width: depth * 14 }} aria-hidden='true' />
                            )
                            : null}
                          {viewMode === 'tree'
                            ? (
                              <button
                                type='button'
                                className={styles.expander}
                                disabled={!hasChildren}
                                aria-label={hasChildren ? (isExpanded ? 'Collapse' : 'Expand') : 'No child processes'}
                                onClick={() => {
                                  if (!hasChildren) return;
                                  setExpanded((prev) => {
                                    const next = new Set(prev);
                                    if (next.has(process.pid)) next.delete(process.pid);
                                    else next.add(process.pid);
                                    return next;
                                  });
                                }}
                              >
                                <Icon icon={hasChildren ? (isExpanded ? 'mdi:chevron-down' : 'mdi:chevron-right') : 'mdi:minus'} />
                              </button>
                            )
                            : null}
                          <span className={styles.processName}>{process.name}</span>
                          <span className={`${styles.typeBadge} ${isSystem ? styles.typeSystem : styles.typeUser}`}>
                            {isSystem ? 'system' : 'user'}
                          </span>
                          {hasChildren ? <span className={styles.childBadge}>child</span> : null}
                        </div>
                      </td>
                      <td className={`${styles.numCell} ${cpuHot ? styles.hotNum : ''}`}>
                        {process.cpu.toFixed(0)}%
                      </td>
                      <td className={`${styles.numCell} ${memHot ? styles.hotNum : ''}`}>
                        {process.mem.toFixed(0)}%
                      </td>
                      <td className={styles.stateCell}>
                        <span className={`${styles.stateBadge} ${TONE_CLASS[tone]}`}>
                          <span className={styles.stateDot} />
                          {TONE_LABEL[tone]}
                        </span>
                      </td>
                      <td className={styles.nameCell}>{process.user || '—'}</td>
                      <td className={styles.numCell}>{formatUptime(process.uptime)}</td>
                      <td className={styles.numCell}>{process.threads ?? '—'}</td>
                      <td className={styles.pidCell}>{process.ppid ?? '—'}</td>
                      <td className={styles.actionCell}>
                        <button
                          type='button'
                          className={styles.killBtn}
                          onClick={() => setConfirm({ action: 'stop', pid: process.pid, name: process.name })}
                        >
                          <Icon icon='mdi:pause' className={styles.btnIcon} />
                          Stop
                        </button>
                        <button
                          type='button'
                          className={styles.killBtn}
                          onClick={() => setConfirm({ action: 'restart', pid: process.pid, name: process.name })}
                        >
                          <Icon icon='mdi:restart' className={styles.btnIcon} />
                          Restart
                        </button>
                        <button
                          type='button'
                          className={styles.killBtn}
                          onClick={() => setConfirm({ action: 'kill', pid: process.pid, name: process.name })}
                        >
                          <Icon icon='mdi:close-thick' className={styles.btnIcon} />
                          Kill
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )
          : (
            <div className={styles.emptyState}>
              <Icon icon='mdi:file-search-outline' />
              {loading ? 'Loading processes...' : 'No processes found for this filter.'}
            </div>
          )}

        {pagedProcesses.pageCount > 1
          ? (
            <div className={styles.metaRow}>
              <button
                type='button'
                className={styles.refreshBtn}
                disabled={pagedProcesses.page <= 1}
                onClick={() => setPage(p => Math.max(1, p - 1))}
              >
                Prev
              </button>
              <button
                type='button'
                className={styles.refreshBtn}
                disabled={pagedProcesses.page >= pagedProcesses.pageCount}
                onClick={() => setPage(p => Math.min(pagedProcesses.pageCount, p + 1))}
              >
                Next
              </button>
            </div>
          )
          : null}
      </div>

      <Confirm
        isOpen={confirm != null}
        title={
          confirm
            ? `${confirm.action === 'kill' ? 'Kill' : confirm.action === 'restart' ? 'Restart' : 'Stop'} process`
            : ''
        }
        description={
          confirm
            ? (
              <div>
                <div>
                  <strong>{confirm.name}</strong> (PID {confirm.pid})
                </div>
                <div style={{ marginTop: 8, color: 'rgba(255,255,255,0.75)' }}>
                  {confirm.action === 'kill'
                    ? 'This will forcibly terminate the process.'
                    : confirm.action === 'restart'
                    ? 'This will send a terminate signal; a supervisor may restart it.'
                    : 'This will try to stop the process gracefully.'}
                </div>
              </div>
            )
            : null
        }
        confirmText={confirm?.action === 'kill' ? 'Kill' : confirm?.action === 'restart' ? 'Restart' : 'Stop'}
        danger={confirm?.action === 'kill'}
        onCancel={() => setConfirm(null)}
        onConfirm={async () => {
          if (!confirm) return;
          const { action, pid } = confirm;
          setConfirm(null);
          try {
            const url =
              action === 'kill'
                ? `${API_BASE}/servers/${serverId}/processes/${pid}`
                : action === 'stop'
                ? `${API_BASE}/servers/${serverId}/processes/${pid}/stop`
                : `${API_BASE}/servers/${serverId}/processes/${pid}/restart`;
            const method = action === 'kill' ? 'DELETE' : 'POST';
            const resp = await fetch(url, {
              method,
              headers: { Authorization: `Bearer ${tokenStore.getToken()}` },
            });
            if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
            toastStore.push('success', `${action} sent to PID ${pid}`, 'Processes');
            void fetchProcesses({ silent: true });
          } catch (e) {
            toastStore.push('error', e instanceof Error ? e.message : 'Action failed', 'Processes');
          }
        }}
      />
    </section>
  );
});

export default ProcessesPage;
