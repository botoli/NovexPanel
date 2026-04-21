import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

import { Icon } from '@iconify/react';

import { serverMetricsStore } from '../../../Store/ServerMetricsStore';

import { API_BASE } from '../../../Api/api';
import { useCurrentServer } from '../../../Store/ServerStore';
import { tokenStore } from '../../../Store/TokenStore';
import styles from './MetricsPage.module.scss';

type RangeOption = '10m' | '30m' | '1h' | '2h' | '1d' | '7d';

type MetricsHistoryPoint = {
  timestamp: string;
  cpu: number;
  disk: number;
  ram: number;
  network_tx: number;
  network_rx: number;
};

const RANGE_OPTIONS: Array<{ value: RangeOption; label: string; }> = [
  { value: '10m', label: '10 min' },
  { value: '30m', label: '30 min' },
  { value: '1h', label: '1 hour' },
  { value: '2h', label: '2 hours' },
  { value: '1d', label: '1 day' },
  { value: '7d', label: '7 days' },
];

const AXIS_TICK_STYLE = {
  fill: 'rgba(255,255,255,0.45)',
  fontSize: '11px',
  letterSpacing: '0.02em',
};

const CHART_TOOLTIP_STYLE = {
  background: 'rgba(6,6,6,0.98)',
  border: '1px solid rgba(255,255,255,0.08)',
  borderRadius: 10,
  color: '#fff',
  boxShadow: '0 8px 24px rgba(0,0,0,0.45)',
};

const getRangeInterval = (range: RangeOption): string => {
  switch (range) {
    case '10m':
      return '10s';
    case '30m':
      return '1m';
    case '1h':
      return '1m';
    case '2h':
      return '10m';
    case '1d':
      return '1h';
    case '7d':
      return '1d';
    default:
      return '1m';
  }
};

const clampPercent = (value: number) => Math.max(0, Math.min(100, value));
const formatPercent = (value: number) => `${Math.round(clampPercent(value))}%`;

const MetricsSkeleton = () => (
  <div className={styles.mainContent}>
    <div className={styles.statsGrid}>
      {Array.from({ length: 4 }).map((_, index) => (
        <div
          key={`metric-skeleton-${index}`}
          className={`${styles.statCard} ${styles.skeletonCard}`}
        >
          <div className={styles.skeletonLine} />
          <div className={`${styles.skeletonLine} ${styles.skeletonLineWide}`} />
          <div className={`${styles.skeletonLine} ${styles.skeletonLineShort}`} />
        </div>
      ))}
    </div>

    <div className={styles.chartsGrid}>
      {Array.from({ length: 4 }).map((_, index) => (
        <section
          key={`chart-skeleton-${index}`}
          className={`${styles.chartCard} ${styles.skeletonCard}`}
        >
          <div className={styles.skeletonHeaderRow}>
            <div className={`${styles.skeletonLine} ${styles.skeletonLineShort}`} />
            <div className={`${styles.skeletonLine} ${styles.skeletonLineMedium}`} />
          </div>
          <div className={styles.chartSkeleton} />
        </section>
      ))}
    </div>
  </div>
);

