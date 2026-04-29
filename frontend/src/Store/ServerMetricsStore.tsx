import { makeAutoObservable } from 'mobx';
import type { ServerItem } from '../Pages/Home/HomePage';
import { API_BASE } from '../Api/api';
import { tokenStore } from './TokenStore';
export const serverMetricsStore = {
  nowServers: [] as ServerItem[],
  ServerMetricsError: null as string | null,
  ServerMetricsLoading: true,
  refreshInFlight: null as Promise<void> | null,

  setNowServers(newServers: ServerItem[]) {
    this.nowServers = newServers;
  },
  setServerMetricsError(error: string | null) {
    this.ServerMetricsError = error;
  },
  setServerMetricsLoading(loading: boolean) {
    this.ServerMetricsLoading = loading;
  },

  getNowServers() {
    return this.nowServers;
  },

  updateServerName(serverId: number, name: string | null) {
    this.nowServers = this.nowServers.map(s => (s.id === serverId ? { ...s, name } : s));
  },

  async refreshNow(opts?: { silent?: boolean; }) {
    if (this.refreshInFlight) return this.refreshInFlight;

    const silent = Boolean(opts?.silent);
    if (!silent) {
      this.setServerMetricsLoading(true);
    }
    this.setServerMetricsError(null);

    this.refreshInFlight = (async () => {
      try {
        const token = tokenStore.getToken();
        const response = await fetch(`${API_BASE}/servers`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = await response.json();
        this.setNowServers(data);
        this.setServerMetricsError(null);
      } catch (err) {
        this.setServerMetricsError(err instanceof Error ? err.message : 'Failed to fetch servers');
      } finally {
        if (!silent) {
          this.setServerMetricsLoading(false);
        }
        this.refreshInFlight = null;
      }
    })();

    return this.refreshInFlight;
  },
};
makeAutoObservable(serverMetricsStore);
