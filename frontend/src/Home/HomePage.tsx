import { observer } from "mobx-react-lite";
import styles from "./Home.module.scss";
import LeftPanel from "../LeftPanel/LeftPanel";
import {
  AutorenewOutlined,
  CheckCircleOutlined,
  ComputerOutlined,
  DnsOutlined,
  ErrorOutlined,
  KeyboardArrowDown,
  LayersOutlined,
  Menu,
  StorageOutlined,
} from "@mui/icons-material";
import { dashboardMock } from "./Data";

const renderMetricIcon = (icon: string) => {
  switch (icon) {
    case "cpu":
      return <ComputerOutlined fontSize="small" />;
    case "memory":
      return <LayersOutlined fontSize="small" />;
    case "disk":
      return <StorageOutlined fontSize="small" />;
    default:
      return <ComputerOutlined fontSize="small" />;
  }
};

const renderHighlightIcon = (icon: string) => {
  switch (icon) {
    case "containers":
      return <DnsOutlined fontSize="small" />;
    case "deploy":
      return <AutorenewOutlined fontSize="small" />;
    default:
      return <DnsOutlined fontSize="small" />;
  }
};

const renderActivityIcon = (type: string) => {
  switch (type) {
    case "success":
      return <CheckCircleOutlined fontSize="small" />;
    case "error":
      return <ErrorOutlined fontSize="small" />;
    default:
      return <AutorenewOutlined fontSize="small" />;
  }
};

const HomePage = observer(() => {
  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        <div className={styles.contentWrap}>
          <div className={styles.header}>
            <div className={styles.pageTitle}>
              <h1>{dashboardMock.title}</h1>
            </div>
            <button className={styles.ActionBtn}>
              <Menu fontSize="small" />
              Actions
              <KeyboardArrowDown fontSize="small" />
            </button>
          </div>

          <div className={styles.serverRow}>
            <div
              className={
                dashboardMock.server.status === "online"
                  ? styles.online
                  : styles.offline
              }
            ></div>
            <p className={styles.serverName}>{dashboardMock.server.name}</p>
            <span className={styles.serverStatus}>
              {dashboardMock.server.status}
            </span>
          </div>

          <section className={styles.metricsGrid}>
            {dashboardMock.metrics.map((metric) => (
              <article key={metric.id} className={styles.card}>
                <div className={styles.cardHeader}>
                  <div className={styles.cardTitleWrap}>
                    {renderMetricIcon(metric.icon)}
                    <h3>{metric.label}</h3>
                  </div>
                </div>

                <div className={styles.metricValue}>
                  <span>{metric.value}%</span>
                  <p>
                    {metric.used} {metric.unit} / {metric.total} {metric.unit}
                  </p>
                </div>

                <div className={styles.progressTrack}>
                  <div
                    className={styles.progressFill}
                    style={{ width: `${metric.value}%` }}
                  ></div>
                </div>
              </article>
            ))}
          </section>

          <section className={styles.highlightsGrid}>
            {dashboardMock.highlights.map((item) => (
              <article key={item.id} className={styles.smallCard}>
                <div className={styles.cardTitleWrap}>
                  {renderHighlightIcon(item.icon)}
                  <h3>{item.label}</h3>
                </div>
                <p className={styles.smallCardValue}>{item.value}</p>
                <span className={styles.smallCardNote}>{item.note}</span>
              </article>
            ))}
          </section>

          <section className={styles.activitySection}>
            <h2>Recent Activity</h2>
            <div className={styles.activityCard}>
              {dashboardMock.activity.map((event) => (
                <div key={event.id} className={styles.activityRow}>
                  <div className={styles.activityLeft}>
                    <div className={styles.activityIcon}>
                      {renderActivityIcon(event.type)}
                    </div>
                    <p className={styles.activityText}>
                      {event.message} <span>{event.target}</span>
                    </p>
                  </div>
                  <span className={styles.activityTime}>{event.ago}</span>
                </div>
              ))}

              <div className={styles.activityFooter}>
                <button className={styles.viewAllBtn}>View all</button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
});
export default HomePage;
