import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useCurrentServer } from '../../../Store/ServerStore';
import { tokenStore } from '../../../Store/TokenStore';

import { NavLink, useNavigate } from 'react-router-dom';
import { API_BASE } from '../../../Api/api';
import { Confirm } from '../../../modals/Confirm/Confirm';
import { DeployStore } from '../../../Store/DeployStore';
import { refreshStore } from '../../../Store/RefreshStore';
import { toastStore } from '../../../Store/ToastStore';
import styles from './DeploymentsPage.module.scss';
interface DeployData {
  serverId: number | undefined;
  repoUrl: string;
  commitHash?: string;
  commitAuthor?: string;
  commitMessage?: string;
  branch: string;
  type: string;
  createdAt: string;
  updatedAt?: string;
  deployLogPreview?: string;
  id: number;
  status: string;
  url: string;
  subdirectory: string | null;
  buildCommand: string | null;
  outputDir: string | null;
}
export const DeploymentsPage = observer(() => {
  const { server } = useCurrentServer();
  const isMounted = useRef(true);
  const navigate = useNavigate();
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [DeploymentProjects, setDeploymentProjects] = useState<DeployData[] | null>(null);
  const [confirmId, setConfirmId] = useState<number | null>(null);
  const [expandedLogs, setExpandedLogs] = useState<Set<number>>(() => new Set());

  const getRepoName = (repoUrl: string) => {
    try {
      const cleaned = repoUrl.replace(/\.git$/i, '');
      const parts = cleaned.split('/');
      return parts[parts.length-1] || cleaned;
    } catch {
      return repoUrl;
    }
  };

  const shortHash = (hash?: string) => {
    const h = (hash || '').trim();
    if (!h) return '';
    return h.length > 10 ? h.slice(0, 10) : h;
  };

  const statusMeta = (status: string) => {
    const s = (status || '').toLowerCase();
    if (s === 'running') return { icon: 'mdi:check-circle-outline', label: 'running' };
    if (s === 'failed' || s === 'error') return { icon: 'mdi:alert-circle-outline', label: 'failed' };
    if (s === 'queued' || s === 'pending') return { icon: 'mdi:clock-outline', label: 'queued' };
    if (s === 'building' || s === 'deploying') return { icon: 'mdi:progress-wrench', label: s };
    if (s === 'stopped') return { icon: 'mdi:stop-circle-outline', label: 'stopped' };
    return { icon: 'mdi:information-outline', label: s || 'unknown' };
  };

  const toggleLogs = (id: number) => {
    setExpandedLogs((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const lastLogLines = (preview?: string, maxLines: number = 6) => {
    const text = (preview || '').trimEnd();
    if (!text) return [];
    const lines = text.split(/\r?\n/).filter(Boolean);
    return lines.slice(Math.max(0, lines.length - maxLines));
  };

  const deleteDeploy = async (id: number) => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/deploys/${id}`, {
        method: 'DELETE',
        headers: { authorization: `Bearer ${tokenStore.getToken()}` },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      fetchData();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
      return false;
    } finally {
      setLoading(false);
    }
  };
  const fetchData = useCallback(async (opts?: { silent?: boolean; }) => {
    try {
      const silent = Boolean(opts?.silent);
      if (!silent) setLoading(true);
      setError(null);
      const response = await fetch(
        `${API_BASE}/deploys?serverId=${server?.id}`,
        {
          headers: { Authorization: `Bearer ${tokenStore.getToken()}` },
        },
      );
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      setDeploymentProjects(data);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Произошла ошибка';
      setError(msg);
      if (!opts?.silent) {
        toastStore.push('error', msg, 'Deployments');
      }
    } finally {
      if (!opts?.silent) setLoading(false);
    }
  }, [server?.id]);
  useEffect(() => {
    fetchData();
    const interval = setInterval(() => fetchData({ silent: true }), 2000);
    return () => {
      isMounted.current = false;
      clearInterval(interval);
    };
  }, []);

  useEffect(() => {
    // Manual refresh from global refresh button.
    void fetchData();
  }, [refreshStore.refreshKey]);
  useEffect(() => {
    console.log({ loading, error });
  }, [error]);
  //   [
  //   {
  //     "branch": "master",
  //     "buildCommand": "",
  //     "createdAt": "2026-04-21T02:24:19.979704+03:00",
  //     "deployLogPreview": "running: git clone --depth 1 --branch master https://github.com/botoli/case3.git src in /tmp/novex-deploy-17-4216213173\nCloning into 'src'...\nusing subdirectory: /tmp/novex-deploy-17-4216213173/src/backend\ndetected project type: go\nrunning: go mod download in /tmp/novex-deploy-17-4216213173/src/backend\nrunning: go build -o app . in /tmp/novex-deploy-17-4216213173/src/backend\nDockerfile not found, using process runtime\napp binary found and executable: /tmp/novex-deploy-17-4216213173/src/backend/app\n",
  //     "id": 17,
  //     "outputDir": "",
  //     "port": 34787,
  //     "repoUrl": "https://github.com/botoli/case3.git",
  //     "serverId": 1,
  //     "status": "running",
  //     "subdirectory": "backend",
  //     "type": "go",
  //     "updatedAt": "2026-04-21T02:24:23.945948+03:00",
  //     "url": "http://192.168.0.100:34787"
  //   }
  // ]

  return (
    <div>
      <header className={styles.pageHeader}>
        <div className={styles.headerContent}>
          <h1>
            <Icon icon='mdi:rocket-launch' className={styles.headerIcon} />
            Deployments
          </h1>
          {DeploymentProjects !== null && (
            <span className={styles.deployCount}>
              {DeploymentProjects?.length}
            </span>
          )}
        </div>
        <NavLink to={`/servers/${server?.id}/deploy`}>
          <button type='button' className={styles.headerBtn}>
            <Icon icon='mdi:plus' />
            Create New Deployment
          </button>
        </NavLink>
      </header>

      <section className={styles.DeploysTable}>
        <div className={styles.tableCard}>
          <div className={styles.tableScroll}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>
                    <Icon icon='mdi:source-repository' className={styles.thIcon} />Project
                  </th>
                  <th>
                    <Icon icon='mdi:git-branch' className={styles.thIcon} />Branch
                  </th>
                  <th>
                    <Icon icon='mdi:signal' className={styles.thIcon} />Status
                  </th>
                  <th>
                    <Icon icon='mdi:clock-outline' className={styles.thIcon} />Updated
                  </th>
                  <th>
                    <Icon icon='mdi:link-variant' className={styles.thIcon} />URL
                  </th>
                  <th className={styles.actionsTh}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {loading && DeploymentProjects === null
                  ? (
                    <tr>
                      <td colSpan={6} className={styles.emptyState}>
                        <Icon icon='mdi:progress-clock' className={styles.emptyIcon} />
                        <p>Loading deployments...</p>
                        <span className={styles.emptyHint}>Fetching latest state</span>
                      </td>
                    </tr>
                  )
                  : error
                  ? (
                    <tr>
                      <td colSpan={6} className={styles.emptyState}>
                        <Icon icon='mdi:alert-outline' className={styles.emptyIcon} />
                        <p>Error</p>
                        <span className={styles.emptyHint}>{error}</span>
                      </td>
                    </tr>
                  )
                  : DeploymentProjects?.length === 0
                  ? (
                    <tr>
                      <td colSpan={6} className={styles.emptyState}>
                        <Icon icon='mdi:inbox-outline' className={styles.emptyIcon} />
                        <p>No deployments yet</p>
                        <span className={styles.emptyHint}>
                          Create your first deployment to get started
                        </span>
                      </td>
                    </tr>
                  )
                  : (
                    DeploymentProjects?.map((project) => (
                      <tr key={project.id} className={styles.tableRow}>
                        <td className={styles.colRepo}>
                          <div className={styles.repoBlock}>
                            <div className={styles.repoMain}>
                              <Icon icon='mdi:folder-outline' className={styles.cellIcon} />
                              <strong className={styles.repoName}>{getRepoName(project.repoUrl)}</strong>
                            </div>
                            <code className={styles.repoCode}>{project.repoUrl}</code>
                            {(project.commitMessage || project.commitHash || project.commitAuthor)
                              ? (
                                <div className={styles.commitMeta}>
                                  {project.commitMessage ? <span>{project.commitMessage}</span> : null}
                                  {project.commitHash ? <span className={styles.commitHash}>{shortHash(project.commitHash)}</span> : null}
                                  {project.commitAuthor ? <span>by {project.commitAuthor}</span> : null}
                                </div>
                              )
                              : null}
                          </div>
                        </td>

                        <td className={styles.colBranch}>
                          <span className={styles.inlineInfo}>
                            <Icon icon='mdi:git-branch' className={styles.cellIcon} />
                            {project.branch}
                          </span>
                        </td>

                        <td className={styles.colStatus}>
                          {(() => {
                            const meta = statusMeta(project.status);
                            return (
                              <span
                                className={`${styles.statusBadge} ${
                                  styles[`status-${project.status.toLowerCase()}`]
                                }`}
                              >
                                <Icon icon={meta.icon} className={styles.cellIcon} />
                                <span className={styles.statusDot} />
                                {meta.label}
                              </span>
                            );
                          })()}
                        </td>

                        <td className={styles.colDate}>
                          <span className={styles.inlineInfo}>
                            <Icon icon='mdi:calendar' className={styles.cellIcon} />
                            {project.updatedAt
                              ? new Date(project.updatedAt).toLocaleString()
                              : new Date(project.createdAt).toLocaleString()}
                          </span>
                        </td>

                        <td className={styles.colUrl}>
                          <div className={styles.urlBlock}>
                            {project.url
                              ? (
                                <a
                                  href={project.url}
                                  target='_blank'
                                  rel='noopener noreferrer'
                                  className={styles.urlLink}
                                >
                                  <Icon icon='mdi:open-in-new' className={styles.linkIcon} />
                                  {project.url.replace(/^https?:\/\//, '')}
                                </a>
                              )
                              : <span className={styles.urlEmpty}>—</span>}
                            {project.deployLogPreview
                              ? (
                                <button
                                  type='button'
                                  className={styles.logToggle}
                                  onClick={() => toggleLogs(project.id)}
                                >
                                  <Icon icon={expandedLogs.has(project.id) ? 'mdi:chevron-up' : 'mdi:chevron-down'} />
                                  Logs
                                </button>
                              )
                              : null}
                          </div>
                        </td>

                        <td className={styles.colActions}>
                          <button
                            className={styles.deleteBtn}
                            onClick={() => {
                              setConfirmId(project.id);
                            }}
                            title='Stop deployment'
                          >
                            <Confirm
                              isOpen={confirmId === project.id}
                              title='Confirm Deletion'
                              description='Are you sure you want to delete this deployment? This action cannot be undone.'
                              confirmText='Delete'
                              danger={true} // кнопка будет красной ($color-status-offline)
                              onConfirm={() => deleteDeploy(project.id)}
                              onCancel={() => setConfirmId(null)}
                            />
                            <Icon icon='mdi:stop' />
                          </button>
                          <button
                            className={styles.detailBtn}
                            onClick={() => {
                              navigate(`/servers/${server?.id}/deployments/${project.id}`);
                              DeployStore.setDeployId(project.id);
                            }}
                            title='View details'
                          >
                            <Icon icon='weui:arrow-filled' />
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
              </tbody>
            </table>
            {DeploymentProjects?.map((project) => (
              expandedLogs.has(project.id) && project.deployLogPreview
                ? (
                  <div key={`logs-${project.id}`} className={styles.logPreview}>
                    <div className={styles.logPreviewTitle}>Deployment #{project.id} logs</div>
                    <pre>{lastLogLines(project.deployLogPreview, 8).join('\n')}</pre>
                  </div>
                )
                : null
            ))}
          </div>
        </div>
      </section>
    </div>
  );
});
