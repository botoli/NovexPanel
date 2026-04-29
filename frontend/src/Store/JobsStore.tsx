import { makeAutoObservable, runInAction } from 'mobx';
import { apiRequest } from '../Api/client';

export type Job = {
  id: number;
  name: string;
  type: string;
  command: string;
  status: string;
  cron_expr?: string;
};

export type JobRun = {
  id: number;
  status: string;
  triggered_by: string;
  started_at: string;
  finished_at?: string;
  error?: string;
};

export const jobsStore = makeAutoObservable({
  loading: false,
  jobs: [] as Job[],
  runs: [] as JobRun[],
  logs: [] as Array<{ id: number; line: string; stream: string; created_at: string; }>,

  async loadJobs(serverId: number) {
    this.loading = true;
    try {
      const data = await apiRequest<Job[]>(`/servers/${serverId}/jobs`);
      runInAction(() => { this.jobs = data; });
    } finally {
      runInAction(() => { this.loading = false; });
    }
  },

  async createJob(serverId: number, payload: { name: string; type: string; command: string; cron?: string; }) {
    await apiRequest<{ id: number; }>(`/servers/${serverId}/jobs`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    await this.loadJobs(serverId);
  },

  async runJob(jobId: number) {
    await apiRequest(`/jobs/${jobId}/run`, { method: 'POST' });
  },

  async loadJobDetails(jobId: number) {
    const [runs, logs] = await Promise.all([
      apiRequest<JobRun[]>(`/jobs/${jobId}/runs`),
      apiRequest<Array<{ id: number; line: string; stream: string; created_at: string; }>>(`/jobs/${jobId}/logs`),
    ]);
    runInAction(() => {
      this.runs = runs;
      this.logs = logs;
    });
  },
});
