import { Icon } from '@iconify/react';

import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LogoIcon } from '../../Icons/Icon.tsx';
import styles from './LeftPanel.module.scss';
import { type Tab, Tabs } from './tabs.ts';
export const AppsIcon = () => <Icon icon='icon-park-solid:page-template' fontSize='25' />;
export const ReceiptIcon = () => <Icon icon='radix-icons:activity-log' fontSize='25' />;
export const DockerIcon = () => <Icon icon='mdi:docker' fontSize='25' />;
export const RocketLaunchIcon = () => <Icon icon='tabler:crane' fontSize='25' />;
export const SettingsIcon = () => <Icon icon='mdi:settings' fontSize='25' />;
export const TerminalIcon = () => <Icon icon='mdi:terminal' fontSize='25' />;
const LeftPanel = observer(() => {
  const location = useLocation();
  const currentPath = location.pathname;

  const [tabs, setTabs] = useState(() => {
    const storedTabs = localStorage.getItem('tabs');
    return storedTabs ? JSON.parse(storedTabs) : Tabs;
  });

  const [isMenuOpen, setIsMenuOpen] = useState(false);

  const getIcon = (name: string) => {
    switch (name.toLowerCase()) {
      case 'servers':
        return <Icon icon='mingcute:grid-line' fontSize='25' />;
      case 'deploy':
        return <Icon icon='material-symbols:deployed-code-sharp' fontSize='25' />;
      case 'containers':
        return <Icon icon='boxicons:container' fontSize='25' />;
      case 'terminal':
        return <Icon icon='gravity-ui:terminal' fontSize='25' />;
      case 'logs':
        return <Icon icon='icon-park-outline:upload-logs' fontSize='25' />;
      case 'system':
        return <Icon icon='material-symbols:settings-rounded' fontSize='25' />;
      default:
        return null;
    }
  };

  function toogleActive(name: string) {
    setTabs((prev: Tab[]) => prev?.map((tab) => ({ ...tab, active: tab.name === name })));
    if (window.innerWidth <= 1024) {
      setIsMenuOpen(false);
    }
  }

  useEffect(() => {
    const storedTabs = localStorage.getItem('tabs');
    if (storedTabs) {
      setTabs(JSON.parse(storedTabs));
    }
  }, []);

  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth > 1024) {
        setIsMenuOpen(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  return (
    <>
      <div
        className={`${styles.allheader} ${isMenuOpen ? styles.menuOpen : ''}`}
      >
        <div className={styles.logo_Container}>
          <svg
            width='544'
            height='513'
            viewBox='0 0 544 513'
            fill='none'
            xmlns='http://www.w3.org/2000/svg'
          >
            <rect width='429' height='125' fill='#D9D9D9' fill-opacity='1' />
            <rect y='193' width='429' height='126' fill='#D9D9D9' fill-opacity='1' />
            <rect y='387' width='544' height='126' fill='#D9D9D9' fill-opacity='1' />
            <rect x='429' y='125' width='115' height='194' fill='#D9D9D9' fill-opacity='1' />
          </svg>

          <h1>NOVEX</h1>
        </div>

        <div className={styles.Tabs}>
          {tabs?.map((tab: Tab) => {
            const tabPath = `/${tab.name.toLowerCase()}`;
            const isActive = currentPath === tabPath || currentPath.startsWith(tabPath + '/');

            return (
              <Link key={tab.id} to={tabPath !== '/servers' ? tabPath : '/'}>
                <div
                  className={isActive ? styles.tabactive : styles.tab}
                  onClick={() => toogleActive(tab.name)}
                >
                  {getIcon(tab.name)}
                  <p>{tab.name}</p>
                </div>
              </Link>
            );
          })}
        </div>
        <Link to='/account' className={styles.accountLink}>
          <div className={styles.account}>
            <Icon icon='mdi:account' fontSize='25' />
            Account
          </div>
        </Link>
      </div>
    </>
  );
});
export default LeftPanel;
