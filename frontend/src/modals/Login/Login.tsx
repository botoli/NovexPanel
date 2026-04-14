import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { tokenStore } from '../../Store/TokenStore';
import styles from './Login.module.scss';

const Login = observer(() => {
  const [email, setEmail] = useState<string>('');
  const [password, setPassword] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const navigate = useNavigate();
  const handleSubmit = async () => {
    // Валидация
    if (!email || !email.includes('@') || !email.includes('.')) {
      setError('Введите корректный email');
      return;
    }
    if (!password) {
      setError('Введите пароль');
      return;
    }
    if (password.length < 6) {
      setError('Пароль должен быть не менее 6 символов');
    }
    setLoading(true);
    setError(null);

    try {
      const response = await fetch('http://localhost:8380/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }), // ← отправляем напрямую
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || `HTTP ${response.status}`);
      }

      const data = await response.json();
      tokenStore.setToken(data.token);
      navigate('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.contentWrap}>
      <Link to='/' className={styles.returnBtn}>
        <Icon icon='mdi:arrow-left' className={styles.returnIcon} />
        Return
      </Link>

      <div className={styles.panel}>
        <header className={styles.header}>
          <h1 className={styles.title}>Login</h1>
          <p className={styles.subtitle}>Access your profile.</p>
        </header>

        <div className={styles.formWrap}>
          <label className={styles.field}>
            <span>Email</span>
            <input
              type='email'
              placeholder='name@example.com'
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>

          <label className={styles.field}>
            <span>Password</span>
            <input
              type='password'
              placeholder='Your password'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>

          <button type='button' className={styles.primaryBtn} onClick={handleSubmit}>
            Continue
          </button>

          <p className={styles.footerText}>
            No account yet?
            <Link to='/register'>Create one</Link>
          </p>
        </div>
      </div>
    </div>
  );
});

export default Login;
