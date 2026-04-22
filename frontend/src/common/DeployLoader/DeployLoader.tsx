import { Icon } from '@iconify/react';
import styles from './DeployLoader.module.scss';

export const DeployLoader = ({ label = 'Deploying...' }) => {
  return (
    <div className={styles.loaderWrapper}>
      <div className={styles.loader}>
        {/* Геометрический акцент */}
        <div className={styles.bracketGroup}>
          <span className={styles.bracketOpen}>[</span>
          <div className={styles.sequenceDots}>
            <span className={styles.dot} />
            <span className={styles.dot} />
            <span className={styles.dot} />
          </div>
          <span className={styles.bracketClose}>]</span>
        </div>

        {/* Текстовый статус */}
        <div className={styles.statusText}>
          <span className={styles.label}>{label}</span>
          <span className={styles.ellipsis}>
            <span className={styles.dot1}>.</span>
            <span className={styles.dot2}>.</span>
            <span className={styles.dot3}>.</span>
          </span>
        </div>

        {/* Минималистичный прогресс */}
        <div className={styles.progressTrack}>
          <div className={styles.progressFill} />
        </div>

        {/* Декоративный акцент (опционально) */}
        <Icon icon='mdi:rocket-outline' className={styles.rocketIcon} />
      </div>
    </div>
  );
};
