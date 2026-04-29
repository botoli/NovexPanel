import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useMemo, useState } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { settingsStore, type ThemeMode } from '../../Store/SettingsStore';
import styles from './SettingsPage.module.scss';

type TabKey = 'appearance' | 'ui' | 'terminal' | 'notifications';

const Toggle = ({ value, onChange, label }: { value: boolean; onChange: (v: boolean) => void; label: string }) => (
  <button
    type='button'
    aria-label={label}
    className={`${styles.toggle} ${value ? styles.toggleOn : ''}`}
    onClick={() => onChange(!value)}
  >
    <span className={`${styles.toggleKnob} ${value ? styles.toggleKnobOn : ''}`} />
  </button>
);

const SettingsPage = observer(() => {
  const [tab, setTab] = useState<TabKey>('appearance');
  const s = settingsStore.state;

  const tabTitle = useMemo(() => {
    if (tab === 'appearance') return 'Appearance';
    if (tab === 'ui') return 'UI';
    if (tab === 'terminal') return 'Terminal';
    return 'Notifications';
  }, [tab]);

  return (
    <div className={styles.page}>
      <LeftPanel />
      <main className={styles.main}>
        <div className={styles.wrap}>
          <header className={styles.header}>
            <div>
              <h1 className={styles.title}>
                <Icon icon='mdi:cog-outline' />
                Settings
              </h1>
              <p className={styles.subtitle}>Personalize NovexPanel. Changes apply immediately.</p>
            </div>
            <div className={styles.tabs} aria-label='Settings sections'>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'appearance' ? styles.tabActive : ''}`}
                onClick={() => setTab('appearance')}
              >
                <Icon icon='mdi:palette-outline' />
                Appearance
              </button>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'ui' ? styles.tabActive : ''}`}
                onClick={() => setTab('ui')}
              >
                <Icon icon='mdi:view-dashboard-outline' />
                UI
              </button>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'terminal' ? styles.tabActive : ''}`}
                onClick={() => setTab('terminal')}
              >
                <Icon icon='mdi:terminal' />
                Terminal
              </button>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'notifications' ? styles.tabActive : ''}`}
                onClick={() => setTab('notifications')}
              >
                <Icon icon='mdi:bell-outline' />
                Notifications
              </button>
            </div>
          </header>

          <section className={styles.card} aria-label={tabTitle}>
            {tab === 'appearance'
              ? (
                <>
                  <h2 className={styles.cardTitle}>Theme</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Theme mode</div>
                        <div className={styles.hint}>Dark, Light (inverted), or follow system.</div>
                      </div>
                      <select
                        className={styles.select}
                        value={s.themeMode}
                        onChange={(e) => settingsStore.setThemeMode(e.target.value as ThemeMode)}
                      >
                        <option value='system'>System</option>
                        <option value='dark'>Dark</option>
                        <option value='light'>Light</option>
                      </select>
                    </div>
                  </div>
                </>
              )
              : null}

            {tab === 'ui'
              ? (
                <>
                  <h2 className={styles.cardTitle}>UI preferences</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Compact mode</div>
                        <div className={styles.hint}>Reduces whitespace in dense screens.</div>
                      </div>
                      <Toggle
                        label='Toggle compact mode'
                        value={s.compactMode}
                        onChange={settingsStore.setCompactMode.bind(settingsStore)}
                      />
                    </div>

                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Animations</div>
                        <div className={styles.hint}>Disable to reduce motion.</div>
                      </div>
                      <Toggle
                        label='Toggle animations'
                        value={s.animations}
                        onChange={settingsStore.setAnimations.bind(settingsStore)}
                      />
                    </div>

                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Table density</div>
                        <div className={styles.hint}>Controls row padding for tables.</div>
                      </div>
                      <select
                        className={styles.select}
                        value={s.tableDensity}
                        onChange={(e) => settingsStore.setTableDensity(e.target.value as any)}
                      >
                        <option value='comfortable'>Comfortable</option>
                        <option value='compact'>Compact</option>
                      </select>
                    </div>

                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Sidebar collapsed</div>
                        <div className={styles.hint}>Collapse left navigation.</div>
                      </div>
                      <Toggle
                        label='Toggle sidebar collapsed'
                        value={s.sidebarCollapsed}
                        onChange={settingsStore.setSidebarCollapsed.bind(settingsStore)}
                      />
                    </div>
                  </div>
                </>
              )
              : null}

            {tab === 'terminal'
              ? (
                <>
                  <h2 className={styles.cardTitle}>Terminal</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Font size</div>
                        <div className={styles.hint}>10–22 px.</div>
                      </div>
                      <input
                        className={styles.input}
                        type='number'
                        min={10}
                        max={22}
                        value={s.terminalFontSize}
                        onChange={(e) => settingsStore.setTerminalFontSize(Number(e.target.value))}
                      />
                    </div>

                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Cursor style</div>
                        <div className={styles.hint}>Block / underline / bar.</div>
                      </div>
                      <select
                        className={styles.select}
                        value={s.terminalCursorStyle}
                        onChange={(e) => settingsStore.setTerminalCursorStyle(e.target.value as any)}
                      >
                        <option value='bar'>Bar</option>
                        <option value='block'>Block</option>
                        <option value='underline'>Underline</option>
                      </select>
                    </div>

                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Theme</div>
                        <div className={styles.hint}>Applies to terminal only.</div>
                      </div>
                      <select
                        className={styles.select}
                        value={s.terminalTheme}
                        onChange={(e) => settingsStore.setTerminalTheme(e.target.value as any)}
                      >
                        <option value='novex'>Novex</option>
                        <option value='classic'>Classic</option>
                      </select>
                    </div>

                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Scrollback</div>
                        <div className={styles.hint}>How many lines to keep in memory.</div>
                      </div>
                      <input
                        className={styles.input}
                        type='number'
                        min={500}
                        max={50000}
                        value={s.terminalScrollback}
                        onChange={(e) => settingsStore.setTerminalScrollback(Number(e.target.value))}
                      />
                    </div>
                  </div>
                </>
              )
              : null}

            {tab === 'notifications'
              ? (
                <>
                  <h2 className={styles.cardTitle}>Notifications</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Enable notifications</div>
                        <div className={styles.hint}>Controls all notifications in the UI.</div>
                      </div>
                      <Toggle
                        label='Toggle notifications'
                        value={s.notificationsEnabled}
                        onChange={settingsStore.setNotificationsEnabled.bind(settingsStore)}
                      />
                    </div>
                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Deployment notifications</div>
                        <div className={styles.hint}>Show deploy success/failure notifications.</div>
                      </div>
                      <Toggle
                        label='Toggle deployment notifications'
                        value={s.deploymentNotifications}
                        onChange={settingsStore.setDeploymentNotifications.bind(settingsStore)}
                      />
                    </div>
                    <div className={styles.row}>
                      <div className={styles.rowText}>
                        <div className={styles.label}>Error notifications</div>
                        <div className={styles.hint}>Show error toasts.</div>
                      </div>
                      <Toggle
                        label='Toggle error notifications'
                        value={s.errorNotifications}
                        onChange={settingsStore.setErrorNotifications.bind(settingsStore)}
                      />
                    </div>
                  </div>
                </>
              )
              : null}
          </section>
        </div>
      </main>
    </div>
  );
});

export default SettingsPage;

