import { apiRequest } from '../client';

export type RiskLevel = 'low' | 'medium' | 'high' | 'critical';
export interface RiskAnalysisRequest { command: string; server_id: number; cwd?: string; }
export interface RiskFinding { code: string; title: string; details: string; affected_paths: string[]; }
export interface RiskAnalysisResponse { risk_level: RiskLevel; summary: string; findings: RiskFinding[]; blocked: boolean; requires_confirmation: boolean; policy_matches: string[]; sandbox_preview: string[]; }
export interface GuardedExecuteRequest { command: string; server_id: number; risk_ack: boolean; reason: string; }
export interface GuardedExecuteResponse { execution_id: string; status: 'queued' | 'running' | 'finished' | 'blocked'; trace: string[]; }

export const analyzeCommandRisk = (payload: RiskAnalysisRequest) => apiRequest<RiskAnalysisResponse>('/ai/command-guard/analyze', { method: 'POST', body: JSON.stringify(payload) });
export const guardedExecute = (payload: GuardedExecuteRequest) => apiRequest<GuardedExecuteResponse>('/ai/command-guard/execute', { method: 'POST', body: JSON.stringify(payload) });
export const loadCommandAudit = (serverId: number) => apiRequest<Array<{ id:number; command:string; risk_level:RiskLevel; decision:string; created_at:string }>>(`/servers/${serverId}/audit/commands`);
