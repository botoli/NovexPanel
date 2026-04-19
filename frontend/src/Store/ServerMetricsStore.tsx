import { makeAutoObservable } from 'mobx';
import type { ServerItem } from '../Pages/Home/HomePage';
export const serverMetricsStore = {
  nowServers: [] as ServerItem[],
  ServerMetricsError: null as string | null,
  ServerMetricsLoading: false,

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
};
makeAutoObservable(serverMetricsStore);
