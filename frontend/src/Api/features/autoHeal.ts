import { apiRequest } from '../client';

export interface AutoHealRule {
  id: number;
  name: string;
  metric: string;
  condition: 'gt' | 'lt' | 'eq';
  threshold: number;
  duration_seconds: number;
  action: 'restart' | 'reload' | 'runbook' | 'alert' | 'rollback_deploy' | 'block_traffic';
  retry_limit: number;
  cooldown_seconds: number;
  enabled: boolean;
}

export const fetchRules = (serverId: number) => apiRequest<AutoHealRule[]>(`/servers/${serverId}/auto-heal/rules`);
export const createRule = (serverId: number, payload: Omit<AutoHealRule, 'id'>) => apiRequest<AutoHealRule>(`/servers/${serverId}/auto-heal/rules`, { method: 'POST', body: JSON.stringify(payload) });
export const fetchHistory = (serverId: number) => apiRequest<Array<{ id:number; rule_name:string; status:string; message:string; created_at:string }>>(`/servers/${serverId}/auto-heal/history`);
