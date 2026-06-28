import { NavLink } from 'react-router-dom';
import { clearAuthToken } from '../api';

export default function Navbar() {
  function handleLogout() {
    clearAuthToken();
    window.location.reload();
  }

  return (
    <nav className="navbar">
      <div className="nav-brand">
        <span className="nav-icon">📝</span>
        <span className="nav-title">Dear Diary</span>
      </div>
      <div className="nav-links">
        <NavLink to="/" end className={({ isActive }) => isActive ? 'active' : ''}>
          看板
        </NavLink>
        <NavLink to="/calendar" className={({ isActive }) => isActive ? 'active' : ''}>
          日历
        </NavLink>
        <NavLink to="/search" className={({ isActive }) => isActive ? 'active' : ''}>
          搜索
        </NavLink>
        <NavLink to="/memories" className={({ isActive }) => isActive ? 'active' : ''}>
          记忆
        </NavLink>
        <button className="nav-logout" onClick={handleLogout}>
          退出
        </button>
      </div>
    </nav>
  );
}
