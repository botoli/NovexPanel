import { Route, Routes } from 'react-router-dom';

import { observer } from 'mobx-react-lite';

import { useLoadServers } from './Hooks/useLoadServers';
import HomePage from './Pages/Home/HomePage';
import MetricsPage from './Pages/ServerPage/Metrics/MetricsPage';
import ProcessesPage from './Pages/ServerPage/Processes/ProcessesPage';
import ServerPage from './Pages/ServerPage/ServerPage';
import './Styles/app.scss';
import { ToastHost } from './common/Toast/ToastHost';
import Account from './modals/Account/Account';
import Login from './modals/Login/Login';
import Registration from './modals/Registration/Registration';
import DomainsPage from './Pages/Domains/DomainsPage';
import NetworkPage from './Pages/Network/NetworkPage';
import { DeploymentDetailPage } from './Pages/ServerPage/Deploy/DeploymentDetailPage/DeploymentDetailPage';
import { DeploymentsPage } from './Pages/ServerPage/Deploy/DeploymentsPage';
import { DeployPage } from './Pages/ServerPage/Deploy/DeploymentsPage/Deploy';
import FilesPage from './Pages/ServerPage/Files/FilesPage';
import RunbookDetailPage from './Pages/ServerPage/Runbooks/RunbookDetailPage';
import RunbooksPage from './Pages/ServerPage/Runbooks/RunbooksPage';
import SecretsPage from './Pages/ServerPage/Secrets/SecretsPage';
import ServicesPage from './Pages/ServerPage/Services/ServicesPage';
import { TerminalPage } from './Pages/ServerPage/Terminal/Terminal';
import SettingsPage from './Pages/Settings/SettingsPage';
import { settingsStore } from './Store/SettingsStore';

const App = observer(() => {
  useLoadServers();
  settingsStore.hydrate();

  return (
    <>
      <ToastHost />
      <Routes>
        <Route path='/' element={<HomePage />} />
        <Route path='/network' element={<NetworkPage />} />
        <Route path='/domains' element={<DomainsPage />} />
        <Route path='/settings' element={<SettingsPage />} />
        <Route path='/secrets' element={<SecretsPage />} />

        <Route path='/servers/:id' element={<ServerPage />}>
          <Route path='metrics' element={<MetricsPage />} />
          <Route path='processes' element={<ProcessesPage />} />
          <Route path='terminal' element={<TerminalPage />} />
          <Route path='deployments' element={<DeploymentsPage />} />
          <Route path='deployments/:deployId' element={<DeploymentDetailPage />} />
          <Route path='deploy' element={<DeployPage />} />
          <Route path='runbooks' element={<RunbooksPage />} />
          <Route path='runbooks/:runbookId' element={<RunbookDetailPage />} />
          <Route path='services' element={<ServicesPage />} />
          <Route path='files' element={<FilesPage />} />
          <Route path='secrets' element={<SecretsPage />} />
        </Route>

        <Route path='/account' element={<Account />} />
        <Route path='/login' element={<Login />} />
        <Route path='/register' element={<Registration />} />
      </Routes>
    </>
  );
});

export default App;
