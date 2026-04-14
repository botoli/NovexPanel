import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { Link, useNavigate } from 'react-router-dom';
import { agentTokenStore } from '../../Store/AgentTokenStore';
import { tokenStore } from '../../Store/TokenStore';
import styles from './Account.module.scss';

const Account = observer(() => {
  const navigate = useNavigate();
  return (
    <div className={styles.contentWrap}>
      <Link to='/' className={styles.returnBtn}>
        <Icon icon='mdi:arrow-left' className={styles.returnIcon} />
        Return
      </Link>
      {tokenStore.token
        ? (
          <div className={styles.authPanel}>
            <header className={styles.header}>
              <h1 className={styles.title}>
                Account <span className={styles.smallText}>Settings</span>
              </h1>
              <p className={styles.subtitle}>Update your profile details and security.</p>
            </header>

            <div className={styles.formWrap}>
              <section className={styles.section}>
                <div className={styles.sectionLabel}>
                  <Icon icon='mdi:email-outline' className={styles.icon} />
                  <h2>Email</h2>
                </div>
                <input type='email' placeholder='name@example.com' />
              </section>

              <section className={styles.section}>
                <div className={styles.sectionLabel}>
                  <Icon icon='solar:password-bold' className={styles.icon} />
                  <h2>Password</h2>
                </div>
                <input type='password' placeholder='New password' />
              </section>

              <section className={styles.section}>
                <div className={styles.sectionLabel}>
                  <Icon icon='mdi:account-outline' className={styles.icon} />
                  <h2>Username</h2>
                </div>
                <input type='text' placeholder='Username' />
              </section>

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
        : (
          <div className={styles.authButtons}>
            <Link to='/login' className={styles.loginBtn}>
              <span className={styles.authButtonText}>
                <span>Login</span>
                <span className={styles.authButtonHint}>
                  Sign in to continue with your profile.
                </span>
              </span>
              <Icon icon='mdi:login' className={styles.loginIcon} />
            </Link>
            <Link to='/register' className={styles.registerBtn}>
              <span className={styles.authButtonText}>
                <span>Register</span>
                <span className={styles.authButtonHint}>Create a new account in a few steps.</span>
              </span>
              <Icon icon='mdi:account-plus' className={styles.registerIcon} />
            </Link>
          </div>
        )}
    </div>
  );
});

export default Account;