const MetricsPage = observer(() => {
  const { server } = useCurrentServer();
  const [metricsHistory, setMetricsHistory] = useState<MetricsHistoryPoint[]>([]);
  const [range, setRange] = useState<RangeOption>('10m');
  const [isRangeSwitching, setIsRangeSwitching] = useState(false);

  const interval = getRangeInterval(range);

  const cpuSeries = useMemo(() => {
    return metricsHistory.map((point) => ({
      t: new Date(point.timestamp).toLocaleString(),
      v: Math.floor(point.cpu),
    }));
  }, [metricsHistory]);

  const diskIoSeries = useMemo(() => {
    return metricsHistory.map((point) => ({
      t: new Date(point.timestamp).toLocaleString(),
      v: Math.floor(point.disk),
    }));
  }, [metricsHistory]);

  const ramSeries = useMemo(() => {
    return metricsHistory.map((point) => ({
      t: new Date(point.timestamp).toLocaleString(),
      v: Math.floor(point.ram),
    }));
  }, [metricsHistory]);

  const networkSeries = useMemo(() => {
    return metricsHistory.map((point) => ({
      t: new Date(point.timestamp).toLocaleString(),
      in: Math.floor(point.network_tx / 1024),
      out: Math.floor(point.network_rx / 1024),
    }));
  }, [metricsHistory]);

  const fetchData = useCallback(async () => {
    try {
      const response = await fetch(
        `${API_BASE}/servers/${server?.id}/metrics?range=${range}&interval=${interval}`,
        {
          headers: { Authorization: `Bearer ${tokenStore.getToken()}` },
        },
      );
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      setMetricsHistory(data as MetricsHistoryPoint[]);
    } catch (err) {
      if ((err as any)?.name === 'AbortError') return;
      console.error('Error fetching server metrics:', err);
    }
  }, [server?.id, range, interval]);
  useEffect(() => {
    if (!Number.isFinite(server?.id)) return;

    if (!tokenStore.getToken()) return;

    void fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, [server?.id, fetchData]);

  useEffect(() => {
    if (!isRangeSwitching) return;

    const timeoutId = window.setTimeout(() => {
      setIsRangeSwitching(false);
    }, 260);

    return () => window.clearTimeout(timeoutId);
  }, [isRangeSwitching]);

  const onRangeChange = (nextRange: RangeOption) => {
    if (nextRange === range) return;
    setIsRangeSwitching(true);
    setRange(nextRange);
  };

  const renderRangeButtons = () => (
    <div className={styles.rangeButtons}>
      {RANGE_OPTIONS.map((option) => (
        <button
          key={option.value}
          type='button'
          aria-pressed={range === option.value}
          onClick={() => onRangeChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );

  const chartBodyClassName = isRangeSwitching
    ? `${styles.chartBody} ${styles.chartBodySwitching}`
    : styles.chartBody;

  const isHistoryLoading = metricsHistory.length === 0;

  if (!Number.isFinite(server?.id)) {
    return <div className={styles.stateMessage}>Invalid server id</div>;
  }

  if (serverMetricsStore.getNowServers().length === 0) {
    return <MetricsSkeleton />;
  }

  if (!server) {
    return <div className={styles.stateMessage}>Server not found</div>;
  }

  const isOverheated = server.last_metrics.temperature > 70;

  return (
    <div className={styles.mainContent}>
      <div className={styles.statsGrid}>
        <div
          className={`${styles.statCard} ${styles.statCardCpu} ${
            isOverheated ? styles.overheated : ''
          }`}
        >
          <Icon icon='heroicons:cpu-chip-16-solid' className={styles.cardGhostIcon} />

          <div className={styles.statHeader}>
            <p className={styles.statLabel}>
              <Icon icon='heroicons:cpu-chip-16-solid' className={styles.statLabelIcon} /> CPU
            </p>
            <span className={styles.statPillTemperature}>
              {server.last_metrics.temperature.toFixed(1)}°C
            </span>
          </div>
          <div className={styles.statValueRow}>
            <p className={styles.statValue}>{Math.floor(server.last_metrics.cpu.usage)}%</p>
          </div>
          <div className={styles.progressTrack}>
            <div
              className={styles.progressFill}
              style={{ width: `${Math.floor(server.last_metrics.cpu.usage)}%` }}
            />CrossFit Basement
          </div>
          <div className={styles.loadAVG}>
            Load average
            {server.last_metrics.cpu.load_avg && (
              <span className={styles.statHint} key={server.id}>
                {server.last_metrics.cpu.load_avg[0]} (1 min) {server.last_metrics.cpu.load_avg[1]}
                {' '}
                (5 min) {server.last_metrics.cpu.load_avg[2]} (15 min)
              </span>
            )}

            {isOverheated && (
              <div className={styles.overheatMsg} role='status' aria-live='polite'>
                <Icon icon='mdi:alert-circle-outline' />
                Overheating
              </div>
            )}
          </div>
        </div>

        <div className={`${styles.statCard} ${styles.statCardRam}`}>
          <Icon icon='fa6-solid:memory' className={styles.cardGhostIcon} />

          <div className={styles.statHeader}>
            <p className={styles.statLabel}>
              <Icon icon='fa6-solid:memory' className={styles.statLabelIcon} /> RAM
            </p>
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
          <Icon icon='mdi:harddisk' className={styles.cardGhostIcon} />

          <div className={styles.statHeader}>
            <p className={styles.statLabel}>
              <Icon icon='mdi:harddisk' className={styles.statLabelIcon} /> Disk
            </p>
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
          <Icon icon='mdi:lan' className={styles.cardGhostIcon} />

          <div className={styles.statHeader}>
            <p className={styles.statLabel}>
              <Icon icon='mdi:lan' className={styles.statLabelIcon} /> Network
            </p>
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
            <h2 className={styles.chartTitle}>
              <Icon icon='heroicons:cpu-chip-16-solid' className={styles.chartTitleIcon} />{' '}
              CPU usage
            </h2>
            {renderRangeButtons()}
          </header>

          <div className={chartBodyClassName}>
            {isHistoryLoading
              ? <div className={styles.chartSkeleton} />
              : (
                <ResponsiveContainer key={`cpu-${range}`} width='100%' height={240} minWidth={0}>
                  <AreaChart
                    data={cpuSeries}
                    margin={{ top: 8, right: 18, left: 0, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id='cpuFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='0%' stopColor='var(--chart-fill)' stopOpacity={0.2} />
                        <stop offset='100%' stopColor='var(--chart-fill)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.05)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      domain={[0, 100]}
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)', strokeWidth: 1 }}
                      contentStyle={CHART_TOOLTIP_STYLE}
                      labelStyle={{ color: 'rgba(255,255,255,0.66)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='v'
                      stroke='var(--chart-line)'
                      fill='url(#cpuFill)'
                      strokeWidth={2.2}
                      dot={false}
                      activeDot={{ r: 4, strokeWidth: 0, fill: 'var(--chart-line)' }}
                      isAnimationActive
                      animationDuration={250}
                      animationEasing='ease-out'
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
          </div>
        </section>

        <section className={`${styles.chartCard} ${styles.chartCardDiskIo}`}>
          <header className={styles.chartHeader}>
            <h2 className={styles.chartTitle}>
              <Icon icon='mdi:harddisk' className={styles.chartTitleIcon} /> Disk load
            </h2>
            {renderRangeButtons()}
          </header>

          <div className={chartBodyClassName}>
            {isHistoryLoading
              ? <div className={styles.chartSkeleton} />
              : (
                <ResponsiveContainer key={`disk-${range}`} width='100%' height={240} minWidth={0}>
                  <AreaChart data={diskIoSeries} margin={{ top: 8, right: 18, left: 0, bottom: 0 }}>
                    <defs>
                      <linearGradient id='diskFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='0%' stopColor='var(--chart-fill)' stopOpacity={0.2} />
                        <stop offset='100%' stopColor='var(--chart-fill)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.05)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      domain={[0, 100]}
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)', strokeWidth: 1 }}
                      contentStyle={CHART_TOOLTIP_STYLE}
                      labelStyle={{ color: 'rgba(255,255,255,0.66)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='v'
                      stroke='var(--chart-line)'
                      fill='url(#diskFill)'
                      strokeWidth={2.2}
                      dot={false}
                      activeDot={{ r: 4, strokeWidth: 0, fill: 'var(--chart-line)' }}
                      isAnimationActive
                      animationDuration={250}
                      animationEasing='ease-in-out'
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
          </div>
        </section>

        <section className={`${styles.chartCard} ${styles.chartCardRam}`}>
          <header className={styles.chartHeader}>
            <h2 className={styles.chartTitle}>
              <Icon icon='mdi:memory' className={styles.chartTitleIcon} /> RAM usage
            </h2>

            {renderRangeButtons()}
          </header>

          <div className={chartBodyClassName}>
            {isHistoryLoading
              ? <div className={styles.chartSkeleton} />
              : (
                <ResponsiveContainer key={`ram-${range}`} width='100%' height={240} minWidth={0}>
                  <AreaChart data={ramSeries} margin={{ top: 8, right: 18, left: 0, bottom: 0 }}>
                    <defs>
                      <linearGradient id='ramFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='0%' stopColor='var(--chart-fill)' stopOpacity={0.2} />
                        <stop offset='100%' stopColor='var(--chart-fill)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.05)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      domain={[0, 100]}
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)', strokeWidth: 1 }}
                      contentStyle={CHART_TOOLTIP_STYLE}
                      labelStyle={{ color: 'rgba(255,255,255,0.66)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='v'
                      stroke='var(--chart-line)'
                      fill='url(#ramFill)'
                      strokeWidth={2.2}
                      isAnimationActive
                      animationDuration={250}
                      animationEasing='ease-in-out'
                      dot={false}
                      activeDot={{ r: 4, strokeWidth: 0, fill: 'var(--chart-line)' }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
          </div>
        </section>

        <section className={`${styles.chartCard} ${styles.chartCardNetwork}`}>
          <header className={styles.chartHeader}>
            <h2 className={styles.chartTitle}>
              <Icon icon='mdi:lan' className={styles.chartTitleIcon} /> Network I/O
            </h2>
            {renderRangeButtons()}
          </header>

          <div className={chartBodyClassName}>
            {isHistoryLoading
              ? <div className={styles.chartSkeleton} />
              : (
                <ResponsiveContainer
                  key={`network-${range}`}
                  width='100%'
                  height={240}
                  minWidth={0}
                >
                  <AreaChart
                    data={networkSeries}
                    margin={{ top: 8, right: 18, left: 0, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id='netInFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='0%' stopColor='var(--chart-fill-1)' stopOpacity={0.2} />
                        <stop offset='100%' stopColor='var(--chart-fill-1)' stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id='netOutFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='0%' stopColor='var(--chart-fill-2)' stopOpacity={0.2} />
                        <stop offset='100%' stopColor='var(--chart-fill-2)' stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} stroke='rgba(255,255,255,0.05)' />
                    <XAxis
                      dataKey='t'
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      interval='preserveStartEnd'
                    />
                    <YAxis
                      axisLine={false}
                      tickLine={false}
                      tick={AXIS_TICK_STYLE}
                      width={28}
                    />
                    <Tooltip
                      cursor={{ stroke: 'rgba(255,255,255,0.12)', strokeWidth: 1 }}
                      contentStyle={CHART_TOOLTIP_STYLE}
                      labelStyle={{ color: 'rgba(255,255,255,0.66)' }}
                    />
                    <Area
                      type='monotone'
                      dataKey='in'
                      name='Incoming'
                      stroke='var(--chart-line-1)'
                      fill='url(#netInFill)'
                      strokeWidth={2.1}
                      dot={false}
                      activeDot={{ r: 4, strokeWidth: 0, fill: 'var(--chart-line-1)' }}
                      connectNulls
                      isAnimationActive
                      animationDuration={250}
                      animationEasing='ease-in-out'
                    />
                    <Area
                      type='monotone'
                      dataKey='out'
                      name='Outgoing'
                      stroke='var(--chart-line-2)'
                      fill='url(#netOutFill)'
                      strokeWidth={2.1}
                      dot={false}
                      activeDot={{ r: 4, strokeWidth: 0, fill: 'var(--chart-line-2)' }}
                      connectNulls
                      isAnimationActive
                      animationDuration={250}
                      animationEasing='ease-in-out'
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
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
  );
});
export default MetricsPage;
