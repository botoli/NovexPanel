import { makeAutoObservable } from 'mobx';

export const globalStore = {
  loading: false,
  error: null as string | null,
  setLoading(loading: boolean) {
    this.loading = loading;
  },
  setError(error: string | null) {
    this.error = error;
  },
};
makeAutoObservable(globalStore);
