import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { useCurrentServer } from '../../../Store/ServerStore';
import { jobsStore } from '../../../Store/JobsStore';
import styles from './JobsPage.module.scss';

const JobsPage = observer(() => {
  const { serverId } = useCurrentServer();
  const [name, setName] = useState('');
  const [command, setCommand] = useState('');
  const [selectedJob, setSelectedJob] = useState<number | null>(null);

  useEffect(() => {
    if (Number.isFinite(serverId)) {
      void jobsStore.loadJobs(serverId);
    }
  }, [serverId]);

  useEffect(() => {
    if (selectedJob) void jobsStore.loadJobDetails(selectedJob);
  }, [selectedJob]);

  if (!Number.isFinite(serverId)) return null;

  return (
    <div className={styles.wrap}>
      <section className={styles.card}>
        <h2>Actions / Jobs</h2>
        <div className={styles.createRow}>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder='Job name' />
          <input value={command} onChange={(e) => setCommand(e.target.value)} placeholder='Command' />
          <button
            type='button'
            onClick={async () => {
              await jobsStore.createJob(serverId, { name, type: 'custom', command });
              setName('');
              setCommand('');
            }}
          >
            Create Job
          </button>
        </div>
      </section>

      <section className={styles.grid}>
        <div className={styles.card}>
          <h3>Jobs</h3>
          {jobsStore.jobs.map((job) => (
            <div key={job.id} className={styles.jobRow}>
              <button type='button' onClick={() => setSelectedJob(job.id)}>{job.name}</button>
              <span>{job.status}</span>
              <button type='button' onClick={() => jobsStore.runJob(job.id)}>Run</button>
            </div>
          ))}
        </div>
        <div className={styles.card}>
          <h3>Runs & Logs</h3>
          {selectedJob
            ? (
              <>
                <div className={styles.logs}>
                  {jobsStore.logs.map(log => <div key={log.id}>{log.line}</div>)}
                </div>
              </>
            )
            : <p>Select job</p>}
        </div>
      </section>
    </div>
  );
});

export default JobsPage;
