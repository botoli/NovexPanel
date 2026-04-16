import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
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
const formatTemperature = (value: number) => `${Math.round(value)}C`;

const HomePage = observer(() => {
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [isAddServerModalOpen, setIsAddServerModalOpen] = useState<boolean>(false);
  const [serverName, setServerName] = useState<string>('');
  const hasServers = serverMetricsStore.getServerMetrics().length > 0;
  const navigate = useNavigate();

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
                      {serverMetricsStore.getNowServers().filter(server => server.online).length}

                      Active servers
                    </p>
                  </div>
                </div>

                <section className={styles.serverGrid}>
                  {serverMetricsStore.getNowServers().map(server => (
                    <Link
                      to={`/servers/${server.id}`}
                      key={server.id}
                      className={styles.serverLink}
                    >
                      <article
                        // TODO: объединить serverCardOnline и serverCardOffline в один класс с модификатором
                        className={server.online
                          ? styles.serverCardOnline
                          : styles.serverCardOffline}
                        key={server.id}
                      >
                        <div className={styles.serverTop}>
                          <div className={styles.serverIdentity}>
                            <div className={server.online ? styles.online : styles.offline}></div>
                            <h2>{server.name ?? `Server #${server.id}`}</h2>
                          </div>
                          <span className={styles.serverStatus}>
                            {server.online ? 'Online' : 'Offline'}
                          </span>
                        </div>

                        <p className={styles.serverIp}>{server.ip}</p>

                        <div className={styles.serverStack}>
                          {[
                            {
                              key: 'cpu',
                              label: 'CPU',
                              value: formatPercent(server.last_metrics.cpu.usage || 0),
                            },
                            {
                              key: 'ram',
                              label: 'RAM',
                              value: formatPercent(server.last_metrics.ram.percent || 0),
                            },
                            {
                              key: 'disk',
                              label: 'Disk',
                              value: formatPercent(server.last_metrics.disk.percent || 0),
                            },
                            {
                              key: 'cpu-temperature',
                              label: 'CPU Temp',
                              value: formatTemperature(server.last_metrics.temperature || 0),
                            },
                          ].map(layer => (
                            <div className={styles.stackLayer} key={layer.key}>
                              <span className={styles.layerLabel}>{layer.label}</span>
                              <strong className={styles.layerValue}>{layer.value}</strong>
                            </div>
                          ))}
                        </div>
                      </article>
                    </Link>
                  ))}
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
