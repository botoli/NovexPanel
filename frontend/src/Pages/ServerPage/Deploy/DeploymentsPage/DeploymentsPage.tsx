import { observer } from 'mobx-react-lite';

import { Icon } from '@iconify/react';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { NavLink, useParams } from 'react-router-dom';
import { serverMetricsStore } from '../../../../Store/ServerMetricsStore';
import { useCurrentServer } from '../../../../Store/ServerStore';
import { tokenStore } from '../../../../Store/TokenStore';
import styles from './DeploymentsPage.module.scss';
import { API_BASE } from '../../../../Api/api';
interface DeployData {
  serverId: number | undefined;
  repoUrl: string;
  branch: string;
  type: string;
  subdirectory: string | null;
  buildCommand: string | null;
  outputDir: string | null;
}
export const DeployPage = observer(() => {
  const { server } = useCurrentServer();
  const langs = [
    { 'name': 'Node.js', 'icon': 'logos:nodejs-icon', description: 'JavaScript runtime' },
    { 'name': 'Go', 'icon': 'logos:go', description: 'Go programming language' },
    { 'name': 'Python', 'icon': 'logos:python', description: 'Python programming language' },
    { 'name': 'Vite', 'icon': 'logos:vite-icon', description: 'Vite development server' },
  ];
  const [GithubUrl, setGithubUrl] = useState<string>('');
  const [choosedLanguage, setChoosedLanguage] = useState('Node.js');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [DeploymentProjects, setDeploymentProjects] = useState();
  const [subdirectory, setSubdirectory] = useState<string>('');
  const deployData: DeployData = useMemo(() => {
    console.log(GithubUrl);
    console.log(choosedLanguage);
    console.log(subdirectory);
    return {
      serverId: server?.id,
      repoUrl: GithubUrl,
      branch: 'master',
      type: choosedLanguage,
      subdirectory: subdirectory,
      buildCommand: null,
      outputDir: null,
    };
  }, [server?.id, GithubUrl, choosedLanguage, subdirectory]);

  const postData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/deploy`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
        body: JSON.stringify(deployData),
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      console.log(data);

      return data;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [GithubUrl, choosedLanguage, subdirectory]);

  return (
    <div className={styles.mainContent}>
      <div className={styles.headerRow}>
        <div className={styles.headerText}>
          <h1 className={styles.title}>Deploy</h1>
          <p className={styles.subtitle}>Deploy your application in minutes</p>
        </div>
        <NavLink to={`/servers/${server?.id}/deploy`}>
          <button type='button' className={styles.headerBtn}>
            <Icon icon='mdi:arrow-left' />
            Back
          </button>
        </NavLink>
      </div>

      <div className={styles.centerStage}>
        <section className={styles.deployCard} aria-label='Deploy your project'>
          <div className={styles.form}>
            <div className={styles.field}>
              <div className={styles.fieldLabelRow}>
                <p className={styles.label}>
                  GitHub Repository
                  <span className={styles.labelHint} aria-hidden='true'>
                    <Icon icon='mdi:help-circle-outline' />
                  </span>
                </p>
              </div>

              <div className={styles.inputShell}>
                <Icon icon='mdi:github' className={styles.inputIcon} />
                <input
                  className={styles.input}
                  type='url'
                  inputMode='url'
                  placeholder='https://github.com/username/repository'
                  value={GithubUrl}
                  onChange={(e) => setGithubUrl(e.target.value)}
                />
                <Icon icon='mdi:check-circle' className={styles.inputCheck} />
              </div>
              <input
                type='text'
                placeholder='Папка проекта (например, bot/)'
                value={subdirectory}
                onChange={(e) => setSubdirectory(e.target.value)}
              />
              <p className={styles.helperText}>
                <Icon icon='mdi:lock-outline' className={styles.helperIcon} />
                We only read your public repository. No code is stored.
              </p>
            </div>

            <div className={styles.field}>
              <div className={styles.fieldLabelRow}>
                <p className={styles.label}>
                  Build Technology
                  <span className={styles.labelHint} aria-hidden='true'>
                    <Icon icon='mdi:help-circle-outline' />
                  </span>
                </p>
              </div>

              <div className={styles.techGrid} role='list'>
                {langs.map((lang) => (
                  <button
                    key={lang.name}
                    type='button'
                    className={`${styles.techCard} ${
                      choosedLanguage === lang.name ? styles.techCardSelected : ''
                    }`}
                    onClick={() => {
                      setChoosedLanguage(lang.name);
                    }}
                  >
                    <span className={styles.techCheck} aria-hidden='true'>
                      {choosedLanguage === lang.name && <Icon icon='mdi:check' />}
                    </span>
                    <span className={styles.techIcon} aria-hidden='true'>
                      <Icon icon={lang.icon} />
                    </span>
                    <span className={styles.techName}>{lang.name}</span>
                    <span className={styles.techDesc}>{lang.description}</span>
                  </button>
                ))}
              </div>
            </div>

            <button type='button' className={styles.advancedRow}>
              <span>Advanced Options</span>
              <Icon icon='mdi:chevron-down' />
            </button>

            <button
              type='button'
              className={styles.primaryBtn}
              onClick={() => {
                console.log(GithubUrl);
                postData();
              }}
            >
              <Icon icon='mdi:rocket-launch-outline' />
              Deploy Project
            </button>
          </div>
        </section>
      </div>
    </div>
  );
});
