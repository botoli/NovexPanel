import { Route, Routes } from 'react-router-dom';

import { observer } from 'mobx-react-lite';

import { useLoadServers } from './Hooks/useLoadServers';
import HomePage from './Pages/Home/HomePage';
import MetricsPage from './Pages/ServerPage/Metrics/MetricsPage';
import ProcessesPage from './Pages/ServerPage/Processes/ProcessesPage';
import ServerPage from './Pages/ServerPage/ServerPage';
import './Styles/app.scss';
import Account from './modals/Account/Account';
import Login from './modals/Login/Login';
import Registration from './modals/Registration/Registration';
import { DeploymentsPage } from './Pages/ServerPage/Deploy/Deploy';
import { DeployPage } from './Pages/ServerPage/Deploy/DeploymentsPage/DeploymentsPage';
import { TerminalPage } from './Pages/ServerPage/Terminal/Terminal';

const App = observer(() => {
  useLoadServers();

  return (
    <Routes>
      <Route path='/' element={<HomePage />} />

      <Route path='/servers/:id' element={<ServerPage />}>
        <Route path='metrics' element={<MetricsPage />} />
        <Route path='processes' element={<ProcessesPage />} />
        <Route path='terminal' element={<TerminalPage />} />
        <Route path='deploy' element={<DeploymentsPage />} />
        <Route path='deployments' element={<DeployPage />} />
      </Route>

      <Route path='/account' element={<Account />} />
      <Route path='/login' element={<Login />} />
      <Route path='/register' element={<Registration />} />
    </Routes>
  );
});

export default App;
