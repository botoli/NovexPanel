import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import AuthBtns from '../../common/AuthBtns/AuthBtns';
import { agentTokenStore } from '../../Store/AgentTokenStore';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import { tokenStore } from '../../Store/TokenStore';
import LeftPanel from '../LeftPanel/LeftPanel';
import styles from './Home.module.scss';

type TopProcess = {
  cpu: number;
  mem: number;
  pid: number;
  name: string;
};

type CpuHistoryPoint = {
  t: number;
  v: number;
};

export type ServerItem = {
  id: number;
  ip: string;
  name: string | null;
  online: boolean;
  last_metrics: {
    cpu: {
      cores: number;
      usage: number;
      load_avg: number[];
    };
    ram: {
      free: number;
      used: number;
      total: number;
      percent: number;
    };
    disk: {
      free: number;
      used: number;
      total: number;
      percent: number;
    };
    uptime: number;
    network: {
      rx_bytes: number;
      rx_speed: number;
      tx_bytes: number;
      tx_speed: number;
    };
    temperature: number;
    top_processes: TopProcess[];
  };
};

const clampPercent = (value: number) => Math.max(0, Math.min(100, value));
const formatPercent = (value: number) => `${Math.round(clampPercent(value))}%`;

const SPARKLINE_WINDOW_MS = 60 * 60 * 1000;
const SPARKLINE_POINTS_LIMIT = 1800;
const SPARKLINE_WIDTH = 136;
const SPARKLINE_HEIGHT = 34;

const buildSparklinePath = (values: number[]) => {
  const baseline = SPARKLINE_HEIGHT - 3;

  if (values.length === 0) {
    return {
      linePath: `M 0 ${baseline} L ${SPARKLINE_WIDTH} ${baseline}`,
      areaPath:
        `M 0 ${SPARKLINE_HEIGHT} L 0 ${baseline} L ${SPARKLINE_WIDTH} ${baseline} L ${SPARKLINE_WIDTH} ${SPARKLINE_HEIGHT} Z`,
    };
  }

  const topPadding = 2;
  const bottomPadding = 2;
  const drawableHeight = SPARKLINE_HEIGHT - topPadding - bottomPadding;
  const step = values.length > 1 ? SPARKLINE_WIDTH / (values.length - 1) : SPARKLINE_WIDTH;

  const points = values.map((value, index) => {
    const x = values.length > 1 ? step * index : SPARKLINE_WIDTH / 2;
    const y = topPadding + (1 - clampPercent(value) / 100) * drawableHeight;

    return { x, y };
  });

  const linePath = points
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
    .join(' ');

  const firstPoint = points[0];
  const lastPoint = points[points.length - 1];

  return {
    linePath,
    areaPath: `${linePath} L ${lastPoint.x.toFixed(2)} ${SPARKLINE_HEIGHT} L ${
      firstPoint.x.toFixed(2)
    } ${SPARKLINE_HEIGHT} Z`,
  };
};

