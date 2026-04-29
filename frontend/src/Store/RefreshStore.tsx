import { makeAutoObservable, runInAction } from 'mobx';

type RefreshScope = 'all' | 'server';

export const refreshStore = makeAutoObservable({
  refreshKey: 0,
  refreshing: false,
  scope: 'all' as RefreshScope,

  trigger(scope: RefreshScope = 'all') {
    this.scope = scope;
    this.refreshKey += 1;
  },

  async run<T>(fn: () => Promise<T>, scope: RefreshScope = 'all') {
    if (this.refreshing) return;
    runInAction(() => {
      this.refreshing = true;
      this.scope = scope;
      this.refreshKey += 1;
    });
    try {
      await fn();
    } finally {
      runInAction(() => {
        this.refreshing = false;
      });
    }
  },
});

