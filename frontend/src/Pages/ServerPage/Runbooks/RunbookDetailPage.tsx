import { observer } from 'mobx-react-lite';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Confirm } from '../../../modals/Confirm/Confirm';
import { runbooksStore } from '../../../Store/RunbooksStore';
import { useCurrentServer } from '../../../Store/ServerStore';
import styles from './RunbookDetailPage.module.scss';

const RunbookDetailPage = observer(() => {
  const { runbookId } = useParams<{ runbookId: string; }>();
  const { serverId } = useCurrentServer();
  const navigate = useNavigate();
  const [variablesJson, setVariablesJson] = useState('{}');
  const [confirmExecute, setConfirmExecute] = useState(false);
  const [fromVersion, setFromVersion] = useState<number>(0);
  const [toVersion, setToVersion] = useState<number>(0);

  const runbookIdNum = Number(runbookId);

  useEffect(() => {
    if (!Number.isFinite(serverId) || !Number.isFinite(runbookIdNum)) return;
    void runbooksStore.loadDetail(serverId, runbookIdNum);
  }, [serverId, runbookIdNum]);

  useEffect(() => () => runbooksStore.unsubscribeExecutionLogs(), []);

  const executionStatusClass = (status: string) => {
    const value = status.toLowerCase();
    if (value === 'success' || value === 'rolled_back') return styles.success;
    if (value === 'running' || value === 'pending') return styles.running;
    return styles.failed;
  };

  const parsedVariables = useMemo(() => {
    try {
      return JSON.parse(variablesJson);
    } catch {
      return {};
    }
  }, [variablesJson]);

  if (!Number.isFinite(serverId) || !Number.isFinite(runbookIdNum)) return null;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h2>{runbooksStore.current?.title || 'Runbook'}</h2>
          <p>{runbooksStore.current?.description}</p>
        </div>
        <div className={styles.actions}>
          <button type='button' className={styles.btn} onClick={() => setConfirmExecute(true)}>Execute runbook</button>
          <button
            type='button'
            className={styles.btn}
            onClick={() => {
              if (!runbooksStore.current) return;
              void runbooksStore.dryRun(serverId, runbookIdNum, {
                version: runbooksStore.current.latest_version,
                variables: parsedVariables,
              });
            }}
          >
            Dry-run preview
          </button>
          <button
            type='button'
            className={styles.btnDanger}
            onClick={async () => {
              await runbooksStore.remove(serverId, runbookIdNum);
              navigate(`/servers/${serverId}/runbooks`);
            }}
          >
            Delete
          </button>
        </div>
      </div>

      <div className={styles.grid}>
        <section className={styles.card}>
          <h3>Variables editor</h3>
          <textarea className={styles.textarea} value={variablesJson} onChange={e => setVariablesJson(e.target.value)} />
        </section>

        <section className={styles.card}>
          <h3>Version timeline</h3>
          <div className={styles.timeline}>
            {runbooksStore.versions.map(version => (
              <button
                type='button'
                key={version.id}
                className={styles.timelineItem}
                onClick={() => {
                  setFromVersion(version.version);
                  if (toVersion === 0) setToVersion(runbooksStore.current?.latest_version ?? version.version);
                }}
              >
                v{version.version} {version.change_note ? `— ${version.change_note}` : ''}
              </button>
            ))}
          </div>
        </section>

        <section className={styles.card}>
          <h3>Diff between versions</h3>
          <div className={styles.row}>
            <input className={styles.input} type='number' value={fromVersion || ''} onChange={e => setFromVersion(Number(e.target.value || 0))} placeholder='from version' />
            <input className={styles.input} type='number' value={toVersion || ''} onChange={e => setToVersion(Number(e.target.value || 0))} placeholder='to version' />
            <button
              type='button'
              className={styles.btn}
              onClick={() => {
                if (fromVersion > 0 && toVersion > 0) {
                  void runbooksStore.loadDiff(serverId, runbookIdNum, fromVersion, toVersion);
                }
              }}
            >
              Compare
            </button>
          </div>
          <pre className={styles.pre}>{JSON.stringify(runbooksStore.diff, null, 2)}</pre>
        </section>

        <section className={styles.card}>
          <h3>Dry-run preview</h3>
          <pre className={styles.pre}>{JSON.stringify(runbooksStore.dryRunPreview, null, 2)}</pre>
        </section>
      </div>

      <section className={styles.card}>
        <h3>Execution history</h3>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Started</th>
                <th>Finished</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {runbooksStore.executions.map(execution => (
                <tr key={execution.id}>
                  <td>{execution.id}</td>
                  <td><span className={`${styles.badge} ${executionStatusClass(execution.status)}`}>{execution.status}</span></td>
                  <td>{execution.started_at ? new Date(execution.started_at).toLocaleString() : '—'}</td>
                  <td>{execution.finished_at ? new Date(execution.finished_at).toLocaleString() : '—'}</td>
                  <td className={styles.row}>
                    <button
                      type='button'
                      className={styles.btn}
                      onClick={async () => {
                        await runbooksStore.loadExecutionLogs(serverId, execution.id);
                        runbooksStore.subscribeExecutionLogs(execution.id);
                      }}
                    >
                      Live logs
                    </button>
                    <button type='button' className={styles.btn} onClick={() => runbooksStore.rollback(serverId, runbookIdNum, execution.id)}>Rollback</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className={styles.card}>
        <h3>Execution logs</h3>
        <div className={styles.logs}>
          {runbooksStore.executionLogs.map((item, idx) => (
            <div key={idx} className={styles.logLine}>
              [{item.status || item.stream || 'info'}] {item.line || item.summary || ''}
            </div>
          ))}
        </div>
      </section>

      <section className={styles.card}>
        <h3>Audit trail</h3>
        <div className={styles.logs}>
          {runbooksStore.audit.map(item => (
            <div key={item.id} className={styles.logLine}>
              {item.action} — {new Date(item.created_at).toLocaleString()}
            </div>
          ))}
        </div>
      </section>

      <Confirm
        isOpen={confirmExecute}
        title='Execute runbook'
        description='Runbook will execute commands on the connected server. Continue?'
        confirmText='Execute'
        onCancel={() => setConfirmExecute(false)}
        onConfirm={async () => {
          setConfirmExecute(false);
          const res = await runbooksStore.execute(serverId, runbookIdNum, {
            version: runbooksStore.current?.latest_version,
            variables: parsedVariables,
          });
          await runbooksStore.loadExecutionLogs(serverId, res.execution_id);
          runbooksStore.subscribeExecutionLogs(res.execution_id);
        }}
      />
    </div>
  );
});

export default RunbookDetailPage;
