import { Icon } from '@iconify/react';
import { observer } from 'mobx-react-lite';
import { toastStore } from '../../Store/ToastStore';
import styles from './ToastHost.module.scss';

const toneClass = (kind: string) => {
  if (kind === 'success') return styles.toneSuccess;
  if (kind === 'error') return styles.toneError;
  return styles.toneInfo;
};

const toneIcon = (kind: string) => {
  if (kind === 'success') return 'mdi:check-circle-outline';
  if (kind === 'error') return 'mdi:alert-circle-outline';
  return 'mdi:information-outline';
};

export const ToastHost = observer(() => {
  if (toastStore.items.length === 0) return null;

  return (
    <div className={styles.host} role='region' aria-label='Notifications'>
      {toastStore.items.map(item => (
        <div key={item.id} className={`${styles.toast} ${toneClass(item.kind)}`} role='status'>
          <Icon icon={toneIcon(item.kind)} className={styles.icon} />
          <div className={styles.body}>
            {item.title ? <div className={styles.title}>{item.title}</div> : null}
            <div className={styles.message}>{item.message}</div>
          </div>
          <button
            type='button'
            className={styles.close}
            aria-label='Dismiss notification'
            onClick={() => toastStore.remove(item.id)}
          >
            <Icon icon='mdi:close' />
          </button>
        </div>
      ))}
    </div>
  );
});

