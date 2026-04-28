import { Icon } from '@iconify/react';
import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { API_BASE } from '../../Api/api';
import styles from './Registration.module.scss';
export interface Registration {
  email: string;
  password: string;
}
export interface ValidateError {
  message: string;
  type: string[] | string;
}
const Registration = () => {
  const [email, setEmail] = useState<string>('');
  const [password, setPassword] = useState<string>('');
  const [confirmPassword, setConfirmPassword] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [validateError, setValidateError] = useState<ValidateError[]>([]);

  useEffect(() => {
    const newErrors: Array<{ message: string; type: string | string[]; }> = [];

    // Валидация email (только если пользователь начал ввод)
    if (email.length > 0) {
      const emailRegex = /^[^\s@]+@([^\s@]+\.)+[^\s@]+$/;
      if (!emailRegex.test(email)) {
        newErrors.push({
          message: 'Введите корректный email (пример: user@domain.com)',
          type: 'email',
        });
      }
    }

    // Валидация пароля (только если пользователь начал ввод)
    if (password.length > 0) {
      if (password.length < 8) {
        newErrors.push({ message: 'Пароль должен быть не менее 8 символов', type: 'password' });
      } else if (password.length > 72) {
        newErrors.push({ message: 'Пароль не должен превышать 72 символа', type: 'password' });
      }
    }

    // Валидация подтверждения пароля (только если пользователь начал ввод в любое из полей)
    if (confirmPassword.length > 0 || password.length > 0) {
      if (confirmPassword.length === 0 && password.length > 0) {
        newErrors.push({ message: 'Подтвердите пароль', type: 'confirmPassword' });
      } else if (
        password !== confirmPassword && password.length > 0 && confirmPassword.length > 0
      ) {
        newErrors.push({ message: 'Пароли не совпадают', type: ['confirmPassword', 'password'] });
      }
    }

    // Обновляем состояние ошибок
    setValidateError(newErrors);
  }, [email, password, confirmPassword]);
  const navigate = useNavigate();
  const handleSubmit = async () => {
    // Валидация

    setValidateError([]);
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
    console.log({ loading });
  }, [error]);
  console.log(validateError);

  // Вспомогательная функция для получения ошибки по типу
  const getErrorByType = (type: string) => {
    return validateError.find(error =>
      Array.isArray(error.type)
        ? error.type.includes(type)
        : error.type === type
    );
  };

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
            {getErrorByType('email') && (
              <p className={styles.error}>{getErrorByType('email')?.message}</p>
            )}
          </label>

          <label className={styles.field}>
            <span>Password</span>
            <input
              type='password'
              placeholder='Create password'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            {getErrorByType('password') && (
              <p className={styles.error}>{getErrorByType('password')?.message}</p>
            )}
          </label>

          <label className={styles.field}>
            <span>Confirm Password</span>
            <input
              type='password'
              placeholder='Confirm password'
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
            {getErrorByType('confirmPassword') && (
              <p className={styles.error}>{getErrorByType('confirmPassword')?.message}</p>
            )}
          </label>

          <button
            type='button'
            className={styles.primaryBtn}
            onClick={handleSubmit}
            disabled={validateError.length > 0 || !email || !password || !confirmPassword}
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
