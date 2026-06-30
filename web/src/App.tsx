import { useEffect, useState } from 'react';
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { isAuthenticated, clearAuthToken, getAuthStatus } from './api';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Calendar from './pages/Calendar';
import Search from './pages/Search';
import Memories from './pages/Memories';
import DiaryViewer from './pages/DiaryViewer';
import Login from './pages/Login';

function RequireAuth({ children, authEnabled }: { children: React.ReactNode; authEnabled: boolean | null }) {
  if (authEnabled === null) {
    return <div className="loading">加载中...</div>;
  }
  if (authEnabled && !isAuthenticated()) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

function App() {
  const navigate = useNavigate();
  const [authEnabled, setAuthEnabled] = useState<boolean | null>(null);

  useEffect(() => {
    getAuthStatus()
      .then((status) => setAuthEnabled(status.enabled))
      .catch(() => setAuthEnabled(true));
  }, []);

  useEffect(() => {
    function handleAuthRequired() {
      if (authEnabled === false) {
        return;
      }
      clearAuthToken();
      navigate('/login', { replace: true });
    }
    window.addEventListener('auth:required', handleAuthRequired);
    return () => window.removeEventListener('auth:required', handleAuthRequired);
  }, [authEnabled, navigate]);

  const protectedPage = (page: React.ReactNode) => (
    <RequireAuth authEnabled={authEnabled}>{page}</RequireAuth>
  );

  return (
    <Routes>
      <Route path="/login" element={authEnabled === false ? <Navigate to="/dashboard" replace /> : <Login />} />
      <Route path="/" element={<Layout />}>
        <Route index element={protectedPage(<Dashboard />)} />
        <Route path="dashboard" element={protectedPage(<Dashboard />)} />
        <Route path="calendar" element={protectedPage(<Calendar />)} />
        <Route path="search" element={protectedPage(<Search />)} />
        <Route path="memories" element={protectedPage(<Memories />)} />
        <Route path="diary/:date" element={protectedPage(<DiaryViewer />)} />
      </Route>
    </Routes>
  );
}

export default App;
