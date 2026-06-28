import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getCalendar, type CalendarMonth } from '../api';

const WEEKDAYS = ['一', '二', '三', '四', '五', '六', '日'];

export default function Calendar() {
  const [now, setNow] = useState(new Date());
  const [months, setMonths] = useState<CalendarMonth[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const navigate = useNavigate();

  const year = now.getFullYear();
  const month = now.getMonth() + 1;

  async function load() {
    try {
      setLoading(true);
      const data = await getCalendar();
      setMonths(data);
      setError('');
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  const currentMonthData = months.find((m) => m.month === `${year}-${String(month).padStart(2, '0')}`);

  const todayStr = new Date().toISOString().slice(0, 10);

  return (
    <div>
      <div className="page-header">
        <h1>日历</h1>
        <div className="meta">{months.length} 个月</div>
      </div>

      <div className="calendar-nav">
        <button onClick={() => setNow((p) => new Date(p.getFullYear(), p.getMonth() - 1, 1))}>
          ◀ 上个月
        </button>
        <h2>{year} 年 {month} 月</h2>
        <button onClick={() => setNow((p) => new Date(p.getFullYear(), p.getMonth() + 1, 1))}>
          下个月 ▶
        </button>
      </div>

      {loading && <div className="loading">加载中...</div>}
      {error && <div className="error">{error}</div>}

      {!loading && !error && currentMonthData && (
        <div className="card" style={{ padding: 20 }}>
          <div className="calendar-weekdays">
            {WEEKDAYS.map((w) => (
              <div key={w}>{w}</div>
            ))}
          </div>
          <div className="calendar-grid">
            {currentMonthData.days.map((day, i) => (
              <div
                key={i}
                className={`calendar-cell ${day.isPadding ? 'is-padding' : ''} ${day.isToday ? 'is-today' : ''} ${day.isWritten ? 'is-written' : ''}`}
                onClick={() => !day.isPadding && navigate(`/diary/${day.date}`)}
              >
                <span className="day-num">{day.isPadding ? '' : day.day}</span>
                {day.isWritten && <span className="dot" />}
              </div>
            ))}
          </div>
          <div style={{ marginTop: 12, fontSize: '0.8rem', color: 'var(--muted)' }}>
            点击有日记的日期查看内容。今天: {todayStr}
          </div>
        </div>
      )}

      {!loading && !error && !currentMonthData && (
        <div className="card">
          <div className="empty">该月没有日记数据。</div>
        </div>
      )}

      {/* All months list */}
      <section style={{ marginTop: 40 }}>
        <h2 style={{ fontSize: '1.05rem', fontWeight: 900, marginBottom: 14 }}>所有月份</h2>
        <div className="board-grid" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))' }}>
          {months.map((m) => (
            <div
              className="card"
              key={m.month}
              style={{ cursor: 'pointer' }}
              onClick={() => {
                const [y, mo] = m.month.split('-').map(Number);
                setNow(new Date(y, mo - 1, 1));
              }}
            >
              <strong>{m.title}</strong>
              <p style={{ color: 'var(--muted)', fontSize: '0.85rem', margin: '4px 0 0' }}>{m.count} 篇日记</p>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
