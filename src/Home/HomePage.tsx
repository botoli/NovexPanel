import { observer } from "mobx-react-lite";
import styles from "./Home.module.scss";
import LeftPanel from "../LeftPanel/LeftPanel";

const HomePage = observer(() => {
  return (
    <div className={styles.Page}>
      <LeftPanel />
      <div className={styles.mainContent}>
        <h1>Home Page</h1>
      </div>
    </div>
  );
});
export default HomePage;
