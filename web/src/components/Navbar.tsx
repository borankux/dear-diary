import { NavLink } from 'react-router-dom';
import { Bookmark, CalendarDays, LayoutGrid, LogOut, NotebookPen, Search } from 'lucide-react';
import { clearAuthToken } from '../api';

const NAV_ITEMS = [
  { to: '/', label: '看板', icon: LayoutGrid, end: true },
  { to: '/calendar', label: '日历', icon: CalendarDays },
  { to: '/search', label: '搜索', icon: Search },
  { to: '/memories', label: '记忆', icon: Bookmark },
];

export default function Navbar() {
  function handleLogout() {
    clearAuthToken();
    window.location.reload();
  }

  return (
    <nav className="navbar">
      <div className="nav-brand">
        <span className="nav-icon"><NotebookPen size={19} strokeWidth={2.2} /></span>
        <span className="nav-title">Dear Diary</span>
      </div>
      <div className="nav-links">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon;
          return (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => isActive ? 'active' : ''}>
              <Icon size={16} strokeWidth={2.1} />
              <span>{item.label}</span>
            </NavLink>
          );
        })}
        <button className="nav-logout" onClick={handleLogout}>
          <LogOut size={16} strokeWidth={2.1} />
          <span>退出</span>
        </button>
      </div>
    </nav>
  );
}
