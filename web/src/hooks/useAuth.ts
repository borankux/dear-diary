import { useCallback, useEffect, useState } from 'react';
import { isAuthenticated, clearAuthToken } from '../api';

export function useAuth() {
  const [auth, setAuth] = useState(isAuthenticated());

  useEffect(() => {
    function check() {
      setAuth(isAuthenticated());
    }
    window.addEventListener('auth:required', check);
    window.addEventListener('storage', check);
    return () => {
      window.removeEventListener('auth:required', check);
      window.removeEventListener('storage', check);
    };
  }, []);

  const logout = useCallback(() => {
    clearAuthToken();
    setAuth(false);
    window.location.reload();
  }, []);

  return { isAuthenticated: auth, logout };
}
