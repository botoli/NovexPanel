import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { API_BASE } from '../../Api/api';
import AuthBtns from '../../common/AuthBtns/AuthBtns';
import { agentTokenStore } from '../../Store/AgentTokenStore';
import { tokenStore } from '../../Store/TokenStore';
import styles from './Account.module.scss';
export interface TokenData {
  created_at: string;
  expires_at: string;
  id: number;
  last_used_at: string;
  name: string;
  revoked: boolean;
  server: {
    id: number;
    ip: string;
    name: string;
    online: boolean;
  };
  token_prefix: string;
}
const Account = observer(() => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [tokensData, setTokensData] = useState<TokenData[]>([]);
  const getTokensData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/auth/tokens`, {
        headers: {
          Authorization: `Bearer ${tokenStore.token}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      setTokensData(data);
      console.log(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  }, [tokenStore]);
  useEffect(() => {
    getTokensData();
  }, []);
  const deleteToken = async (tokenId: number) => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${API_BASE}/auth/tokens/${tokenId}`, {
        method: 'DELETE',
        headers: {
          Authorization: `Bearer ${tokenStore.getToken()}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
      await getTokensData();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    console.log({ loading, error });
  }, [error]);
  return (
    <div className={styles.contentWrap}>
      <Link to='/' className={styles.returnBtn}>
        <Icon icon='mdi:arrow-left' className={styles.returnIcon} />
        Return
      </Link>
      {tokenStore.getToken()
        ? (
          <div className={styles.authPanel}>
            <header className={styles.header}>
              <h1 className={styles.title}>
                Account <span className={styles.smallText}>Settings</span>
              </h1>
              <p className={styles.subtitle}>Update your profile details and security.</p>
            </header>
            <section className={styles.serversTokensTable}>
              <div className={styles.tableCard}>
                <div className={styles.tableScroll}>
                  <table className={styles.table} aria-label='Servers and tokens'>
                    <thead>
                      <tr>
                        <th>Server</th>
                        <th>Agent token</th>
                        <th>Created</th>
                        <th>Name</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {tokensData.filter(token => !token.revoked).map(token => (
                        <tr key={token.id}>
                          <td>
                            {token.name}
                          </td>
                          <td>
                            {token.token_prefix}***
                          </td>
                          <td>
                            {new Date(token.created_at).toLocaleDateString()}
                          </td>
                          <td>
                            {token.server?.name}
                          </td>
                          <td>
                            {token.server?.online
                              ? <span className={styles.statusOnline}>Online</span>
                              : <span className={styles.statusOffline}>Offline</span>}
                          </td>
                          <td>
                            <button
                              className={styles.deleteBtn}
                              onClick={() => deleteToken(token.id)}
                            >
                              Delete Token
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </section>
            <div className={styles.formWrap}>
              <div className={styles.actions}>
                <button
                  type='button'
                  className={styles.ghostBtn}
                  onClick={() => {
                    tokenStore.clearToken();
                    agentTokenStore.clearAgentToken();
                    navigate('/');
                  }}
                >
                  <Icon
                    icon='mdi:logout'
                    className={styles.btnIcon}
                  />
                  Log out
                </button>
                <button type='button' className={styles.deleteBtn}>
                  <Icon icon='mdi:delete-outline' className={styles.btnIcon} />
                  Delete account
                </button>
              </div>
            </div>
          </div>
        )
        : <AuthBtns />}
    </div>
  );
});

export default Account;
