import { Route, Routes } from 'react-router-dom';
import HomePage, { type ServerItem } from './Pages/Home/HomePage';
import './Styles/app.scss';
import { observer } from 'mobx-react-lite';
import { useCallback, useEffect, useRef } from 'react';
import { useLoadServers } from './Hooks/useLoadServers';
import Account from './modals/Account/Account';
import Login from './modals/Login/Login';
import Registration from './modals/Registration/Registration';
import ServerPage from './Pages/ServerPage/ServerPage';

const App = observer(() => {
  useLoadServers();
  return (
    <Routes>
      <Route path='/' element={<HomePage />} />
      <Route path='/servers/:id' element={<ServerPage />} />
      <Route path='/account' element={<Account />} />
      <Route path='/login' element={<Login />} />
      <Route path='/register' element={<Registration />} />
    </Routes>
  );
});
export default App;
