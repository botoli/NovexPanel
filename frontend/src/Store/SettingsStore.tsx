import { makeAutoObservable, runInAction } from 'mobx';

export type ThemeMode = 'system' | 'dark' | 'light';
export type CursorStyle = 'block' | 'underline' | 'bar';
export type TerminalTheme = 'novex' | 'classic';

type SettingsState = {
  themeMode: ThemeMode;
  compactMode: boolean;
  animations: boolean;
  tableDensity: 'comfortable' | 'compact';
  sidebarCollapsed: boolean;

  terminalFontSize: number;
  terminalCursorStyle: CursorStyle;
  terminalTheme: TerminalTheme;
  terminalScrollback: number;

  notificationsEnabled: boolean;
  deploymentNotifications: boolean;
  errorNotifications: boolean;
};

const STORAGE_KEY = 'novex.settings.v1';

const defaultState = (): SettingsState => ({
  themeMode: 'system',
  compactMode: false,
  animations: true,
  tableDensity: 'comfortable',
  sidebarCollapsed: false,

  terminalFontSize: 14,
  terminalCursorStyle: 'bar',
  terminalTheme: 'novex',
  terminalScrollback: 5000,

  notificationsEnabled: true,
  deploymentNotifications: true,
  errorNotifications: true,
});

const clamp = (n: number, min: number, max: number) => Math.max(min, Math.min(max, n));

export const settingsStore = makeAutoObservable({
  state: defaultState(),
  hydrated: false,

  hydrate() {
    if (this.hydrated) return;
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as Partial<SettingsState>;
        runInAction(() => {
          this.state = { ...defaultState(), ...parsed };
        });
      }
    } catch {
      // ignore corrupt storage
    } finally {
      runInAction(() => {
        this.hydrated = true;
      });
      this.applyAll();
    }
  },

  persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.state));
    } catch {
      // ignore
    }
  },

  setThemeMode(mode: ThemeMode) {
    this.state.themeMode = mode;
    this.persist();
    this.applyTheme();
  },

  setCompactMode(v: boolean) {
    this.state.compactMode = v;
    this.persist();
    this.applyUI();
  },

  setAnimations(v: boolean) {
    this.state.animations = v;
    this.persist();
    this.applyUI();
  },

  setTableDensity(v: 'comfortable' | 'compact') {
    this.state.tableDensity = v;
    this.persist();
    this.applyUI();
  },

  setSidebarCollapsed(v: boolean) {
    this.state.sidebarCollapsed = v;
    this.persist();
  },

  setTerminalFontSize(v: number) {
    this.state.terminalFontSize = clamp(Math.round(v), 10, 22);
    this.persist();
  },

  setTerminalCursorStyle(v: CursorStyle) {
    this.state.terminalCursorStyle = v;
    this.persist();
  },

  setTerminalTheme(v: TerminalTheme) {
    this.state.terminalTheme = v;
    this.persist();
  },

  setTerminalScrollback(v: number) {
    this.state.terminalScrollback = clamp(Math.round(v), 500, 50000);
    this.persist();
  },

  setNotificationsEnabled(v: boolean) {
    this.state.notificationsEnabled = v;
    this.persist();
  },
  setDeploymentNotifications(v: boolean) {
    this.state.deploymentNotifications = v;
    this.persist();
  },
  setErrorNotifications(v: boolean) {
    this.state.errorNotifications = v;
    this.persist();
  },

  applyAll() {
    this.applyTheme();
    this.applyUI();
  },

  applyTheme() {
    const body = document.body;
    body.classList.remove('theme-light', 'theme-dark');
    const mode = this.state.themeMode;
    const systemPrefersLight = window.matchMedia?.('(prefers-color-scheme: light)')?.matches;
    const effective = mode === 'system' ? (systemPrefersLight ? 'light' : 'dark') : mode;
    body.classList.add(effective === 'light' ? 'theme-light' : 'theme-dark');
  },

  applyUI() {
    const body = document.body;
    body.classList.toggle('ui-compact', this.state.compactMode);
    body.classList.toggle('ui-no-animations', !this.state.animations);
    body.classList.toggle('ui-density-compact', this.state.tableDensity === 'compact');
  },
});

