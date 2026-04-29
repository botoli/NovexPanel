import { observer } from 'mobx-react-lite';

import { Icon } from '@iconify/react';
import { useMemo, useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';

import { useCurrentServer } from '../../Store/ServerStore';
import { refreshStore } from '../../Store/RefreshStore';
import { serverMetricsStore } from '../../Store/ServerMetricsStore';
import { toastStore } from '../../Store/ToastStore';
import { API_BASE } from '../../Api/api';
import { tokenStore } from '../../Store/TokenStore';
import LeftPanel from '../LeftPanel/LeftPanel';
import styles from './ServerPage.module.scss';

const ServerPage = observer(() => {
  const { server } = useCurrentServer();
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState('');
  const [renameSaving, setRenameSaving] = useState(false);

  const serverId = server?.id ?? null;
  const currentName = useMemo(() => (server?.name ?? `Server #${server?.id ?? ''}`), [server?.id, server?.name]);
  const tabClassName = ({ isActive }: { isActive: boolean; }) =>
    isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab;

  const openRename = () => {
    if (!serverId) return;
    setRenameValue(server?.name ?? '');
    setRenameOpen(true);
  };

  const submitRename = async () => {
    if (!serverId) return;
    const name = renameValue.trim();
    if (!name) {
      toastStore.push('error', 'Name cannot be empty.', 'Rename');
      return;
    }
    if (name.length > 120) {
      toastStore.push('error', 'Name is too long (max 120 characters).', 'Rename');
      return;
    }

    const prevName = server?.name ?? null;
    serverMetricsStore.updateServerName(serverId, name);
    setRenameSaving(true);
    try {
      const response = await fetch(`${API_BASE}/servers/${serverId}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
        body: JSON.stringify({ name }),
      });
      if (!response.ok) {
        let msg = `HTTP ${response.status}`;
        try {
          const body = await response.json();
          if (body?.error) msg = String(body.error);
        } catch {
          // ignore
        }
        throw new Error(msg);
      }
      setRenameOpen(false);
      toastStore.push('success', 'Server name updated.', 'Rename');
      void serverMetricsStore.refreshNow({ silent: true });
    } catch (err) {
      serverMetricsStore.updateServerName(serverId, prevName);
      toastStore.push('error', err instanceof Error ? err.message : 'Rename failed', 'Rename');
    } finally {
      setRenameSaving(false);
    }
  };

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
                    <button
                      type='button'
                      className={styles.iconBtn}
                      aria-label='Edit server name'
                      onClick={openRename}
                    >
                      <Icon icon='mdi:pencil' />
                    </button>
                  </div>
                  <div className={styles.serverMeta}>
                    <span className={styles.serverIp}>{server?.ip}</span>
                    <span className={styles.statusDot} aria-label='Online' />
                  </div>
                </div>

                <div className={styles.actions}>
                  <button
                    type='button'
                    className={styles.actionBtn}
                    disabled={refreshStore.refreshing}
                    onClick={() => {
                      void refreshStore.run(async () => {
                        await serverMetricsStore.refreshNow();
                      }, 'all').catch((err) => {
                        toastStore.push('error', err instanceof Error ? err.message : 'Refresh failed', 'Refresh');
                      });
                    }}
                  >
                    <Icon icon='mdi:refresh' />
                    {refreshStore.refreshing ? 'Refreshing...' : 'Refresh'}
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
                <NavLink to={`/servers/${server?.id}/deployments`} end className={tabClassName}>
                  Deployments
                </NavLink>
                <NavLink to={`/servers/${server?.id}/jobs`} end className={tabClassName}>
                  Jobs
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

      {renameOpen && serverId
        ? (
          <div
            className={styles.modalOverlay}
            role='dialog'
            aria-modal='true'
            aria-label='Rename server'
            onMouseDown={(e) => {
              if (e.target === e.currentTarget && !renameSaving) setRenameOpen(false);
            }}
          >
            <div className={styles.modalCard}>
              <div className={styles.modalTitleRow}>
                <div>
                  <h2 className={styles.modalTitle}>Rename server</h2>
                  <p className={styles.modalSubtitle}>New name will be saved on the backend.</p>
                </div>
                <button
                  type='button'
                  className={styles.modalCloseBtn}
                  aria-label='Close'
                  disabled={renameSaving}
                  onClick={() => setRenameOpen(false)}
                >
                  <Icon icon='mdi:close' />
                </button>
              </div>

              <label className={styles.modalLabel}>
                Name
                <input
                  className={styles.modalInput}
                  value={renameValue}
                  autoFocus
                  maxLength={120}
                  onChange={(e) => setRenameValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') void submitRename();
                    if (e.key === 'Escape' && !renameSaving) setRenameOpen(false);
                  }}
                  placeholder={currentName}
                  disabled={renameSaving}
                />
              </label>

              <div className={styles.modalActions}>
                <button
                  type='button'
                  className={styles.modalBtn}
                  disabled={renameSaving}
                  onClick={() => setRenameOpen(false)}
                >
                  Cancel
                </button>
                <button
                  type='button'
                  className={`${styles.modalBtn} ${styles.modalPrimaryBtn}`}
                  disabled={renameSaving}
                  onClick={() => void submitRename()}
                >
                  {renameSaving ? 'Saving...' : 'Save'}
                </button>
              </div>
            </div>
          </div>
        )
        : null}
    </div>
  );
});

export default ServerPage;
