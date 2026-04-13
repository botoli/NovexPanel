import { observer } from "mobx-react-lite";
import styles from "./Home.module.scss";
import LeftPanel from "../LeftPanel/LeftPanel";
interface Mocdata {
id:number
serverName:string
serverStatus:string
cpuUsage:string
memoryUsage:string
memoryUsageGB:number
TotalMemoryGB:number
DiskUsage:string
DiskUsageGB:number
DiskTotalGB:number
}
const mockData = [{
  id:0,
  serverName:"Thinkpad x220",
  serverStatus : "online",
  cpuUsage:"30%",
  memoryUsage:"53%",
  memoryUsageGB:4.3,
  TotalMemoryGB:8,
  DiskUsage:"17%",
  DiskUsageGB:35,
  DiskTotalGB:128,


}]
const HomePage = observer(() => {
  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        <div className={styles.hedaer}>
          <div className={styles.pageTitle}>
            <h1>Overview</h1>
          </div>
          <button className={styles.ActionBtn}>Action</button>
        </div>
        <div className={styles.}>{}</div>
      </div>
    </div>
  );
});
export default HomePage;
