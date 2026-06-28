import { useEffect } from 'react';
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { isAuthenticated } from './api';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Calendar from './pages/Calendar';
import Search from './pages/Search';
import Memories from './pages/Memories';
import DiaryViewer from './pages/DiaryViewer';
import Login from './pages/Login';

function RequireAuth({ children }: { children: React.ReactNode }) {
  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

function App() {
  const navigate = useNavigate();

  useEffect(() => {
    function handleAuthRequired() {
      navigate('/login', { replace: true });
    }
    window.addEventListener('auth:required', handleAuthRequired);
    return () => window.removeEventListener('auth:required', handleAuthRequired);
  }, [navigate]);

  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<RequireAuth><Dashboard /></RequireAuth>} />
        <Route path="dashboard" element={<RequireAuth><Dashboard /></RequireAuth>} />
        <Route path="calendar" element={<RequireAuth><Calendar /></RequireAuth>} />
        <Route path="search" element={<RequireAuth><Search /></RequireAuth>} />
        <Route path="memories" element={<RequireAuth><Memories /></RequireAuth>} />
        <Route path="diary/:date" element={<RequireAuth><DiaryViewer /></RequireAuth>} />
      </Route>
    </Routes>
  );
}

export default App;
