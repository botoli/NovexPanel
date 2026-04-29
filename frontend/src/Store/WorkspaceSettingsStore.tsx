import { makeAutoObservable, runInAction } from 'mobx';
import { apiRequest } from '../Api/client';

type Member = { id: number; email: string; role: string; };
type APITokenItem = { id: number; name: string; token_prefix: string; revoked: boolean; expires_at: string; };

export const workspaceSettingsStore = makeAutoObservable({
  members: [] as Member[],
  apiTokens: [] as APITokenItem[],
  revealToken: '' as string,

  async loadMembers() {
    const data = await apiRequest<Member[]>('/settings/members');
    runInAction(() => { this.members = data; });
  },
  async addMember(email: string, role: string) {
    await apiRequest('/settings/members', { method: 'POST', body: JSON.stringify({ email, role }) });
    await this.loadMembers();
  },
  async removeMember(id: number) {
    await apiRequest(`/settings/members/${id}`, { method: 'DELETE' });
    await this.loadMembers();
  },
  async loadApiTokens() {
    const data = await apiRequest<APITokenItem[]>('/settings/api-tokens');
    runInAction(() => { this.apiTokens = data; });
  },
  async createApiToken(name: string, expiresInDays = 90) {
    const data = await apiRequest<{ token: string; }>('/settings/api-tokens', {
      method: 'POST',
      body: JSON.stringify({ name, expires_in_days: expiresInDays }),
    });
    runInAction(() => { this.revealToken = data.token; });
    await this.loadApiTokens();
  },
  clearRevealToken() {
    this.revealToken = '';
  },
});
