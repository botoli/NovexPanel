import { observer } from 'mobx-react-lite';

import { Icon } from '@iconify/react';
import { Link, NavLink, Outlet, useParams } from 'react-router-dom';

import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import LeftPanel from '../LeftPanel/LeftPanel';
import styles from './ServerPage.module.scss';

const ServerPage = observer(() => {
  const { id } = useParams<{ id?: string; }>();
  const serverId = id ? Number(id) : Number.NaN;

  const allServers = serverMetricsStore.getNowServers();
  const server = allServers.find(s => s.id === serverId);

  const tabClassName = ({ isActive }: { isActive: boolean; }) =>
    isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab;

  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        <div className={styles.contentWrap}>
          <div className={styles.topBar}>
            <div className={styles.serverHeading}>
              <div className={styles.serverTitleRow}>
                <h1 className={styles.serverTitle}>{server?.name}</h1>
                <button type='button' className={styles.iconBtn} aria-label='Edit server name'>
                  <Icon icon='mdi:pencil' />
                </button>
              </div>
              <div className={styles.serverMeta}>
                <span className={styles.serverIp}>{server?.ip}</span>
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
            <NavLink to={`/servers/${serverId}/metrics`} end className={tabClassName}>
              Metrics
            </NavLink>
            <NavLink to={`/servers/${serverId}/terminal`} end className={tabClassName}>
              Terminal
            </NavLink>
            <NavLink to={`/servers/${serverId}/processes`} end className={tabClassName}>
              Processes
            </NavLink>
            <Link to={`/servers/${serverId}/deploy`} className={styles.tab}>
              Deploy
            </Link>
          </div>

          <Outlet />
        </div>
      </div>
    </div>
  );
});

export default ServerPage;
