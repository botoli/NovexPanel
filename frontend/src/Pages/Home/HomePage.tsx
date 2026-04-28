import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { API_BASE } from '../../Api/api';
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

const formatBytes = (value: number) => `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`;

const formatUptime = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return '0m';
  }

  const totalMinutes = Math.floor(seconds / 60);
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;

  const parts: string[] = [];

  if (days > 0) {
    parts.push(`${days}d`);
  }

  if (hours > 0 || days > 0) {
    parts.push(`${hours}h`);
  }

  parts.push(`${minutes}m`);

  return parts.join(' ');
};

const HomePage = observer(() => {
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [isAddServerModalOpen, setIsAddServerModalOpen] = useState<boolean>(false);
  const [serverName, setServerName] = useState<string>('');
  const servers = serverMetricsStore.getNowServers();
  const hasServers = servers.length > 0;
  const navigate = useNavigate();

  const getLoadToneClassName = (value: number) => {
    if (value >= 85) return styles.toneCritical;
    if (value >= 65) return styles.toneHigh;
    if (value >= 40) return styles.toneMedium;
    return styles.toneLow;
  };

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

      const response = await fetch(`${API_BASE}/auth/tokens`, {
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
                    const ramUsed = formatBytes(server.last_metrics.ram.used);
                    const ramTotal = formatBytes(server.last_metrics.ram.total);
                    const diskUsed = formatBytes(server.last_metrics.disk.used);
                    const diskTotal = formatBytes(server.last_metrics.disk.total);
                    const cpuToneClass = getLoadToneClassName(cpuUsage);
                    const memoryToneClass = getLoadToneClassName(ramUsage);
                    const diskToneClass = getLoadToneClassName(diskUsage);
                    const hostname = server.name ?? `Server #${server.id}`;

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
                              <div className={server.online ? styles.online : styles.offline} />
                              <h2>{server.name ?? `Server #${server.id}`}</h2>
                            </div>
                            <p className={styles.serverIp}>{server.ip}</p>
                          </div>

                          <div className={styles.serverHeaderRight}>
                            <span className={styles.serverStatus}>
                              {server.online ? 'Online' : 'Offline'}
                            </span>
                            <div className={styles.serverActions}>
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

                        <div className={styles.metricGrid}>
                          <div className={`${styles.metricCard} ${styles.metricCardCpu}`}>
                            <div className={styles.metricCardHeader}>
                              <span className={styles.metricCardLabel}>
                                <Icon icon='heroicons:cpu-chip-16-solid' /> CPU
                              </span>
                            </div>
                            <div className={styles.metricCardBody}>
                              <strong
                                className={`${styles.metricCardValue} ${styles.monoValue} ${cpuToneClass}`}
                              >
                                {formatPercent(cpuUsage)}
                              </strong>
                              <span className={styles.metricCardMeta}>
                                {server.last_metrics.cpu.cores} cores
                              </span>
                            </div>
                          </div>

                          <div className={styles.metricCard}>
                            <div className={styles.metricCardHeader}>
                              <span className={styles.metricCardLabel}>
                                <Icon icon='fa6-solid:memory' /> RAM
                              </span>
                            </div>
                            <div className={styles.metricCardBody}>
                              <strong
                                className={`${styles.metricCardValue} ${styles.monoValue} ${memoryToneClass}`}
                              >
                                {formatPercent(ramUsage)}
                              </strong>
                              <span className={styles.metricCardMeta}>
                                {ramUsed} / {ramTotal}
                              </span>
                            </div>
                          </div>

                          <div className={styles.metricCard}>
                            <div className={styles.metricCardHeader}>
                              <span className={styles.metricCardLabel}>
                                <Icon icon='mdi:harddisk' /> Disk
                              </span>
                            </div>
                            <div className={styles.metricCardBody}>
                              <strong
                                className={`${styles.metricCardValue} ${styles.monoValue} ${diskToneClass}`}
                              >
                                {formatPercent(diskUsage)}
                              </strong>
                              <span className={styles.metricCardMeta}>
                                {diskUsed} / {diskTotal}
                              </span>
                            </div>
                          </div>
                        </div>

                        <div className={styles.serverDetailsGrid}>
                          <div className={styles.serverDetail}>
                            <span className={styles.serverDetailLabel}>
                              <Icon icon='line-md:computer' /> Hostname
                            </span>
                            <strong className={styles.serverDetailValue}>{hostname}</strong>
                          </div>

                          <div className={styles.serverDetail}>
                            <span className={styles.serverDetailLabel}>
                              <Icon icon='iconamoon:clock-light' fontSize='30' /> Uptime
                            </span>
                            <strong className={`${styles.serverDetailValue} ${styles.monoValue}`}>
                              {formatUptime(server.last_metrics.uptime)}
                            </strong>
                          </div>

                          <div className={styles.serverDetail}>
                            <span className={styles.serverDetailLabel}>
                              <Icon icon='streamline-ultimate:temperature-thermometer-medium' />
                              {' '}
                              Temperature
                            </span>
                            <strong className={`${styles.serverDetailValue} ${styles.monoValue}`}>
                              {server.last_metrics.temperature.toFixed(1)}°C
                            </strong>
                          </div>

                          <div className={styles.serverDetail}>
                            <span className={styles.serverDetailLabel}>
                              <Icon icon='streamline-flex:network-remix' /> Network
                            </span>
                            <strong className={`${styles.serverDetailValue} ${styles.monoValue}`}>
                              {(server.last_metrics.network.tx_speed / 1024).toFixed(1)} /{' '}
                              {(server.last_metrics.network.rx_speed / 1024).toFixed(1)} KB/s
                            </strong>
                          </div>
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
