import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import { tokenStore } from '../../Store/TokenStore';
import LeftPanel from '../LeftPanel/LeftPanel';
import styles from './ServerPage.module.scss';

const networkSeries = [
  { t: '02:35', in: 140, out: 22 },
  { t: '02:40', in: 150, out: 24 },
  { t: '02:45', in: 160, out: 28 },
  { t: '02:50', in: 155, out: 25 },
  { t: '02:55', in: 170, out: 30 },
  { t: '03:00', in: 210, out: 35 },
  { t: '03:05', in: 175, out: 27 },
  { t: '03:10', in: 180, out: 26 },
  { t: '03:15', in: 165, out: 24 },
  { t: '03:20', in: 190, out: 29 },
  { t: '03:25', in: 520, out: 55 },
  { t: '03:30', in: 200, out: 32 },
  { t: '03:35', in: 175, out: 26 },
  { t: '03:40', in: 185, out: 28 },
  { t: '03:45', in: 195, out: 31 },
  { t: '03:50', in: 180, out: 27 },
  { t: '03:55', in: 210, out: 34 },
  { t: '04:00', in: 195, out: 30 },
  { t: '04:05', in: 220, out: 36 },
  { t: '04:10', in: 200, out: 32 },
  { t: '04:15', in: 240, out: 38 },
  { t: '04:20', in: 420, out: 46 },
  { t: '05:35', in: 260, out: 40 },
];

