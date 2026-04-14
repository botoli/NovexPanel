import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useEffect } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { dashboardMock } from './Data';
import styles from './Home.module.scss';
export const Menu = () => <Icon icon='mdi:menu' fontSize='30' />;
export const KeyboardArrowDown = () => <Icon icon='mdi:keyboard-arrow-down' fontSize='30' />;
export const ComputerOutlined = () => <Icon icon='ri:cpu-line' fontSize='25' />;
export const LayersOutlined = () => <Icon icon='lucide:memory-stick' fontSize='25' />;
export const StorageOutlined = () => <Icon icon='mdi:harddisk' fontSize='25' />;
export const DnsOutlined = () => <Icon icon='mdi:dns' fontSize='25' />;
export const DeployOutlined = () => <Icon icon='eos-icons:code-deploy' fontSize='25' />;
export const CheckCircleOutlined = () => <Icon icon='mdi:check-circle' fontSize='25' />;
export const ErrorOutlined = () => <Icon icon='mdi:error' fontSize='25' />;
const renderMetricIcon = (icon: string) => {
  switch (icon) {
    case 'cpu':
      return <ComputerOutlined />;
    case 'memory':
      return <LayersOutlined />;
    case 'disk':
      return <StorageOutlined />;
    default:
      return <ComputerOutlined />;
  }
};

const renderHighlightIcon = (icon: string) => {
  switch (icon) {
    case 'containers':
      return <DnsOutlined />;
    case 'deploy':
      return <DeployOutlined />;
    default:
      return <DnsOutlined />;
  }
};

const renderActivityIcon = (type: string) => {
  switch (type) {
    case 'success':
      return <CheckCircleOutlined />;
    case 'error':
      return <ErrorOutlined />;
    default:
      return <DeployOutlined />;
  }
};

const HomePage = observer(() => {
  useEffect(() => {
    async function fetchData() {
      try {
        const response = await fetch('http://localhost:8380/servers');
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        console.log('Fetched dashboard data:', data);
      } catch (error) {
        console.error('Error fetching dashboard data:', error);
      }
    }

    fetchData();
  }, []);
  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        <div className={styles.contentWrap}>
          <div className={styles.header}>
            <div className={styles.pageTitle}>
              <h1>{dashboardMock.title}</h1>
            </div>
            <button className={styles.ActionBtn}>
              <Menu />
              Actions
              <KeyboardArrowDown />
            </button>
          </div>

          <div className={styles.serverRow}>
            <div
              className={dashboardMock.server.status === 'online'
                ? styles.online
                : styles.offline}
            >
            </div>
            <p className={styles.serverName}>{dashboardMock.server.name}</p>
            <span className={styles.serverStatus}>
              {dashboardMock.server.status}
            </span>
          </div>

          <section className={styles.metricsGrid}>
            {dashboardMock.metrics.map((metric) => (
              <article key={metric.id} className={styles.card}>
                <div className={styles.cardHeader}>
                  <div className={styles.cardTitleWrap}>
                    {renderMetricIcon(metric.icon)}
                    <h3>{metric.label}</h3>
                  </div>
                </div>

                <div className={styles.metricValue}>
                  <span>{metric.value}%</span>
                  <p>
                    {metric.used} {metric.unit} / {metric.total} {metric.unit}
                  </p>
                </div>

                <div className={styles.progressTrack}>
                  <div
                    className={styles.progressFill}
                    style={{ width: `${metric.value}%` }}
                  >
                  </div>
                </div>
              </article>
            ))}
          </section>

          <section className={styles.highlightsGrid}>
            {dashboardMock.highlights.map((item) => (
              <article key={item.id} className={styles.smallCard}>
                <div className={styles.cardTitleWrap}>
                  {renderHighlightIcon(item.icon)}
                  <h3>{item.label}</h3>
                </div>
                <p className={styles.smallCardValue}>{item.value}</p>
                <span className={styles.smallCardNote}>{item.note}</span>
              </article>
            ))}
          </section>

          <section className={styles.activitySection}>
            <h2>Recent Activity</h2>
            <div className={styles.activityCard}>
              {dashboardMock.activity.map((event) => (
                <div key={event.id} className={styles.activityRow}>
                  <div className={styles.activityLeft}>
                    <div className={styles.activityIcon}>
                      {renderActivityIcon(event.type)}
                    </div>
                    <p className={styles.activityText}>
                      {event.message} <span>{event.target}</span>
                    </p>
                  </div>
                  <span className={styles.activityTime}>{event.ago}</span>
                </div>
              ))}

              <div className={styles.activityFooter}>
                <button className={styles.viewAllBtn}>View all</button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
});
export default HomePage;
