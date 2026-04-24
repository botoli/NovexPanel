import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { data, Link, useParams } from 'react-router-dom';
import { API_BASE } from '../../../../Api/api';
import { DeployStore } from '../../../../Store/DeployStore';
import { useCurrentServer } from '../../../../Store/ServerStore';
import { tokenStore } from '../../../../Store/TokenStore';
import styles from './DeploymentDetailPage.module.scss';
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
// {
//   "branch": "master",
//   "buildCommand": "",
//   "build_command": "",
//   "createdAt": "2026-04-23T02:51:36.305176+03:00",
//   "envVars": {},
//   "env_vars": {},
//   "errorMessage": null,
//   "error_message": null,
//   "finishedAt": "2026-04-23T02:52:31.703185+03:00",
//   "finished_at": "2026-04-23T02:52:31.703185+03:00",
//   "id": 2,
//   "outputDir": "",
//   "output_dir": "",
//   "port": 44859,
//   "repoUrl": "https://github.com/botoli/NovexPanel.git",
//   "serverId": 1,
//   "status": "running",
//   "subdirectory": "frontend",
//   "type": "vite",
//   "updatedAt": "2026-04-23T02:52:31.703559+03:00",
//   "url": "http://192.168.0.100:44859"
// }
export const DeploymentDetailPage = observer(() => {
  // const [envVisible, setEnvVisible] = useState<Record<string, boolean>>({});
  const [deployData, setDeployData] = useState<DeployData | null>(null);
  const [deployLogs, setDeployLogs] = useState();
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
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
      console.log(data);
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
      const response = await fetch(`${API_BASE}/deploys/${DeployStore.getDeployId()}/log `, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      setDeployLogs(data);
      console.log(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  };
  // {
  //   "deployId": 3,
  //   "deployLog": "running: git clone --depth 1 --branch master https://github.com/botoli/NovexPanel.git src in /tmp/novex-deploy-3-788127518\nКлонирование в «src»...\nusing subdirectory: /tmp/novex-deploy-3-788127518/src/frontend\ndetected project type: vite\nvite project: installing dependencies (npm install)\nrunning: npm install in /tmp/novex-deploy-3-788127518/src/frontend\nadded 299 packages, and audited 300 packages in 12s\n74 packages are looking for funding\n  run `npm fund` for details\nfound 0 vulnerabilities\nnpm WARN EBADENGINE Unsupported engine {\nnpm WARN EBADENGINE   package: 'eslint-visitor-keys@5.0.1',\nnpm WARN EBADENGINE   required: { node: '^20.19.0 || ^22.13.0 || >=24' },\nnpm WARN EBADENGINE   current: { node: 'v18.19.1', npm: '9.2.0' }\nnpm WARN EBADENGINE }\nnpm WARN EBADENGINE Unsupported engine {\nnpm WARN EBADENGINE   package: '@vitejs/plugin-react@6.0.1',\nnpm WARN EBADENGINE   required: { node: '^20.19.0 || >=22.12.0' },\nnpm WARN EBADENGINE   current: { node: 'v18.19.1', npm: '9.2.0' }\nnpm WARN EBADENGINE }\nnpm WARN EBADENGINE Unsupported engine {\nnpm WARN EBADENGINE   package: 'react-router@7.14.0',\nnpm WARN EBADENGINE   required: { node: '>=20.0.0' },\nnpm WARN EBADENGINE   current: { node: 'v18.19.1', npm: '9.2.0' }\nnpm WARN EBADENGINE }\nnpm WARN EBADENGINE Unsupported engine {\nnpm WARN EBADENGINE   package: 'react-router-dom@7.14.0',\nnpm WARN EBADENGINE   required: { node: '>=20.0.0' },\nnpm WARN EBADENGINE   current: { node: 'v18.19.1', npm: '9.2.0' }\nnpm WARN EBADENGINE }\nnpm WARN EBADENGINE Unsupported engine {\nnpm WARN EBADENGINE   package: 'rolldown@1.0.0-rc.15',\nnpm WARN EBADENGINE   required: { node: '^20.19.0 || >=22.12.0' },\nnpm WARN EBADENGINE   current: { node: 'v18.19.1', npm: '9.2.0' }\nnpm WARN EBADENGINE }\nnpm WARN EBADENGINE Unsupported engine {\nnpm WARN EBADENGINE   package: 'vite@8.0.8',\nnpm WARN EBADENGINE   required: { node: '^20.19.0 || >=22.12.0' },\nnpm WARN EBADENGINE   current: { node: 'v18.19.1', npm: '9.2.0' }\nnpm WARN EBADENGINE }\nnpm WARN deprecated xterm-addon-fit@0.8.0: This package is now deprecated. Move to @xterm/addon-fit instead.\nnpm WARN deprecated xterm@5.3.0: This package is now deprecated. Move to @xterm/xterm instead.\nvite project: building static assets (npm run build)\nrunning: npm run build in /tmp/novex-deploy-3-788127518/src/frontend\n> qwe@0.0.0 build\n> tsc -b && vite build\nYou are using Node.js 18.19.1. Vite requires Node.js version 20.19+ or 22.12+. Please upgrade your Node.js version.\nfile:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:534\n\t\t\t\tthis.dispatchEvent(new CustomEvent(`command:${commandName}`, { detail: command }));\n\t\t\t\t                       ^\nReferenceError: CustomEvent is not defined\n    at CAC.parse (file:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:534:28)\n    at file:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:835:5\n    at ModuleJob.run (node:internal/modules/esm/module_job:195:25)\n    at async ModuleLoader.import (node:internal/modules/esm/loader:336:24)\nNode.js v18.19.1\nnpm run build failed: npm run build failed in /tmp/novex-deploy-3-788127518/src/frontend: exit status 1. output: You are using Node.js 18.19.1. Vite requires Node.js version 20.19+ or 22.12+. Please upgrade your Node.js version.\nfile:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:534\n\t\t\t\tthis.dispatchEvent(new CustomEvent(`command:${commandName}`, { detail: command }));\n\t\t\t\t                       ^\n\nReferenceError: CustomEvent is not defined\n    at CAC.parse (file:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:534:28)\n    at file:///tmp/n...\nnpm run build failed: npm run build failed in /tmp/novex-deploy-3-788127518/src/frontend: exit status 1. output: You are using Node.js 18.19.1. Vite requires Node.js version 20.19+ or 22.12+. Please upgrade your Node.js version.\nfile:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:534\n\t\t\t\tthis.dispatchEvent(new CustomEvent(`command:${commandName}`, { detail: command }));\n\t\t\t\t                       ^\n\nReferenceError: CustomEvent is not defined\n    at CAC.parse (file:///tmp/novex-deploy-3-788127518/src/frontend/node_modules/vite/dist/node/cli.js:534:28)\n    at file:///tmp/n...\n",
  //   "port": 0,
  //   "status": "failed",
  //   "subdirectory": "frontend",
  //   "type": "vite",
  //   "url": ""
  // }
  // const toggleEnv = useCallback((key: string) => {
  //   setEnvVisible((prev) => ({ ...prev, [key]: !prev[key] }));
  // }, []);
  useEffect(() => {
    fetchDeployData();
    fetchDeployLogs();
  }, []);
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
              {deployData?.createdAt && new Date(deployData.createdAt).toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
              })}
            </span>
          </div>
          <div className={styles.infoItem}>
            <span className={styles.infoLabel}>Обновлён</span>
            <span className={styles.infoValue}>
              {deployData?.updatedAt && new Date(deployData.updatedAt).toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
              })}
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
        {
          /* <div className={styles.logToolbar}>
          <button
            type='button'
            className={`${styles.btn} ${styles.btnSecondary}`}
            onClick={downloadBuildLog}
          >
            <Icon icon='mdi:download' />
            Скачать логи
          </button>{' '}
          <button
            type='button'
            className={`${styles.btn} ${styles.btnDanger}`}
            onClick={clearBuildLogLocal}
          >
            <Icon icon='mdi:eraser' />
            Очистить
          </button>
        </div> */
        }
        <textarea
          className={styles.logArea}
          value={deployLogs?.deployLog}
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
        {
          /* <div className={styles.logToolbar}>
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
        </div> */
        }
        <pre
          className={`${styles.logStream} `}
          role='log'
        >
          {deployLogs?.deployLog}
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
});
