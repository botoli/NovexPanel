import { Icon } from '@iconify/react';

import { observer } from 'mobx-react-lite';
import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { serverMetricsStore } from '../../../Store/ServerMetricsStore';
import styles from './ProcessesPage.module.scss';

type ProcessTone = 'calm' | 'watch' | 'hot';

const getProcessTone = (cpu: number, mem: number): ProcessTone => {
  if (cpu >= 70 || mem >= 70) return 'hot';
  if (cpu >= 35 || mem >= 35) return 'watch';
  return 'calm';
};

const TONE_LABEL: Record<ProcessTone, string> = {
  calm: 'Stable',
  watch: 'Watch',
  hot: 'High',
};

const TONE_CLASS: Record<ProcessTone, string> = {
  calm: styles.toneCalm,
  watch: styles.toneWatch,
  hot: styles.toneHot,
};

const ProcessesSkeleton = () => (
  <section className={styles.processes}>
    <header className={styles.header}>
      <div className={styles.heading}>
        <div className={`${styles.skeletonLine} ${styles.skeletonTitle}`} />
        <div className={`${styles.skeletonLine} ${styles.skeletonSubtitle}`} />
      </div>

      <div className={styles.toolbar}>
        <div className={`${styles.skeletonControl} ${styles.skeletonSearch}`} />
        <div className={`${styles.skeletonControl} ${styles.skeletonButton}`} />
      </div>
    </header>

    <div className={styles.tableCard}>
      <div className={styles.tableSkeleton}>
        {Array.from({ length: 7 }).map((_, index) => (
          <div key={`process-row-skeleton-${index}`} className={styles.skeletonRow}>
            <span className={`${styles.skeletonLine} ${styles.skeletonPid}`} />
            <span className={`${styles.skeletonLine} ${styles.skeletonName}`} />
            <span className={`${styles.skeletonLine} ${styles.skeletonNum}`} />
            <span className={`${styles.skeletonLine} ${styles.skeletonNum}`} />
            <span className={`${styles.skeletonLine} ${styles.skeletonStatus}`} />
            <span className={`${styles.skeletonLine} ${styles.skeletonAction}`} />
          </div>
        ))}
      </div>
    </div>
  </section>
);

const ProcessesPage = observer(() => {
  const [searchText, setSearchText] = useState<string>('');
  const { id } = useParams<{ id?: string; }>();
  const serverId = id ? Number(id) : Number.NaN;

  const allServers = serverMetricsStore.getNowServers();
  const server = allServers.find(s => s.id === serverId);

  const filteredProcesses = useMemo(() => {
    if (!server) return [];

    return server.last_metrics.top_processes.filter(p =>
      p.name.toLowerCase().includes(searchText.toLowerCase())
    );
  }, [server, searchText]);

  if (!Number.isFinite(serverId)) {
    return <div className={styles.stateMessage}>Invalid server id</div>;
  }

  if (allServers.length === 0) {
    return <ProcessesSkeleton />;
  }

  if (!server) {
    return <div className={styles.stateMessage}>Server not found</div>;
  }

  const hasProcesses = filteredProcesses.length > 0;

  return (
    <section className={styles.processes}>
      <header className={styles.header}>
        <div className={styles.heading}>
          <h2 className={styles.title}>
            <Icon icon='mdi:format-list-bulleted-square' className={styles.titleIcon} />
            Processes
          </h2>

          <p className={styles.subtitle}>Live list of processes running on the server.</p>
        </div>

        <div className={styles.toolbar}>
          <label className={styles.search} aria-label='Filter processes by name'>
            <Icon icon='mdi:magnify' />
            <input
              type='text'
              placeholder='Filter by name...'
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
            />
          </label>

          <button type='button' className={styles.refreshBtn}>
            <Icon icon='mdi:refresh' />
            Refresh
          </button>
        </div>
      </header>

      <div className={styles.metaRow}>
        <span className={styles.metaPill}>
          <Icon icon='mdi:layers-triple-outline' />
          {filteredProcesses.length} visible
        </span>
        <span className={styles.metaHint}>Actions are shown on row hover.</span>
      </div>

      <div className={styles.tableCard}>
        {hasProcesses
          ? (
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.pidCol}>PID</th>
                  <th>Name</th>
                  <th className={styles.numCol}>CPU</th>
                  <th className={styles.numCol}>MEM</th>
                  <th className={styles.stateCol}>State</th>
                  <th className={styles.actionCol}>Action</th>
                </tr>
              </thead>
              <tbody>
                {filteredProcesses.map((process) => {
                  const tone = getProcessTone(process.cpu, process.mem);

                  return (
                    <tr key={process.pid}>
                      <td className={styles.pidCell}>{process.pid}</td>
                      <td className={styles.nameCell}>
                        <span className={styles.processName}>{process.name}</span>
                      </td>
                      <td className={styles.numCell}>{process.cpu.toFixed(0)}%</td>
                      <td className={styles.numCell}>{process.mem.toFixed(0)}%</td>
                      <td className={styles.stateCell}>
                        <span className={`${styles.stateBadge} ${TONE_CLASS[tone]}`}>
                          <span className={styles.stateDot} />
                          {TONE_LABEL[tone]}
                        </span>
                      </td>
                      <td className={styles.actionCell}>
                        <button type='button' className={styles.killBtn}>
                          <Icon icon='mdi:close-thick' className={styles.btnIcon} />
                          Kill
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )
          : (
            <div className={styles.emptyState}>
              <Icon icon='mdi:file-search-outline' />
              No processes found for this filter.
            </div>
          )}
      </div>
    </section>
  );
});

export default ProcessesPage;
