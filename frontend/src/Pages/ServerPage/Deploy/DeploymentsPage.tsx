import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useCurrentServer } from '../../../Store/ServerStore';
import { tokenStore } from '../../../Store/TokenStore';

import { NavLink } from 'react-router-dom';
import { API_BASE } from '../../../Api/api';
import styles from './DeploymentsPage.module.scss';
interface DeployData {
  serverId: number | undefined;
  repoUrl: string;
  branch: string;
  type: string;
  createdAt: string;
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

  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [DeploymentProjects, setDeploymentProjects] = useState<DeployData[] | null>(null);
  const deleteDeploy = async (id: number) => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/deploys/${id}`, {
        method: 'DELETE',
        headers: { authorization: `Bearer ${tokenStore.getToken()}` },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
      return false;
    } finally {
      setLoading(false);
    }
  };
  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
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
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  }, [server?.id]);
  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => {
      isMounted.current = false;
      clearInterval(interval);
    };
  }, []);
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
                    <Icon icon='mdi:code-tags' className={styles.thIcon} />Language
                  </th>
                  <th>
                    <Icon icon='mdi:git-branch' className={styles.thIcon} />Branch
                  </th>
                  <th>
                    <Icon icon='mdi:clock-outline' className={styles.thIcon} />Created At
                  </th>
                  <th>
                    <Icon icon='mdi:github' className={styles.thIcon} />Repository
                  </th>
                  <th>
                    <Icon icon='mdi:signal' className={styles.thIcon} />Status
                  </th>
                  <th>
                    <Icon icon='mdi:link-variant' className={styles.thIcon} />URL
                  </th>
                  <th className={styles.actionsTh}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {DeploymentProjects?.length === 0
                  ? (
                    <tr>
                      <td colSpan={7} className={styles.emptyState}>
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
                        <td className={styles.colType}>
                          <span className={styles.langBadge}>{project.type}</span>
                        </td>
                        <td className={styles.colBranch}>
                          <Icon icon='mdi:git' className={styles.cellIcon} />
                          {project.branch}
                        </td>
                        <td className={styles.colDate}>
                          <Icon icon='mdi:calendar' className={styles.cellIcon} />
                          {new Date(project.createdAt).toLocaleDateString()}
                        </td>
                        <td className={styles.colRepo}>
                          <code className={styles.repoCode}>{project.repoUrl}</code>
                        </td>
                        <td className={styles.colStatus}>
                          <span
                            className={`${styles.statusBadge} ${
                              styles[`status-${project.status.toLowerCase()}`]
                            }`}
                          >
                            <span className={styles.statusDot} />
                            {project.status}
                          </span>
                        </td>
                        <td className={styles.colUrl}>
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
                        </td>
                        <td className={styles.colActions}>
                          <button
                            className={styles.deleteBtn}
                            onClick={() =>
                              deleteDeploy(project.id)}
                            title='Stop deployment'
                          >
                            <Icon icon='mdi:stop' />
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  );
});
