import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { API_BASE } from '../../../../Api/api';
import { DeployStore } from '../../../../Store/DeployStore';
import { useCurrentServer } from '../../../../Store/ServerStore';
import { tokenStore } from '../../../../Store/TokenStore';
import styles from './DeploymentDetailPage.module.scss';
const WS_BASE = import.meta.env.VITE_WS_URL || 'ws://localhost:8380';
function formatDateTime(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}
interface DeployData {
  branch: string;
  buildCommand: string | null;
  createdAt: string | Date;
  id: number;
  outputDir: string | null;
  port: number;
  repoUrl: string;
  serverId: number;
  status: string;
  subdirectory: string | null;
  type: string;
  updatedAt: string | Date;
  url: string;
}

export const DeploymentDetailPage = observer(() => {
  const navigate = useNavigate();
  // const [envVisible, setEnvVisible] = useState<Record<string, boolean>>({});
  const [deployData, setDeployData] = useState<DeployData | null>(null);
  const [deployLogs, setDeployLogs] = useState<
    { deploy_id: number; line: string; stream: string; }[]
  >([]);
  const [appLogs, setAppLogs] = useState<{ line: string; stream: string; }[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const { server } = useCurrentServer();
  const fetchDeployData = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/deploys/${DeployStore.getDeployId()}`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      setDeployData(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  };
  const fetchDeployLogs = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/deploys/${DeployStore.getDeployId()}/log`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      const newLines = data.lines.map((lineItem: any) => ({
        deploy_id: DeployStore.getDeployId(),
        line: lineItem.line,
        stream: lineItem.stream,
      }));
      setDeployLogs(prev => [...prev, ...newLines]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
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
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
      return false;
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    const ws = new WebSocket(`${WS_BASE}/site/ws?token=${tokenStore.getToken()}`);
    wsRef.current = ws;

    ws.onopen = () => {
      ws.send(
        JSON.stringify({
          type: 'subscribe_deploy_logs',
          deploy_id: DeployStore.getDeployId(),
        }),
      );
      ws.send(JSON.stringify({
        type: 'subscribe_runtime_logs',
        deploy_id: DeployStore.getDeployId(),
      }));
    };
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.type === 'deploy_log_line') {
        const newLine = {
          deploy_id: DeployStore.getDeployId(),
          line: data.line,
          stream: data.stream,
        };
        setDeployLogs(prev => [...prev, newLine]);
      }
      if (data.type === 'runtime_log_line') {
        setAppLogs(prev => [...prev, {
          line: data.line,
          stream: data.stream,
        }]);
      }
    };
    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'unsubscribe_deploy_logs',
          deploy_id: DeployStore.getDeployId(),
        }));
      }
      ws.send(JSON.stringify({
        type: 'unsubscribe_runtime_logs',
        deploy_id: DeployStore.getDeployId(),
      }));
      ws.close();
    };
  }, [DeployStore.getDeployId()]);

  useEffect(() => {
    fetchDeployData();
    fetchDeployLogs();
  }, []);
  useEffect(() => {
    console.log({ loading, error });
  }, [error]);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  const handleScroll = () => {
    if (!logContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current;
    const isAtBottom = scrollTop + clientHeight >= scrollHeight - 50;
    setAutoScroll(isAtBottom);
  };
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [deployLogs, autoScroll]);

  return (
    <div className={styles.root}>
      <div className={styles.backRow}>
        <Link to={`/servers/${server?.id}/deployments/`} className={styles.backLink}>
          <Icon icon='mdi:arrow-left' />
          К списку деплоев
        </Link>
      </div>

      {/* 1. Шапка */}
      <header className={styles.pageHeader}>
        <div className={styles.titleRow}>
          <div className={styles.titleBlock}>
            <h1 className={styles.title}>
              <span>Деплой #{deployData?.id}</span>
              <span
                className={`${styles.statusBadge} `}
                role='status'
              >
                <span className={styles.statusDot} />
                {deployData?.status}
              </span>
            </h1>
            {DeployStore.deployId != null && (
              <span
                className={styles.infoLabel}
                style={{ textTransform: 'none', letterSpacing: 'normal' }}
              >
                ID: {DeployStore.deployId}
              </span>
            )}
          </div>
          <div className={styles.headerActions}>
            <button type='button' className={`${styles.btn} ${styles.btnSecondary}`}>
              <Icon icon='mdi:restart' />
              Restart
            </button>
            <button type='button' className={`${styles.btn} ${styles.btnSecondary}`}>
              <Icon icon='mdi:stop' />
              Stop
            </button>
            <button type='button' className={`${styles.btn} ${styles.btnSecondary}`}>
              <Icon icon='mdi:text-box-outline' />
              Logs
            </button>
            <button
              type='button'
              className={`${styles.btn} ${styles.btnDanger}`}
              onClick={() => {
                deleteDeploy(DeployStore.getDeployId());
                navigate(`/servers/${server?.id}/deployments/`);
              }}
            >
              <Icon icon='mdi:delete-outline' />
              Delete
            </button>
          </div>
        </div>

        <div className={styles.infoGrid}>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Репозиторий</span>
            <a
              className={styles.link}
              href={deployData?.repoUrl}
              target='_blank'
              rel='noopener noreferrer'
            >
              {deployData?.repoUrl}
            </a>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Ветка</span>
            <span className={styles.infoValue}>{deployData?.branch}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Тип</span>
            <span className={styles.infoValue}>{deployData?.type}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Папка проекта</span>
            <span className={styles.infoValue}>{deployData?.subdirectory || '—'}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Создан</span>
            <span className={styles.infoValue}>
              {formatDateTime(deployData?.createdAt ? deployData.createdAt.toString() : '')}
            </span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Обновлён</span>
            <span className={styles.infoValue}>
              {formatDateTime(deployData?.updatedAt ? deployData.updatedAt.toString() : '')}
            </span>
          </div>
        </div>
      </header>

      {/* 2. Переменные окружения */}
      {
        /* <section className={styles.section} aria-labelledby='env-heading'>
        <h2 className={styles.sectionTitle} id='env-heading'>
          Переменные окружения
        </h2>
        {deployData?.env.length === 0
          ? <p className={styles.emptyHint}>Нет переменных окружения</p>
          : (
            <div className={styles.tableWrap}>
              <table className={styles.dataTable}>
                <thead>
                  <tr>
                    <th>KEY</th>
                    <th>VALUE</th>
                    <th style={{ width: 1 }} />
                  </tr>
                </thead>
                {
                  /* <tbody>
                  {data.env.map((row) => {
                    const vis = envVisible[row.key] === true;
                    return (
                      <tr key={row.key}>
                        <td>
                          <code>{row.key}</code>
                        </td>
                        <td>
                          <code>
                            {vis
                              ? row.value
                              : '*'.repeat(Math.min(32, Math.max(8, row.value.length)))}
                          </code>
                        </td>
                        <td>
                          <button
                            type='button'
                            className={styles.showBtn}
                            onClick={() => toggleEnv(row.key)}
                          >
                            {vis ? 'скрыть' : 'показать'}
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody> */
      }
      {
        /* </table>
            </div>
          )}
      </section> */
      }

      {/* 3. Сборка и запуск */}
      <section className={styles.section} aria-labelledby='build-heading'>
        <h2 className={styles.sectionTitle} id='build-heading'>
          Сборка и запуск
        </h2>
        <div className={styles.kvList}>
          {deployData?.buildCommand
            ? (
              <div className={styles.kvRow}>
                <span className={styles.kvKey}>Команда сборки</span>
                <span className={styles.kvValue}>
                  <code>{deployData?.buildCommand}</code>
                </span>
              </div>
            )
            : null}
          {deployData?.outputDir
            ? (
              <div className={styles.kvRow}>
                <span className={styles.kvKey}>Выходная папка</span>
                <span className={styles.kvValue}>
                  <code>{deployData?.outputDir}</code>
                </span>
              </div>
            )
            : null}
          <div className={styles.kvRow}>
            <span className={styles.kvKey}>Порт приложения</span>
            <span className={styles.kvValue}>{deployData?.port}</span>
          </div>
          <div className={styles.kvRow}>
            <span className={styles.kvKey}>Внешний порт</span>
            <span className={styles.kvValue}>{deployData?.port}</span>
          </div>
          <div className={styles.kvRow}>
            <span className={styles.kvKey}>Ссылка на сервис</span>
            <span className={styles.kvValue}>
              <a
                className={styles.link}
                href={deployData?.url}
                target='_blank'
                rel='noopener noreferrer'
              >
                {deployData?.url}
              </a>
            </span>
          </div>
        </div>
      </section>

      {
        /* 4. Ресурсы
      <section className={styles.section} aria-labelledby='res-heading'>
        <h2 className={styles.sectionTitle} id='res-heading'>
          Ресурсы
        </h2>
        <div className={styles.metricsRow}>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>CPU</span>
            <span className={styles.metricValue}>{}</span>
          </div>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>RAM</span>
            <span className={styles.metricValue}>{deployData?.resources.ram}</span>
          </div>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>Uptime</span>
            <span className={styles.metricValue}>{deployData?.resources.uptime}</span>
          </div>
          {deployData?.resources.hostPid
            ? (
              <div className={styles.metric}>
                <span className={styles.metricLabel}>PID (хост)</span>
                <span className={styles.metricValue}>{deployData?.resources.hostPid}</span>
              </div>
            )
            : null}
        </div>
      </section> */
      }

      {/* 5. Логи сборки */}
      <section className={styles.section} aria-labelledby='buildlog-heading'>
        <h2 className={styles.sectionTitle} id='buildlog-heading'>
          Логи сборки
        </h2>

        <div className={styles.logArea} ref={logContainerRef} onScroll={handleScroll}>
          {deployLogs.map((log, idx) => (
            <pre
              key={idx}
              className={log.stream === 'stderr' ? styles.errorLine : undefined}
              style={{ margin: 0, fontFamily: 'monospace' }}
            >
      {log.line}
            </pre>
          ))}
        </div>
      </section>

      {/* 6. Логи работы приложения */}
      <section className={styles.section} aria-labelledby='runtimelog-heading'>
        <h2 className={styles.sectionTitle}>
          Логи работы приложения
        </h2>
        <textarea
          className={styles.logArea}
          readOnly
          spellCheck={false}
          value={appLogs.map(line => line.line).join('\n')}
          aria-label='Логи работы приложения'
        />
      </section>

      {/* 7. Действия */}
      <section className={styles.section} aria-labelledby='actions-heading'>
        <h2 className={styles.sectionTitle} id='actions-heading'>
          Действия
        </h2>
        <div className={styles.actionsList}>
          <div className={styles.actionRow}>
            <span className={styles.actionLabel}>Перезапустить контейнер</span>
            <button type='button' className={`${styles.btn} ${styles.btnSecondary}`}>
              <Icon icon='mdi:restart' />
              Restart
            </button>
          </div>
          <div className={styles.actionRow}>
            <span className={styles.actionLabel}>Остановить контейнер</span>
            <button type='button' className={`${styles.btn} ${styles.btnSecondary}`}>
              <Icon icon='mdi:stop' />
              Stop
            </button>
          </div>
          <div className={styles.actionRow}>
            <span className={styles.actionLabel}>
              Удалить деплой: контейнер, образ, запись в БД
            </span>
            <button type='button' className={`${styles.btn} ${styles.btnDanger}`}>
              <Icon icon='mdi:delete-outline' />
              Delete
            </button>
          </div>
          <div className={styles.actionRow}>
            <span className={styles.actionLabel}>
              Запустить новый деплой с теми же параметрами (новая сборка)
            </span>
            <button type='button' className={`${styles.btn} ${styles.btnPrimary}`}>
              <Icon icon='mdi:rocket-launch' />
              Redeploy
            </button>
          </div>
        </div>
      </section>
    </div>
  );
});
