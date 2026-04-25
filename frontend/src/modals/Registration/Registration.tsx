import { Icon } from '@iconify/react';
import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { API_BASE } from '../../Api/api';
import styles from './Registration.module.scss';
export interface Registration {
  email: string;
  password: string;
}

const Registration = () => {
  const [email, setEmail] = useState<string>('');
  const [password, setPassword] = useState<string>('');
  const [confirmPassword, setConfirmPassword] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const navigate = useNavigate();
  const handleSubmit = async () => {
    // Валидация
    if (password !== confirmPassword) {
      setError('Пароли не совпадают');
      return;
    }
    if (!email || !email.includes('@')) {
      setError('Введите корректный email');
      return;
    }
    if (!password) {
      setError('Введите пароль');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`${API_BASE}/auth/register`, {
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

      navigate('/login');
      console.log('Успех:', data);
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
      <Link to='/account' className={styles.returnBtn}>
        <Icon icon='mdi:arrow-left' className={styles.returnIcon} />
        Account
      </Link>

      <div className={styles.panel}>
        <header className={styles.header}>
          <h1 className={styles.title}>Register</h1>
          <p className={styles.subtitle}>Create your account.</p>
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
              placeholder='Create password'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          <label className={styles.field}>
            <span>Confirm Password</span>
            <input
              type='password'
              placeholder='Confirm password'
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
          </label>

          <button
            type='button'
            className={styles.primaryBtn}
            onClick={handleSubmit}
          >
            Create account
          </button>

          <p className={styles.footerText}>
            Already have an account?
            <Link to='/login'>Sign in</Link>
          </p>
        </div>
      </div>
    </div>
  );
};

export default Registration;