const HomePage = observer(() => {
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [isAddServerModalOpen, setIsAddServerModalOpen] = useState<boolean>(false);
  const [serverName, setServerName] = useState<string>('');
  const [cpuHistoryByServer, setCpuHistoryByServer] = useState<Record<number, CpuHistoryPoint[]>>(
    {},
  );
  const servers = serverMetricsStore.getNowServers();
  const hasServers = servers.length > 0;
  const navigate = useNavigate();

  const getLoadToneClassName = (value: number) => {
    if (value >= 85) return styles.toneCritical;
    if (value >= 65) return styles.toneHigh;
    if (value >= 40) return styles.toneMedium;
    return styles.toneLow;
  };

  useEffect(() => {
    if (servers.length === 0) {
      setCpuHistoryByServer({});
      return;
    }

    const now = Date.now();

    setCpuHistoryByServer((prev) => {
      const next: Record<number, CpuHistoryPoint[]> = {};

      for (const server of servers) {
        const existing = prev[server.id] ?? [];
        const trimmed = existing.filter(point => now - point.t <= SPARKLINE_WINDOW_MS);
        const usage = clampPercent(server.last_metrics.cpu.usage || 0);

        next[server.id] = [...trimmed, { t: now, v: usage }].slice(-SPARKLINE_POINTS_LIMIT);
      }

      return next;
    });
  }, [servers]);

  const getAgentToken = async () => {
    if (!tokenStore.getToken()) {
      setError('Authentication token is missing. Please log in again.');
      setTimeout(() => {
        navigate('/login');
      }, 800);
      return;
    }
    try {
      setLoading(true);

      const response = await fetch('http://localhost:8380/auth/tokens', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
        body: JSON.stringify({ name: serverName }),
      });

      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      agentTokenStore.setAgentToken(data.agent_token);
      console.log(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
      throw err;
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.Page}>
      <LeftPanel />

      <div className={styles.agentTokenModal}>
        {agentTokenStore.getAgentToken() && (
          <div className={styles.tokenCard}>
            <h2>Agent Token</h2>
            <p className={styles.tokenValue}>{agentTokenStore.getAgentToken()}</p>
            <button
              type='button'
              className={styles.closeBtn}
              onClick={() => agentTokenStore.clearAgentToken()}
            >
              <Icon icon='mdi:close' className={styles.closeIcon} />
            </button>
          </div>
        )}
      </div>

      {isAddServerModalOpen && (
        <div
          className={styles.modalOverlay}
          onMouseDown={(e) => {
            if (e.target === e.currentTarget) {
              setIsAddServerModalOpen(false);
            }
          }}
        >
          <div className={styles.modalCard}>
            <div className={styles.modalHeader}>
              <div>
                <h2 className={styles.modalTitle}>Add server</h2>
                <p className={styles.modalSubtitle}>Server name is optional.</p>
              </div>
              <button
                type='button'
                className={styles.modalCloseBtn}
                onClick={() => setIsAddServerModalOpen(false)}
              >
                <Icon icon='mdi:close' className={styles.closeIcon} />
              </button>
            </div>

            <label className={styles.modalLabel}>
              Server name (optional)
              <input
                className={styles.modalInput}
                value={serverName}
                onChange={(e) => setServerName(e.target.value)}
                placeholder='My VPS'
              />
            </label>

            <div className={styles.modalActions}>
              <button
                type='button'
                className={styles.modalPrimaryBtn}
                onClick={async () => {
                  try {
                    await getAgentToken();
                    setIsAddServerModalOpen(false);
                  } catch {
                    // error is already shown on the page
                  }
                }}
                disabled={loading}
              >
                {loading ? 'Loading...' : 'Get agent token'}
              </button>
            </div>
          </div>
        </div>
      )}

      <div className={styles.mainContent}>
        <div className={styles.contentWrap}>
          {!tokenStore.getToken()
            ? (
              <div className={styles.emptyError}>
                <h1>You are not logged in</h1>
                <AuthBtns />
              </div>
            )
            : error
            ? (
              <div className={styles.emptyError}>
                <div className={styles.errorIconWrap}>
                  <Icon icon='mdi:alert-outline' />
                </div>
                <h2>Error</h2>
                <p>{error}</p>
                <div className={styles.errorActions}>
                  <button
                    type='button'
                    className={styles.retryBtn}
                    // onClick={loadServers}
                    disabled={loading}
                  >
                    Retry
                  </button>
                  <button
                    type='button'
                    className={styles.homeBtn}
                    onClick={() => navigate('/')}
                  >
                    Return Home
                  </button>
                </div>
              </div>
            )
            : hasServers
            ? (
              <>
                <div className={styles.header}>
                  <div className={styles.pageTitle}>
                    <h1>Servers</h1>
                    <p>
                      {servers.filter(server => server.online).length}

                      Active servers
                    </p>
                  </div>
                </div>

                <section className={styles.serverGrid}>
                  {servers.map(server => {
                    const cpuUsage = clampPercent(server.last_metrics.cpu.usage || 0);
                    const ramUsage = clampPercent(server.last_metrics.ram.percent || 0);
                    const diskUsage = clampPercent(server.last_metrics.disk.percent || 0);
                    const cpuToneClass = getLoadToneClassName(cpuUsage);
                    const cpuHistory = cpuHistoryByServer[server.id]
                      ?? [{ t: Date.now(), v: cpuUsage }];
                    const sparkline = buildSparklinePath(cpuHistory.map(point => point.v));

                    return (
                      <article
                        className={`${styles.serverCard} ${
                          server.online ? styles.serverCardOnline : styles.serverCardOffline
                        }`}
                        key={server.id}
                        role='button'
                        tabIndex={0}
                        onClick={() => navigate(`/servers/${server.id}/metrics`)}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault();
                            navigate(`/servers/${server.id}/metrics`);
                          }
                        }}
                      >
                        <header className={styles.serverHeader}>
                          <div className={styles.serverIdentity}>
                            <div className={styles.serverNameLine}>
                              <div className={server.online ? styles.online : styles.offline}></div>
                              <h2>{server.name ?? `Server #${server.id}`}</h2>
                            </div>
                            <p className={styles.serverIp}>{server.ip}</p>
                          </div>

                          <div className={styles.serverHeaderRight}>
                            <span className={styles.serverStatus}>
                              {server.online ? 'Online' : 'Offline'}
                            </span>
                            <div
                              className={styles.serverActions}
                              onClick={(event) => {
                                event.preventDefault();
                                event.stopPropagation();
                              }}
                            >
                              <button
                                type='button'
                                className={styles.actionBtn}
                                title='Open terminal'
                                aria-label='Open terminal'
                                onClick={(event) => {
                                  event.preventDefault();
                                  event.stopPropagation();
                                  navigate(`/servers/${server.id}/terminal`);
                                }}
                              >
                                <Icon icon='mdi:terminal' />
                              </button>
                              <button
                                type='button'
                                className={styles.actionBtn}
                                title='Restart agent'
                                aria-label='Restart agent'
                                onClick={(event) => {
                                  event.preventDefault();
                                  event.stopPropagation();
                                  navigate(`/servers/${server.id}/terminal`, {
                                    state: {
                                      suggestedCommand: 'sudo systemctl restart novex-agent',
                                    },
                                  });
                                }}
                              >
                                <Icon icon='mdi:restart' />
                              </button>
                            </div>
                          </div>
                        </header>

                        <div className={styles.cpuSection}>
                          <div className={styles.primaryMetricHead}>
                            <span className={styles.primaryMetricLabel}>CPU</span>
                            <strong
                              className={`${styles.primaryMetricValue} ${styles.monoValue} ${cpuToneClass}`}
                            >
                              {formatPercent(cpuUsage)}
                            </strong>
                          </div>

                          <div className={styles.progressTrackThin}>
                            <div
                              className={`${styles.progressFillThin} ${cpuToneClass}`}
                              style={{ width: `${cpuUsage}%` }}
                            />
                          </div>

                          <div className={styles.sparklineSection}>
                            <svg
                              className={styles.sparkline}
                              viewBox={`0 0 ${SPARKLINE_WIDTH} ${SPARKLINE_HEIGHT}`}
                              preserveAspectRatio='none'
                              aria-hidden='true'
                            >
                              <path
                                d={sparkline.areaPath}
                                className={`${styles.sparklineFill} ${cpuToneClass}`}
                              />
                              <path
                                d={sparkline.linePath}
                                className={`${styles.sparklineLine} ${cpuToneClass}`}
                              />
                            </svg>
                            <span className={styles.sparklineLabel}>CPU last hour</span>
                          </div>
                        </div>

                        <div className={styles.secondaryMetrics}>
                          {[
                            { key: 'ram', label: 'RAM', value: ramUsage },
                            { key: 'disk', label: 'Disk', value: diskUsage },
                          ].map(metric => {
                            const toneClass = getLoadToneClassName(metric.value);

                            return (
                              <div className={styles.secondaryMetric} key={metric.key}>
                                <div className={styles.secondaryMetricHead}>
                                  <span className={styles.secondaryMetricLabel}>
                                    {metric.label}
                                  </span>
                                  <strong
                                    className={`${styles.secondaryMetricValue} ${styles.monoValue} ${toneClass}`}
                                  >
                                    {formatPercent(metric.value)}
                                  </strong>
                                </div>
                                <div className={styles.progressTrackThin}>
                                  <div
                                    className={`${styles.progressFillThin} ${toneClass}`}
                                    style={{ width: `${metric.value}%` }}
                                  />
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      </article>
                    );
                  })}
                  <article
                    className={styles.addserverCard}
                    onClick={!loading ? () => setIsAddServerModalOpen(true) : undefined}
                  >
                    <div className={styles.addServer}>
                      <Icon icon='icons8:plus' fontSize='120' color='' />
                      <h1>{loading ? 'Loading...' : 'Add server'}</h1>
                    </div>
                  </article>
                </section>
              </>
            )
            : (
              <div className={styles.noServers}>
                <div className={styles.emptyStateCard}>
                  <div className={styles.emptyIconWrap}>
                    <Icon icon='qlementine-icons:server-16' fontSize='132' />
                  </div>
                  <h1>No servers yet</h1>
                  <p className={styles.emptyDescription}>
                    Connect your first agent to start receiving live metrics.
                  </p>
                  <button
                    onClick={() => setIsAddServerModalOpen(true)}
                    className={styles.addServerBtn}
                    disabled={loading}
                  >
                    {loading ? 'Loading...' : 'Add Server'}
                  </button>
                </div>
              </div>
            )}
        </div>
      </div>
    </div>
  );
});

export default HomePage;
