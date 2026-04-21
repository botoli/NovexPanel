import { observer } from 'mobx-react-lite';

import { Icon } from '@iconify/react';
import { Link, NavLink, Outlet } from 'react-router-dom';

import { useCurrentServer } from '../../Store/ServerStore';
import LeftPanel from '../LeftPanel/LeftPanel';
import styles from './ServerPage.module.scss';

const ServerPage = observer(() => {
  const { server } = useCurrentServer();
  const tabClassName = ({ isActive }: { isActive: boolean; }) =>
    isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab;

  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        {server?.online
          ? (
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
                <NavLink to={`/servers/${server?.id}/metrics`} end className={tabClassName}>
                  Metrics
                </NavLink>
                <NavLink to={`/servers/${server?.id}/terminal`} end className={tabClassName}>
                  Terminal
                </NavLink>
                <NavLink to={`/servers/${server?.id}/processes`} end className={tabClassName}>
                  Processes
                </NavLink>
                <NavLink to={`/servers/${server?.id}/deploy`} end className={tabClassName}>
                  Deploy
                </NavLink>
              </div>

              <Outlet />
            </div>
          )
          : (
            <div className={styles.contentWrap}>
              <div className={styles.offlineCard} role='status' aria-live='polite'>
                <div className={styles.offlineHeader}>
                  <span className={styles.offlineIcon} aria-hidden='true'>
                    <Icon icon='mdi:server' />
                  </span>
                  <div className={styles.offlineText}>
                    <h1 className={styles.offlineTitle}>Server Offline</h1>
                    <p className={styles.offlineSubtitle}>
                      Currently unable to connect to the server.
                    </p>
                  </div>
                </div>

                <div className={styles.offlineActions}>
                  <Link to='/' className={styles.actionBtn}>
                    <Icon icon='mdi:home' />
                    Back home
                  </Link>
                </div>
              </div>
            </div>
          )}
      </div>
    </div>
  );
});

export default ServerPage;