const ServerPage = observer(() => {
  const clampPercent = (value: number) => Math.max(0, Math.min(100, value));
  const formatPercent = (value: number) => `${Math.round(clampPercent(value))}%`;

  const [metricsHistory, setMetricsHistory] = useState<any[]>([]);
  const [resolution, setResolution] = useState('1m'); // '1h', '6h', '24h', '7d'

  const { id } = useParams<{ id?: string; }>();
  const serverId = id ? Number(id) : Number.NaN;

  const allServers = serverMetricsStore.getNowServers();
  const server = allServers.find(s => s.id === serverId);

  const cpuSeries = useMemo(() => {
    return metricsHistory.map((point: any) => ({
      t: new Date(point.timestamp).toLocaleString(),
      v: point.cpu < 1 ? Math.floor(point.cpu) : Math.floor(point.cpu),
    }));
  }, [metricsHistory]);
  const diskIoSeries = useMemo(() => {
    return metricsHistory.map((point: any) => ({
      t: new Date(point.timestamp).toLocaleString(),
      v: Math.floor(point.disk),
    }));
  }, [metricsHistory]);
  const ramSeries = useMemo(() => {
    return metricsHistory.map((point: any) => ({
      t: new Date(point.timestamp).toLocaleString(),
      v: Math.floor(point.ram),
    }));
  }, [metricsHistory]);

  useEffect(() => {
    if (!Number.isFinite(serverId)) return;

    const token = tokenStore.getToken();
    if (!token) return;

    const controller = new AbortController();

    const fetchData = async () => {
      try {
        const response = await fetch(
          `http://localhost:8380/servers/${serverId}/metrics?days=7&interval=${resolution}`,
          {
            signal: controller.signal,
            headers: { Authorization: `Bearer ${token}` },
          },
        );
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const data = await response.json();
        setMetricsHistory(data);
        console.log(data);
      } catch (err) {
        if ((err as any)?.name === 'AbortError') return;
        console.error('Error fetching server metrics:', err);
      }
    };

    void fetchData();

    return () => controller.abort();
  }, [serverId, resolution]);

  if (!Number.isFinite(serverId)) {
    return <div className={styles.loading}>Invalid server id</div>;
  }

  if (allServers.length === 0) {
    return <div className={styles.loading}>Loading servers...</div>;
  }

  if (!server) {
    return <div className={styles.loading}>Server not found</div>;
  }

  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        <div className={styles.contentWrap}>
          <div className={styles.topBar}>
            <div className={styles.serverHeading}>
              <div className={styles.serverTitleRow}>
                <h1 className={styles.serverTitle}>{server.name}</h1>
                <button type='button' className={styles.iconBtn} aria-label='Edit server name'>
                  <Icon icon='mdi:pencil' />
                </button>
              </div>
              <div className={styles.serverMeta}>
                <span className={styles.serverIp}>{server.ip}</span>
                <span className={styles.statusDot} aria-label='Online' />
              </div>
            </div>

            <div className={styles.actions}>
              <button type='button' className={styles.actionBtn}>
                <Icon icon='mdi:refresh' />
                Refresh
              </button>
              <Link to='/' className={styles.actionBtn}>
                <Icon icon='mdi:arrow-left' />
                Back to Servers
              </Link>
            </div>
          </div>

          <div className={styles.tabs}>
            <button type='button' className={`${styles.tab} ${styles.activeTab}`}>
              Metrics
            </button>
            <button type='button' className={styles.tab}>
              Terminal
            </button>
            <button type='button' className={styles.tab}>
              Processes
            </button>
            <button type='button' className={styles.tab}>
              Deploy
            </button>
          </div>

          <div className={styles.statsGrid}>
            <div className={`${styles.statCard} ${styles.statCardCpu}`}>
              <div className={styles.statHeader}>
                <p className={styles.statLabel}>CPU</p>
                <span className={styles.statPill}>
                  {Math.floor(server.last_metrics.cpu.usage)}%
                </span>
              </div>
              <div className={styles.statValueRow}>
                <p className={styles.statValue}>{Math.floor(server.last_metrics.cpu.usage)}%</p>
                <div
                  className={styles.progressFill}
                  style={{ width: `${Math.floor(server.last_metrics.cpu.usage)}%` }}
                />
              </div>
              <div className={styles.loadAVG}>
                Load average
                {server.last_metrics.cpu.load_avg && (
                  <span className={styles.statHint} key={server.id}>
                    {server.last_metrics.cpu.load_avg[0]} (1 min){' '}
                    {server.last_metrics.cpu.load_avg[1]} (5 min){' '}
                    {server.last_metrics.cpu.load_avg[2]} (15 min)
                  </span>
                )}
              </div>
            </div>

            <div className={`${styles.statCard} ${styles.statCardRam}`}>
              <div className={styles.statHeader}>
                <p className={styles.statLabel}>RAM</p>
                <span className={styles.statPill}>
                  {formatPercent(server.last_metrics.ram.percent)}
                </span>
              </div>
              <div className={styles.statValueRow}>
                <p className={styles.statValue}>
                  {(server.last_metrics.ram.used / 1024 / 1024 / 1024).toFixed(1)}
                  <span className={styles.statUnit}>GB</span>
                </p>
                <p className={styles.statSubValue}>
                  / {(server.last_metrics.ram.total / 1024 / 1024 / 1024).toFixed(1)} GB
                </p>
              </div>
              <p className={styles.statHint}>RAM</p>
              <div className={styles.progressTrack}>
                <div
                  className={styles.progressFill}
                  style={{ width: `${formatPercent(server.last_metrics.ram.percent)}` }}
                />
              </div>
            </div>

            <div className={`${styles.statCard} ${styles.statCardDisk}`}>
              <div className={styles.statHeader}>
                <p className={styles.statLabel}>Disk</p>
                <span className={styles.statPill}>
                  {formatPercent(server.last_metrics.disk.percent)}
                </span>
              </div>
              <div className={styles.statValueRow}>
                <p className={styles.statValue}>
                  {(server.last_metrics.disk.used / 1024 / 1024 / 1024).toFixed(1)}
                  <span className={styles.statUnit}>GB</span>
                </p>
                <p className={styles.statSubValue}>
                  / {(server.last_metrics.disk.total / 1024 / 1024 / 1024).toFixed(1)} GB
                </p>
              </div>
              <p className={styles.statHint}>RX</p>
              <div className={styles.progressTrack}>
                <div
                  className={styles.progressFill}
                  style={{ width: `${formatPercent(server.last_metrics.disk.percent)}` }}
                />
              </div>
            </div>

            <div className={`${styles.statCard} ${styles.statCardNetwork}`}>
              <div className={styles.statHeader}>
                <p className={styles.statLabel}>Network</p>
                <span className={styles.statPill}>15 min</span>
              </div>
              <div className={styles.statValueRow}>
                <p className={styles.statValue}>
                  {(server.last_metrics.network.tx_speed / 1024).toFixed(1)}
                  <span className={styles.statUnit}>KB/s</span>
                </p>
                <p className={styles.statSubValue}>
                  / {(server.last_metrics.network.rx_speed / 1024).toFixed(1)} KB/s
                </p>
              </div>
              <p className={styles.statHint}>TX</p>
            </div>
          </div>

          <div className={styles.chartsGrid}>
            <section className={`${styles.chartCard} ${styles.chartCardCpu}`}>
              <header className={styles.chartHeader}>
                <h2 className={styles.chartTitle}>CPU</h2>
                <select value={resolution} onChange={e => setResolution(e.target.value)}>
                  <option value='1m'>1 minute</option>
                  <option value='5m'>5 minutes</option>
                  <option value='1h'>1 hour</option>
                </select>
              </header>

              <div className={styles.chartBody}>
                <ResponsiveContainer width='100%' height={240} minWidth={0}>
                  <AreaChart
                    data={cpuSeries}
                    margin={{ top: 8, right: 18, left: 0, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id='cpuFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor='var(--chart-accent)' stopOpacity={0.35} />
                        <stop offset='95%' stopColor='var(--chart-accent)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.06)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      domain={[0, 100]}
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)' }}
                      contentStyle={{
                        background: 'rgba(10,10,10,0.95)',
                        border: '1px solid rgba(255,255,255,0.08)',
                        borderRadius: 10,
                        color: '#fff',
                      }}
                      labelStyle={{ color: 'rgba(255,255,255,0.7)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='v'
                      stroke='var(--chart-accent)'
                      fill='url(#cpuFill)'
                      strokeWidth={0.3}
                      dot={false}
                      activeDot={{ r: 3 }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </section>

            <section className={`${styles.chartCard} ${styles.chartCardRam}`}>
              <header className={styles.chartHeader}>
                <h2 className={styles.chartTitle}>Disk</h2>
                <span className={styles.chartRange}></span>
              </header>

              <div className={styles.chartBody}>
                <ResponsiveContainer width='100%' height={240} minWidth={0}>
                  <AreaChart data={diskIoSeries} margin={{ top: 8, right: 18, left: 0, bottom: 0 }}>
                    <defs>
                      <linearGradient id='ramFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor='var(--chart-accent)' stopOpacity={0.35} />
                        <stop offset='95%' stopColor='var(--chart-accent)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.06)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      domain={[0, 100]}
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)' }}
                      contentStyle={{
                        background: 'rgba(10,10,10,0.95)',
                        border: '1px solid rgba(255,255,255,0.08)',
                        borderRadius: 10,
                        color: '#fff',
                      }}
                      labelStyle={{ color: 'rgba(255,255,255,0.7)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='v'
                      stroke='var(--chart-accent)'
                      fill='url(#ramFill)'
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 3 }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </section>

            <section className={`${styles.chartCard} ${styles.chartCardRam}`}>
              <header className={styles.chartHeader}>
                <h2 className={styles.chartTitle}>RAM</h2>
                <span className={styles.chartRange}>
                  <select value={resolution} onChange={e => setResolution(e.target.value)}>
                    <option value='1m'>1 minute</option>
                    <option value='5m'>5 minutes</option>
                    <option value='1h'>1 hour</option>
                  </select>
                </span>
              </header>

              <div className={styles.chartBody}>
                <ResponsiveContainer width='100%' height={240} minWidth={0}>
                  <AreaChart data={ramSeries} margin={{ top: 8, right: 18, left: 0, bottom: 0 }}>
                    <defs>
                      <linearGradient id='ramFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor='var(--chart-accent)' stopOpacity={0.35} />
                        <stop offset='95%' stopColor='var(--chart-accent)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.06)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      domain={[0, 100]}
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)' }}
                      contentStyle={{
                        background: 'rgba(10,10,10,0.95)',
                        border: '1px solid rgba(255,255,255,0.08)',
                        borderRadius: 10,
                        color: '#fff',
                      }}
                      labelStyle={{ color: 'rgba(255,255,255,0.7)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='v'
                      stroke='var(--chart-accent)'
                      fill='url(#ramFill)'
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 3 }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </section>

            <section className={`${styles.chartCard} ${styles.chartCardNetwork}`}>
              <header className={styles.chartHeader}>
                <h2 className={styles.chartTitle}>Network</h2>
                <span className={styles.chartRange}>15 min</span>
              </header>

              <div className={styles.chartBody}>
                <ResponsiveContainer width='100%' height={240} minWidth={0}>
                  <AreaChart
                    data={networkSeries}
                    margin={{ top: 8, right: 18, left: 0, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id='netInFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor='var(--chart-accent-1)' stopOpacity={0.35} />
                        <stop offset='95%' stopColor='var(--chart-accent-1)' stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id='netOutFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor='var(--chart-accent-2)' stopOpacity={0.35} />
                        <stop offset='95%' stopColor='var(--chart-accent-2)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.06)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      axisLine={false}
                      tickLine={false}
                      tick={{
                        fill: 'rgba(255,255,255,0.5)',
                        fontSize: '11px',
                        letterSpacing: '0.02em',
                      }}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)' }}
                      contentStyle={{
                        background: 'rgba(10,10,10,0.95)',
                        border: '1px solid rgba(255,255,255,0.08)',
                        borderRadius: 10,
                        color: '#fff',
                      }}
                      labelStyle={{ color: 'rgba(255,255,255,0.7)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='in'
                      name='Incoming'
                      stroke='var(--chart-accent-1)'
                      fill='url(#netInFill)'
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 3 }}
                    />
                    <Area
                      type='monotone'
                      dataKey='out'
                      name='Outgoing'
                      stroke='var(--chart-accent-2)'
                      fill='url(#netOutFill)'
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 3 }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>

              <footer className={styles.chartLegend}>
                <span className={styles.legendItem}>
                  <span className={`${styles.legendDot} ${styles.legendDotIncoming}`} /> Incoming
                </span>
                <span className={styles.legendItem}>
                  <span className={`${styles.legendDot} ${styles.legendDotOutgoing}`} /> Outgoing
                </span>
              </footer>
            </section>
          </div>
        </div>
      </div>
    </div>
  );
});

export default ServerPage;
