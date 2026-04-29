import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import LeftPanel from '../LeftPanel/LeftPanel';
import { settingsStore, type ThemeMode } from '../../Store/SettingsStore';
import { workspaceSettingsStore } from '../../Store/WorkspaceSettingsStore';
import { githubStore } from '../../Store/GitHubStore';
import { apiRequest } from '../../Api/client';
import styles from './SettingsPage.module.scss';

type TabKey = 'workspace' | 'github' | 'tokens' | 'preferences';

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
  const [tab, setTab] = useState<TabKey>('workspace');
  const s = settingsStore.state;
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [memberEmail, setMemberEmail] = useState('');
  const [memberRole, setMemberRole] = useState('developer');
  const [tokenName, setTokenName] = useState('');

  useEffect(() => {
    void workspaceSettingsStore.loadMembers();
    void workspaceSettingsStore.loadApiTokens();
    void githubStore.loadConnection();
  }, []);

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
              <button type='button' className={`${styles.tab} ${tab === 'workspace' ? styles.tabActive : ''}`} onClick={() => setTab('workspace')}>
                <Icon icon='mdi:account-group-outline' /> Workspace
              </button>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'github' ? styles.tabActive : ''}`}
                onClick={() => setTab('github')}
              >
                <Icon icon='mdi:github' />
                GitHub
              </button>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'tokens' ? styles.tabActive : ''}`}
                onClick={() => setTab('tokens')}
              >
                <Icon icon='mdi:key-outline' />
                API Tokens
              </button>
              <button
                type='button'
                className={`${styles.tab} ${tab === 'preferences' ? styles.tabActive : ''}`}
                onClick={() => setTab('preferences')}
              >
                <Icon icon='mdi:tune' />
                Preferences
              </button>
            </div>
          </header>

          <section className={styles.card}>
            {tab === 'workspace'
              ? (
                <>
                  <h2 className={styles.cardTitle}>Workspace & Access</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <input className={styles.input} value={email} onChange={(e) => setEmail(e.target.value)} placeholder='New email' />
                      <input className={styles.input} value={password} onChange={(e) => setPassword(e.target.value)} placeholder='New password' />
                      <button type='button' onClick={() => apiRequest('/auth/me', { method: 'PATCH', body: JSON.stringify({ email: email || undefined, new_password: password || undefined }) })}>Update profile</button>
                    </div>
                    <div className={styles.row}>
                      <input className={styles.input} value={memberEmail} onChange={(e) => setMemberEmail(e.target.value)} placeholder='Member email' />
                      <select className={styles.select} value={memberRole} onChange={(e) => setMemberRole(e.target.value)}>
                        <option value='viewer'>Viewer</option>
                        <option value='developer'>Developer</option>
                        <option value='admin'>Admin</option>
                      </select>
                      <button type='button' onClick={() => workspaceSettingsStore.addMember(memberEmail, memberRole)}>Invite</button>
                    </div>
                    {workspaceSettingsStore.members.map((m) => <div className={styles.row} key={m.id}><div>{m.email}</div><div>{m.role}</div><button type='button' onClick={() => workspaceSettingsStore.removeMember(m.id)}>Remove</button></div>)}
                  </div>
                </>
              )
              : null}

            {tab === 'github'
              ? (
                <>
                  <h2 className={styles.cardTitle}>GitHub</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      {githubStore.connection.connected ? <div>Connected as @{githubStore.connection.login}</div> : <div>Not connected</div>}
                      <button type='button' onClick={() => githubStore.connect()}>Connect</button>
                      <button type='button' onClick={() => githubStore.loadRepos()}>Sync repos</button>
                    </div>
                    {githubStore.repos.map((repo) => (
                      <div className={styles.row} key={repo.id}>
                        <div>{repo.full_name}</div>
                        <div>{repo.default_branch}</div>
                        <a href={repo.html_url} target='_blank' rel='noreferrer'>Open</a>
                      </div>
                    ))}
                    {githubStore.connection.connected ? (
                      <div className={styles.row}>
                        <button type='button' onClick={() => githubStore.disconnect()}>Disconnect GitHub</button>
                      </div>
                    ) : null}
                  </div>
                </>
              )
              : null}

            {tab === 'tokens'
              ? (
                <>
                  <h2 className={styles.cardTitle}>CI/CD Tokens</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <input className={styles.input} value={tokenName} onChange={(e) => setTokenName(e.target.value)} placeholder='Token name' />
                      <button type='button' onClick={() => workspaceSettingsStore.createApiToken(tokenName)}>Create token</button>
                      {workspaceSettingsStore.revealToken ? <code>{workspaceSettingsStore.revealToken}</code> : null}
                    </div>
                    {workspaceSettingsStore.apiTokens.map((token) => (
                      <div className={styles.row} key={token.id}>
                        <div>{token.name}</div>
                        <div>{token.token_prefix}</div>
                        <div>{token.revoked ? 'revoked' : 'active'}</div>
                      </div>
                    ))}
                  </div>
                </>
              )
              : null}

            {tab === 'preferences'
              ? (
                <>
                  <h2 className={styles.cardTitle}>Preferences</h2>
                  <div className={styles.grid}>
                    <div className={styles.row}>
                      <div className={styles.label}>Theme mode</div>
                      <select className={styles.select} value={s.themeMode} onChange={(e) => settingsStore.setThemeMode(e.target.value as ThemeMode)}>
                        <option value='system'>System</option>
                        <option value='dark'>Dark</option>
                        <option value='light'>Light</option>
                      </select>
                    </div>
                    <div className={styles.row}>
                      <div className={styles.label}>Compact mode</div>
                      <Toggle label='toggle compact' value={s.compactMode} onChange={settingsStore.setCompactMode.bind(settingsStore)} />
                    </div>
                    <div className={styles.row}>
                      <div className={styles.label}>Notifications</div>
                      <Toggle label='toggle notifications' value={s.notificationsEnabled} onChange={settingsStore.setNotificationsEnabled.bind(settingsStore)} />
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

