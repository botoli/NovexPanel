import { Route, Routes } from 'react-router-dom';
import HomePage from './Pages/Home/HomePage';
import './Styles/app.scss';
import Account from './modals/Account/Account';
import Login from './modals/Login/Login';
import Registration from './modals/Registration/Registration';
function App() {
  return (
    <Routes>
      <Route path='/' element={<HomePage />} />
      <Route path='/account' element={<Account />} />
      <Route path='/login' element={<Login />} />
      <Route path='/register' element={<Registration />} />
    </Routes>
  );
}

export default App;
