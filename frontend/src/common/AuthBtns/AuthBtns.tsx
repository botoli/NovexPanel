import { Icon } from '@iconify/react';
import { Link } from 'react-router-dom';
import styles from './AuthBtns.module.scss';

const AuthBtns = () => {
  return (
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
  );
};
export default AuthBtns;
