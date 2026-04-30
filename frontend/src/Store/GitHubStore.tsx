import { makeAutoObservable, runInAction } from 'mobx';
import { apiRequest } from '../Api/client';

export type GitHubConnection = {
  connected: boolean;
  login?: string;
  avatar_url?: string;
  scope?: string;
};

export const githubStore = makeAutoObservable({
  loading: false,
  error: '',
  connection: { connected: false } as GitHubConnection,
  repos: [] as Array<{ id: number; full_name: string; default_branch: string; clone_url: string; html_url: string; }>,

  async loadConnection() {
    this.loading = true;
    try {
      const data = await apiRequest<GitHubConnection>('/integrations/github');
      runInAction(() => {
        this.connection = data;
      });
    } finally {
      runInAction(() => { this.loading = false; });
    }
  },

  async connect() {
    runInAction(() => {
      this.error = '';
    });
    try {
      const data = await apiRequest<{ url: string; }>('/integrations/github/start');
      window.open(data.url, '_blank', 'noopener,noreferrer,width=900,height=700');
    } catch (error) {
      runInAction(() => {
        this.error = (error as Error).message;
      });
      throw error;
    }
  },

  async loadRepos() {
    const data = await apiRequest<Array<{ id: number; full_name: string; default_branch: string; clone_url: string; html_url: string; }>>('/integrations/github/repos');
    runInAction(() => { this.repos = data; });
  },

  async disconnect() {
    await apiRequest<void>('/integrations/github/disconnect', { method: 'POST' });
    runInAction(() => {
      this.connection = { connected: false };
      this.repos = [];
      this.error = '';
    });
  },
});
