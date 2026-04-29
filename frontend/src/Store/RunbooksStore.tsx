import { makeAutoObservable, runInAction } from 'mobx';
import {
  createRunbook,
  createRunbookVersion,
  deleteRunbook,
  dryRunRunbook,
  executeRunbook,
  getRunbook,
  getRunbookDiff,
  listRunbookAudit,
  listRunbookExecutionLogs,
  listRunbookExecutions,
  listRunbookVersions,
  listRunbooks,
  patchRunbook,
  rollbackRunbookExecution,
  type RunbookExecution,
  type RunbookSummary,
  type RunbookVersion,
} from '../Api/runbooks';
import { tokenStore } from './TokenStore';

const WS_BASE = import.meta.env.VITE_WS_URL || 'ws://localhost:8380';

export const runbooksStore = makeAutoObservable({
  loading: false,
  list: [] as RunbookSummary[],
  current: null as any,
  versions: [] as RunbookVersion[],
  executions: [] as RunbookExecution[],
  executionLogs: [] as any[],
  audit: [] as any[],
  dryRunPreview: null as any,
  diff: [] as any[],
  ws: null as WebSocket | null,
  selectedExecutionId: null as number | null,

  async loadList(serverId: number, search = '', tag = '') {
    this.loading = true;
    try {
      const data = await listRunbooks(serverId, search, tag);
      runInAction(() => { this.list = data; });
    } finally {
      runInAction(() => { this.loading = false; });
    }
  },

  async loadDetail(serverId: number, runbookId: number) {
    this.loading = true;
    try {
      const [detail, versions, executions, audit] = await Promise.all([
        getRunbook(serverId, runbookId),
        listRunbookVersions(serverId, runbookId),
        listRunbookExecutions(serverId, runbookId),
        listRunbookAudit(serverId, runbookId),
      ]);
      runInAction(() => {
        this.current = detail;
        this.versions = versions;
        this.executions = executions;
        this.audit = audit;
      });
    } finally {
      runInAction(() => { this.loading = false; });
    }
  },

  async create(serverId: number, payload: any) {
    await createRunbook(serverId, payload);
    await this.loadList(serverId);
  },

  async update(serverId: number, runbookId: number, payload: any) {
    await patchRunbook(serverId, runbookId, payload);
    await this.loadDetail(serverId, runbookId);
  },

  async remove(serverId: number, runbookId: number) {
    await deleteRunbook(serverId, runbookId);
    await this.loadList(serverId);
  },

  async addVersion(serverId: number, runbookId: number, payload: any) {
    await createRunbookVersion(serverId, runbookId, payload);
    await this.loadDetail(serverId, runbookId);
  },

  async loadDiff(serverId: number, runbookId: number, from: number, to: number) {
    const diff = await getRunbookDiff(serverId, runbookId, from, to);
    runInAction(() => { this.diff = diff?.diff ?? []; });
  },

  async dryRun(serverId: number, runbookId: number, payload: any) {
    const preview = await dryRunRunbook(serverId, runbookId, payload);
    runInAction(() => { this.dryRunPreview = preview?.preview ?? null; });
  },

  async execute(serverId: number, runbookId: number, payload: any) {
    const result = await executeRunbook(serverId, runbookId, payload);
    await this.loadDetail(serverId, runbookId);
    return result;
  },

  async rollback(serverId: number, runbookId: number, executionId: number) {
    await rollbackRunbookExecution(serverId, executionId);
    await this.loadDetail(serverId, runbookId);
  },

  async loadExecutionLogs(serverId: number, executionId: number) {
    const logs = await listRunbookExecutionLogs(serverId, executionId);
    runInAction(() => {
      this.selectedExecutionId = executionId;
      this.executionLogs = logs;
    });
  },

  subscribeExecutionLogs(executionId: number) {
    this.unsubscribeExecutionLogs();
    const token = tokenStore.getToken();
    if (!token) return;
    const ws = new WebSocket(`${WS_BASE}/site/ws?token=${token}`);
    this.ws = ws;
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'subscribe_runbook_execution_logs', execution_id: executionId }));
    };
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data?.execution_id !== executionId) return;
      if (data?.type === 'runbook_execution_log') {
        runInAction(() => {
          this.executionLogs = [...this.executionLogs, data];
        });
      }
      if (data?.type === 'runbook_execution_status') {
        runInAction(() => {
          this.executionLogs = [...this.executionLogs, data];
          this.executions = this.executions.map(ex => (ex.id === executionId
            ? { ...ex, status: data.status ?? ex.status, summary: data.summary ?? ex.summary }
            : ex));
        });
      }
    };
  },

  unsubscribeExecutionLogs() {
    if (!this.ws) return;
    if (this.ws.readyState === WebSocket.OPEN && this.selectedExecutionId) {
      this.ws.send(JSON.stringify({ type: 'unsubscribe_runbook_execution_logs', execution_id: this.selectedExecutionId }));
    }
    this.ws.close();
    this.ws = null;
  },
});
