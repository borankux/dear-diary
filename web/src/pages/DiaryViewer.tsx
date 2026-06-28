import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getDiary, type DiaryEntry } from '../api';

export default function DiaryViewer() {
  const { date } = useParams<{ date: string }>();
  const [entry, setEntry] = useState<DiaryEntry | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!date) return;
    let cancelled = false;
    async function load() {
      try {
        setLoading(true);
        setError('');
        const data = await getDiary(date!);
        if (!cancelled) setEntry(data);
      } catch (e: unknown) {
        if (!cancelled) setError(e instanceof Error ? e.message : '加载失败');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, [date]);

  const prevDate = (d: string) => {
    const t = new Date(d);
    t.setDate(t.getDate() - 1);
    return t.toISOString().slice(0, 10);
  };
  const nextDate = (d: string) => {
    const t = new Date(d);
    t.setDate(t.getDate() + 1);
    return t.toISOString().slice(0, 10);
  };

  if (loading) return <div className="loading">加载日记...</div>;
  if (error) return <div className="error">{error} <Link to="/calendar">返回日历</Link></div>;
  if (!entry) return <div className="error">日记未找到</div>;

  return (
    <div className="diary-viewer">
      <div className="diary-header">
        <h2>{entry.date}</h2>
        <div className="diary-nav">
          <Link to={`/diary/${prevDate(entry.date)}`}>◀ 前一天</Link>
          <Link to={`/diary/${nextDate(entry.date)}`} style={{ marginLeft: 8 }}>后一天 ▶</Link>
        </div>
      </div>
      <div className="diary-content" dangerouslySetInnerHTML={{ __html: entry.html }} />
      <div style={{ marginTop: 24, display: 'flex', justifyContent: 'center', gap: 12 }}>
        <Link to="/calendar" style={{ padding: '10px 20px', border: 'var(--hairline)', borderRadius: 'var(--radius)', background: 'var(--paper)', textDecoration: 'none', color: 'var(--text)', fontWeight: 700 }}>
          返回日历
        </Link>
      </div>
    </div>
  );
}
