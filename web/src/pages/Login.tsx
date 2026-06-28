import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { login, setAuthToken, isAuthenticated } from '../api';

export default function LoginPage() {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    if (isAuthenticated()) {
      navigate('/dashboard', { replace: true });
    }
  }, [navigate]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setIsSubmitting(true);
    try {
      const res = await login(password);
      setAuthToken(res.token);
      navigate('/dashboard', { replace: true });
    } catch {
      setError('密码错误');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">亲爱的日记</h1>
        <form onSubmit={handleSubmit} className="login-form">
          <input
            type="password"
            placeholder="请输入密码"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="login-input"
            autoFocus
          />
          {error && <div className="login-error">{error}</div>}
          <button type="submit" className="login-button" disabled={isSubmitting}>
            {isSubmitting ? '登录中...' : '登录'}
          </button>
        </form>
      </div>
    </div>
  );
}
