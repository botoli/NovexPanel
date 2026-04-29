import { makeAutoObservable } from 'mobx';

export type ToastKind = 'success' | 'error' | 'info';

export type ToastItem = {
  id: string;
  kind: ToastKind;
  title?: string;
  message: string;
  createdAt: number;
};

const DEFAULT_TTL_MS = 4500;

const uid = () => `${Date.now()}-${Math.random().toString(16).slice(2)}`;

export const toastStore = makeAutoObservable({
  items: [] as ToastItem[],

  push(kind: ToastKind, message: string, title?: string, ttlMs: number = DEFAULT_TTL_MS) {
    const id = uid();
    this.items.unshift({ id, kind, title, message, createdAt: Date.now() });
    setTimeout(() => this.remove(id), ttlMs);
    return id;
  },

  remove(id: string) {
    this.items = this.items.filter(t => t.id !== id);
  },

  clear() {
    this.items = [];
  },
});

