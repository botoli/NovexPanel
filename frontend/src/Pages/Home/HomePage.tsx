import { Icon } from '@iconify/react';
import { getAlertTitleUtilityClass } from '@mui/material';
import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { agentTokenStore } from '../../Store/AgentTokenStore';
import { tokenStore } from '../../Store/TokenStore';
import LeftPanel from '../LeftPanel/LeftPanel';
import styles from './Home.module.scss';

type TopProcess = {
  cpu: number;
  mem: number;
  pid: number;
  name: string;
};

type ServerItem = {
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
  const [fetchData, setFetchData] = useState<ServerItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();
  useEffect(() => {
    let isMounted = true;
    let isRequestInFlight = false;

    async function loadServers() {
      if (isRequestInFlight) {
        return;
      }

      isRequestInFlight = true;
      try {
        const response = await fetch('http://localhost:8380/servers', {
          headers: {
            Authorization: `Bearer ${tokenStore.getToken()}`,
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = (await response.json()) as ServerItem[];
        if (isMounted) {
          setFetchData(data);
        }
      } catch (fetchError) {
        console.error('Error fetching dashboard data:', fetchError);
      } finally {
        isRequestInFlight = false;
      }
    }

    loadServers();
    const intervalId = window.setInterval(() => {
      loadServers();
    }, 2000);

    return () => {
      isMounted = false;
      window.clearInterval(intervalId);
    };
  }, []);

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
      setError(null);

      const response = await fetch('http://localhost:8380/auth/tokens', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
      });

      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      agentTokenStore.setAgentToken(data.agent_token);
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
      <div className={styles.mainContent}>
        <div className={styles.contentWrap}>
          {fetchData.length > 0
            ? (
              <>
                <div className={styles.header}>
                  <div className={styles.pageTitle}>
                    <h1>Servers</h1>
                    <p>{fetchData.length} active cards</p>
                  </div>
                </div>

                <section className={styles.serverGrid}>
                  {fetchData.map(server => (
                    <article className={styles.serverCard} key={server.id}>
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
                  ))}
                  <article className={styles.addserverCard} onClick={getAgentToken}>
                    <div className={styles.addServer}>
                      <Icon icon='icons8:plus' fontSize='120' color='' />
                      <h1>Add server</h1>
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

                  {error && <p className={styles.emptyError}>{error}</p>}
                </div>
              </div>
            )}
        </div>
      </div>
    </div>
  );
});

export default HomePage;
