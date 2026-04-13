import { useEffect, useState } from "react";
import styles from "./LeftPanel.module.scss";
import { Tabs, type Tab } from "./tabs.ts";
import { Link, useLocation } from "react-router-dom";
import { observer } from "mobx-react-lite";
import {
  AIIcon,
  CloseIcon,
  CodeIcon,
  ContainersIcon,
  HomeIcon,
  LogoIcon,
  MenuIcon,
  ProjectsIcon,
  SettingsIcon,
  TasksIcon,
} from "../Icons/Icon.tsx";
import "@mui/icons-material";
import {
  Apps,
  RocketLaunch,
  Terminal,
  Receipt,
  Settings,
} from "@mui/icons-material";
const LeftPanel = observer(() => {
  const location = useLocation();
  const currentPath = location.pathname;

  const [tabs, setTabs] = useState(() => {
    const storedTabs = localStorage.getItem("tabs");
    return storedTabs ? JSON.parse(storedTabs) : Tabs;
  });

  const [isMenuOpen, setIsMenuOpen] = useState(false);

  const getIcon = (name: string) => {
    switch (name.toLowerCase()) {
      case "overview":
        return <Apps />;
      case "deploy":
        return <RocketLaunch />;
      case "containers":
        return <ContainersIcon />;
      case "terminal":
        return <Terminal />;
      case "logs":
        return <Receipt />;
      case "system":
        return <Settings />;
      default:
        return null;
    }
  };

  function toogleActive(name: string) {
    setTabs((prev: Tab[]) =>
      prev?.map((tab) => ({ ...tab, active: tab.name === name })),
    );
    if (window.innerWidth <= 1024) {
      setIsMenuOpen(false);
    }
  }

  useEffect(() => {
    const storedTabs = localStorage.getItem("tabs");
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

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  return (
    <>
      <div
        className={`${styles.allheader} ${isMenuOpen ? styles.menuOpen : ""}`}
      >
        <div className={styles.logo_Container}>
          <LogoIcon />
          <h1>NOVEX</h1>
        </div>

        <div className={styles.Tabs}>
          {tabs?.map((tab: Tab) => {
            const tabPath = `/${tab.name.toLowerCase()}`;
            const isActive =
              currentPath === tabPath || currentPath.startsWith(tabPath + "/");

            return (
              <Link key={tab.id} to={tabPath !== "/overview" ? tabPath : "/"}>
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
      </div>
    </>
  );
});
export default LeftPanel;
