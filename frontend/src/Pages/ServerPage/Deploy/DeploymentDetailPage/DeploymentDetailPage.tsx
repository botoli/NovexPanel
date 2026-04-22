import { Icon } from '@iconify/react';
import { useCallback, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import styles from './DeploymentDetailPage.module.scss';

// Мок-данные: заменить на запросы / state при подключении логики
const MOCK = {
  deployNumber: 20,
  status: 'Running' as 'Running' | 'Failed' | 'Stopped',
  repoUrl: 'https://github.com/botoli/case3',
  branch: 'main',
  projectType: 'Go' as 'Go' | 'Node.js' | 'Python' | 'Vite' | string,
  subdirectory: 'backend',
  createdAt: '2026-04-21T14:30:00+03:00',
  updatedAt: '2026-04-23T09:12:00+03:00',
  env: [
    { key: 'NODE_ENV', value: 'production' },
    { key: 'DATABASE_URL', value: 'postgres://user:secret@host:5432/db' },
    { key: 'API_KEY', value: 'sk_live_abc123' },
  ] as { key: string; value: string; }[],
  buildCommand: 'go build -o app .',
  outputDir: '' as string | null,
  appPort: 8080,
  externalPort: 34787,
  serviceUrl: 'http://192.168.0.100:34787',
  resources: {
    cpu: '1.2%',
    ram: '48 MB',
    uptime: '2h 15m',
    hostPid: '48291' as string | null,
  },
  buildLog: 'running: git clone --depth 1 --branch main https://github.com/botoli/case3.git src\n'
    + 'Cloning into \'src\'...\n'
    + 'using subdirectory: /tmp/novex-deploy-20-4216213173/src/backend\n'
    + 'detected project type: go\n'
    + 'running: go mod download\n'
    + 'running: go build -o app .\n'
    + 'Dockerfile not found, using process runtime\n'
    + 'app binary found and executable\n',
  runtimeLogSeed: '[stdout] server listening on :8080\n[stderr] (none)\n[stdout] GET / 200 2ms\n',
} as const;

function formatDateTime(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

function statusClass(s: (typeof MOCK)['status']) {
  if (s === 'Failed') return styles.statusFailed;
  if (s === 'Stopped') return styles.statusStopped;
  return styles.statusRunning;
}

export function DeploymentDetailPage() {
  const { id, deployId } = useParams();

  const [buildLog, setBuildLog] = useState<string>(MOCK.buildLog);
  const [envVisible, setEnvVisible] = useState<Record<string, boolean>>({});
  const [runtimeLog, setRuntimeLog] = useState<string>(MOCK.runtimeLogSeed);
  const [runtimeStreamOn, setRuntimeStreamOn] = useState(false);

  const data = MOCK;

  const serverLinkBase = useMemo(
    () => (id != null && id.length > 0 ? `/servers/${id}/deployments` : '/'),
    [id],
  );

  const toggleEnv = useCallback((key: string) => {
    setEnvVisible((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  const clearBuildLogLocal = useCallback(() => {
    setBuildLog('');
  }, []);

  const downloadBuildLog = useCallback(() => {
    const blob = new Blob([buildLog], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `deploy-${data.deployNumber}-build-log.txt`;
    a.click();
    URL.revokeObjectURL(url);
  }, [buildLog, data.deployNumber]);

  const startRuntime = useCallback(() => {
    setRuntimeStreamOn(true);
  }, []);

  const stopRuntime = useCallback(() => {
    setRuntimeStreamOn(false);
  }, []);

  const clearRuntime = useCallback(() => {
    setRuntimeLog('');
  }, []);

  return (
    <div className={styles.root}>
      <div className={styles.backRow}>
        <Link to={serverLinkBase} className={styles.backLink}>
          <Icon icon='mdi:arrow-left' />
          К списку деплоев
        </Link>
      </div>

      {/* 1. Шапка */}
      <header className={styles.pageHeader}>
        <div className={styles.titleRow}>
          <div className={styles.titleBlock}>
            <h1 className={styles.title}>
              <span>Деплой #{data.deployNumber}</span>
              <span className={`${styles.statusBadge} ${statusClass(data.status)}`} role='status'>
                <span className={styles.statusDot} />
                {data.status}
              </span>
            </h1>
            {deployId != null && (
              <span
                className={styles.infoLabel}
                style={{ textTransform: 'none', letterSpacing: 'normal' }}
              >
                ID: {deployId}
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
            <button type='button' className={`${styles.btn} ${styles.btnDanger}`}>
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
              href={data.repoUrl}
              target='_blank'
              rel='noopener noreferrer'
            >
              {data.repoUrl}
            </a>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Ветка</span>
            <span className={styles.infoValue}>{data.branch}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Тип</span>
            <span className={styles.infoValue}>{data.projectType}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Папка проекта</span>
            <span className={styles.infoValue}>{data.subdirectory || '—'}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Создан</span>
            <span className={styles.infoValue}>{formatDateTime(data.createdAt)}</span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Обновлён</span>
            <span className={styles.infoValue}>{formatDateTime(data.updatedAt)}</span>
          </div>
        </div>
      </header>

      {/* 2. Переменные окружения */}
      <section className={styles.section} aria-labelledby='env-heading'>
        <h2 className={styles.sectionTitle} id='env-heading'>
          Переменные окружения
        </h2>
        {data.env.length === 0
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
                <tbody>
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
                </tbody>
              </table>
            </div>
          )}
      </section>

      {/* 3. Сборка и запуск */}
      <section className={styles.section} aria-labelledby='build-heading'>
        <h2 className={styles.sectionTitle} id='build-heading'>
          Сборка и запуск
        </h2>
        <div className={styles.kvList}>
          {data.buildCommand
            ? (
              <div className={styles.kvRow}>
                <span className={styles.kvKey}>Команда сборки</span>
                <span className={styles.kvValue}>
                  <code>{data.buildCommand}</code>
                </span>
              </div>
            )
            : null}
          {data.outputDir
            ? (
              <div className={styles.kvRow}>
                <span className={styles.kvKey}>Выходная папка</span>
                <span className={styles.kvValue}>
                  <code>{data.outputDir}</code>
                </span>
              </div>
            )
            : null}
          <div className={styles.kvRow}>
            <span className={styles.kvKey}>Порт приложения</span>
            <span className={styles.kvValue}>{data.appPort}</span>
          </div>
          <div className={styles.kvRow}>
            <span className={styles.kvKey}>Внешний порт</span>
            <span className={styles.kvValue}>{data.externalPort}</span>
          </div>
          <div className={styles.kvRow}>
            <span className={styles.kvKey}>Ссылка на сервис</span>
            <span className={styles.kvValue}>
              <a
                className={styles.link}
                href={data.serviceUrl}
                target='_blank'
                rel='noopener noreferrer'
              >
                {data.serviceUrl}
              </a>
            </span>
          </div>
        </div>
      </section>

      {/* 4. Ресурсы */}
      <section className={styles.section} aria-labelledby='res-heading'>
        <h2 className={styles.sectionTitle} id='res-heading'>
          Ресурсы
        </h2>
        <div className={styles.metricsRow}>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>CPU</span>
            <span className={styles.metricValue}>{data.resources.cpu}</span>
          </div>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>RAM</span>
            <span className={styles.metricValue}>{data.resources.ram}</span>
          </div>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>Uptime</span>
            <span className={styles.metricValue}>{data.resources.uptime}</span>
          </div>
          {data.resources.hostPid
            ? (
              <div className={styles.metric}>
                <span className={styles.metricLabel}>PID (хост)</span>
                <span className={styles.metricValue}>{data.resources.hostPid}</span>
              </div>
            )
            : null}
        </div>
      </section>

      {/* 5. Логи сборки */}
      <section className={styles.section} aria-labelledby='buildlog-heading'>
        <h2 className={styles.sectionTitle} id='buildlog-heading'>
          Логи сборки
        </h2>
        <div className={styles.logToolbar}>
          <button
            type='button'
            className={`${styles.btn} ${styles.btnSecondary}`}
            onClick={downloadBuildLog}
          >
            <Icon icon='mdi:download' />
            Скачать логи
          </button>
          <button
            type='button'
            className={`${styles.btn} ${styles.btnDanger}`}
            onClick={clearBuildLogLocal}
          >
            <Icon icon='mdi:eraser' />
            Очистить
          </button>
        </div>
        <textarea
          className={styles.logArea}
          value={buildLog}
          onChange={(e) => setBuildLog(e.target.value)}
          readOnly
          spellCheck={false}
          aria-label='Логи сборки (deploy_log)'
        />
      </section>

      {/* 6. Логи работы приложения */}
      <section className={styles.section} aria-labelledby='runtimelog-heading'>
        <h2 className={styles.sectionTitle} id='runtimelog-heading'>
          Логи работы приложения
        </h2>
        <div className={styles.logToolbar}>
          <button
            type='button'
            className={`${styles.btn} ${styles.btnSecondary}`}
            onClick={startRuntime}
            disabled={runtimeStreamOn}
          >
            <Icon icon='mdi:play' />
            Старт
          </button>
          <button
            type='button'
            className={`${styles.btn} ${styles.btnSecondary}`}
            onClick={stopRuntime}
            disabled={!runtimeStreamOn}
          >
            <Icon icon='mdi:stop' />
            Стоп
          </button>
          <button
            type='button'
            className={`${styles.btn} ${styles.btnDanger}`}
            onClick={clearRuntime}
          >
            <Icon icon='mdi:eraser' />
            Очистить
          </button>
        </div>
        <pre
          className={`${styles.logStream} ${runtimeStreamOn ? styles.logStreamActive : ''}`}
          role='log'
        >
          {runtimeLog || (runtimeStreamOn ? 'Ожидание логов…' : 'Поток остановлен. Нажмите «Старт» для сценария с WebSocket.')}
        </pre>
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
}
