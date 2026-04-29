import { apiRequest } from './client';

export type RunbookSummary = {
  id: number;
  title: string;
  slug: string;
  description: string;
  tags: string[];
  target_type: 'server' | 'service' | 'cluster';
  latest_version: number;
  last_run_status?: string;
  last_run_at?: string;
  updated_at: string;
};

export type RunbookVersion = {
  id: number;
  runbook_id: number;
  version: number;
  definition_json: string;
  created_by: number;
  change_note: string;
  is_rollback_point: boolean;
  created_at: string;
};

export type RunbookExecution = {
  id: number;
  runbook_id: number;
  runbook_version_id: number;
  dry_run: boolean;
  status: string;
  started_at?: string;
  finished_at?: string;
  triggered_by: number;
  summary: string;
  created_at: string;
};

export function listRunbooks(serverId: number, search = '', tag = '') {
  const params = new URLSearchParams();
  if (search.trim()) params.set('search', search.trim());
  if (tag.trim()) params.set('tag', tag.trim());
  const suffix = params.toString() ? `?${params.toString()}` : '';
  return apiRequest<RunbookSummary[]>(`/servers/${serverId}/runbooks${suffix}`);
}

export function getRunbook(serverId: number, runbookId: number) {
  return apiRequest<any>(`/servers/${serverId}/runbooks/${runbookId}`);
}

export function createRunbook(serverId: number, payload: any) {
  return apiRequest<{ id: number; }>(`/servers/${serverId}/runbooks`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function patchRunbook(serverId: number, runbookId: number, payload: any) {
  return apiRequest(`/servers/${serverId}/runbooks/${runbookId}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  });
}

export function deleteRunbook(serverId: number, runbookId: number) {
  return apiRequest(`/servers/${serverId}/runbooks/${runbookId}`, { method: 'DELETE' });
}

export function listRunbookVersions(serverId: number, runbookId: number) {
  return apiRequest<RunbookVersion[]>(`/servers/${serverId}/runbooks/${runbookId}/versions`);
}

export function createRunbookVersion(serverId: number, runbookId: number, payload: any) {
  return apiRequest<{ version: number; }>(`/servers/${serverId}/runbooks/${runbookId}/versions`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function getRunbookDiff(serverId: number, runbookId: number, from: number, to: number) {
  return apiRequest<any>(`/servers/${serverId}/runbooks/${runbookId}/diff?from=${from}&to=${to}`);
}

export function dryRunRunbook(serverId: number, runbookId: number, payload: any) {
  return apiRequest<any>(`/servers/${serverId}/runbooks/${runbookId}/dry-run`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function executeRunbook(serverId: number, runbookId: number, payload: any) {
  return apiRequest<{ execution_id: number; status: string; }>(`/servers/${serverId}/runbooks/${runbookId}/execute`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function rollbackRunbookExecution(serverId: number, executionId: number) {
  return apiRequest<{ execution_id: number; }>(`/servers/${serverId}/runbooks/rollback`, {
    method: 'POST',
    body: JSON.stringify({ execution_id: executionId }),
  });
}

export function listRunbookExecutions(serverId: number, runbookId: number) {
  return apiRequest<RunbookExecution[]>(`/servers/${serverId}/runbooks/${runbookId}/executions`);
}

export function listRunbookExecutionLogs(serverId: number, executionId: number) {
  return apiRequest<any[]>(`/servers/${serverId}/runbooks/executions/${executionId}/logs`);
}

export function listRunbookAudit(serverId: number, runbookId: number) {
  return apiRequest<any[]>(`/servers/${serverId}/runbooks/${runbookId}/audit`);
}
