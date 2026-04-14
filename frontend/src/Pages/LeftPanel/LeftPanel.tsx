import { Icon } from '@iconify/react';

import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LogoIcon } from '../../Icons/Icon.tsx';
import { agentTokenStore } from '../../Store/AgentTokenStore.tsx';
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
      case 'overview':
        return <AppsIcon />;
      case 'deploy':
        return <RocketLaunchIcon />;
      case 'containers':
        return <DockerIcon />;
      case 'terminal':
        return <TerminalIcon />;
      case 'logs':
        return <ReceiptIcon />;
      case 'system':
        return <SettingsIcon />;
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
          <LogoIcon />
          <h1>NOVEX</h1>
        </div>

        <div className={styles.Tabs}>
          {tabs?.map((tab: Tab) => {
            const tabPath = `/${tab.name.toLowerCase()}`;
            const isActive = currentPath === tabPath || currentPath.startsWith(tabPath + '/');

            return (
              <Link key={tab.id} to={tabPath !== '/overview' ? tabPath : '/'}>
                <div
                  className={isActive ? styles.activeTab : styles.tab}
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
