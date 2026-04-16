import { makeAutoObservable } from 'mobx';
import type { ServerItem } from '../Pages/Home/HomePage';
export const serverMetricsStore = {
  serverMetrics: [] as ServerItem[],
  nowServers: [] as ServerItem[],
  ServerMetricsError: null as string | null,
  ServerMetricsLoading: false,
  setServerMetrics(newMetrics: ServerItem[]) {
    this.serverMetrics = [...this.serverMetrics, ...newMetrics];
  },
  setNowServers(newServers: ServerItem[]) {
    this.nowServers = newServers;
  },
  setServerMetricsError(error: string | null) {
    this.ServerMetricsError = error;
  },
  setServerMetricsLoading(loading: boolean) {
    this.ServerMetricsLoading = loading;
  },
  getServerMetrics() {
    return this.serverMetrics;
  },
  getNowServers() {
    return this.nowServers;
  },
  clearServerMetrics() {
    this.serverMetrics = [];
  },
};
makeAutoObservable(serverMetricsStore);
