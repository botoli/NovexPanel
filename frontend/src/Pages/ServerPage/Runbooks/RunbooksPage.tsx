import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { runbooksStore } from '../../../Store/RunbooksStore';
import { useCurrentServer } from '../../../Store/ServerStore';
import styles from './RunbooksPage.module.scss';

const starterDefinition = {
  steps: [{ name: 'check', type: 'shell', command: 'echo ok', timeout: 20 }],
  variables: [],
  timeout: 120,
  retry_policy: { max_attempts: 1 },
};

const RunbooksPage = observer(() => {
  const { serverId } = useCurrentServer();
  const [search, setSearch] = useState('');
  const [tag, setTag] = useState('');

  useEffect(() => {
    if (!Number.isFinite(serverId)) return;
    void runbooksStore.loadList(serverId, search, tag);
  }, [serverId, search, tag]);

  const tags = useMemo(() => {
    const set = new Set<string>();
    runbooksStore.list.forEach(item => item.tags.forEach(tagItem => set.add(tagItem)));
    return [...set].sort();
  }, [runbooksStore.list]);

  if (!Number.isFinite(serverId)) return null;

  return (
    <div className={styles.page}>
      <div className={styles.topRow}>
        <h2>Runbooks</h2>
        <button
          type='button'
          className={styles.btn}
          onClick={async () => {
            await runbooksStore.create(serverId, {
              title: 'New Runbook',
              slug: `runbook-${Date.now()}`,
              description: 'Runbook description',
              tags: ['custom'],
              target_type: 'server',
              definition: starterDefinition,
              change_note: 'initial version',
            });
          }}
        >
          Create runbook
        </button>
      </div>

      <div className={styles.filters}>
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder='Search runbooks'
          className={styles.input}
        />
        <select value={tag} onChange={(e) => setTag(e.target.value)} className={styles.select}>
          <option value=''>All tags</option>
          {tags.map(item => <option key={item} value={item}>{item}</option>)}
        </select>
      </div>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Title</th>
              <th>Slug</th>
              <th>Status</th>
              <th>Latest Version</th>
              <th>Last Run</th>
            </tr>
          </thead>
          <tbody>
            {runbooksStore.list.map(runbook => (
              <tr key={runbook.id}>
                <td>
                  <Link to={`/servers/${serverId}/runbooks/${runbook.id}`} className={styles.link}>
                    {runbook.title}
                  </Link>
                </td>
                <td>{runbook.slug}</td>
                <td>
                  <span className={styles.badge}>{runbook.last_run_status || 'never'}</span>
                </td>
                <td>v{runbook.latest_version}</td>
                <td>{runbook.last_run_at ? new Date(runbook.last_run_at).toLocaleString() : '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
});

export default RunbooksPage;
