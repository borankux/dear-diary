import { useEffect, useState, useRef } from 'react';
import { getStats, getTodos, getCandidates, getDiaries, acceptCandidate, rejectCandidate, updateTodoStatus, clearAuthToken, type Stats, type Todo, type Candidate } from '../api';

const BOARD_COLUMNS = [
  { key: 'inbox', title: 'AI Inbox', desc: 'AI 提取出的建议，只在你提升后才进入可信 todo/memory', color: 'board-inbox' },
  { key: 'active', title: 'Active', desc: '已经提升，尚未开始或未分类的真实 todo', color: 'board-active' },
  { key: 'in-progress', title: 'In Progress', desc: '正在做，应该优先保持可见', color: 'board-in-progress' },
  { key: 'done', title: 'Done', desc: '最近完成的 todo，用来看到闭环', color: 'board-done' },
  { key: 'wont-do', title: "Won't Do", desc: '明确不打算做，避免继续占注意力', color: 'board-wont-do' },
  { key: 'archived', title: 'Archived', desc: '已收起但不算完成的 todo', color: 'board-archived' },
  { key: 'other', title: 'Other', desc: 'AI 或人工暂时无法归类的 todo', color: 'board-other' },
] as const;

const INBOX_PREVIEW_LIMIT = 24;

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [todos, setTodos] = useState<Todo[]>([]);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [sseConnected, setSseConnected] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  async function loadAll() {
    try {
      setLoading(true);
      const [s, t, c] = await Promise.all([getStats(), getTodos(), getCandidates()]);
      setStats(s);
      setTodos(t);
      setCandidates(c);
      setError('');
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { loadAll(); }, []);

  useEffect(() => {
    const es = new EventSource('/api/events');
    esRef.current = es;

    es.onopen = () => setSseConnected(true);
    es.onerror = () => setSseConnected(false);

    es.addEventListener('process_complete', () => {
      getStats().then(setStats).catch(() => {});
      getCandidates().then(setCandidates).catch(() => {});
    });

    es.addEventListener('diary_sync', () => {
      getDiaries().catch(() => {});
      getStats().then(setStats).catch(() => {});
    });

    es.addEventListener('todo_update', () => {
      getTodos().then(setTodos).catch(() => {});
      getStats().then(setStats).catch(() => {});
    });

    es.addEventListener('candidate_new', () => {
      getCandidates().then(setCandidates).catch(() => {});
      getStats().then(setStats).catch(() => {});
    });

    return () => {
      es.close();
      esRef.current = null;
      setSseConnected(false);
    };
  }, []);

  async function handleAccept(id: number) {
    await acceptCandidate(id);
    setCandidates((prev) => prev.filter((c) => c.id !== id));
  }
  async function handleReject(id: number) {
    await rejectCandidate(id);
    setCandidates((prev) => prev.filter((c) => c.id !== id));
  }
  async function handleTodoStatus(id: number, status: string) {
    await updateTodoStatus(id, status);
    setTodos((prev) => prev.filter((t) => t.id !== id || (status === 'active' && t.status === 'active')));
    loadAll();
  }

  function handleLogout() {
    clearAuthToken();
    window.location.reload();
  }

  const groupedTodos = BOARD_COLUMNS.slice(1).reduce<Record<string, Todo[]>>((acc, col) => {
    acc[col.key] = (todos || []).filter((t) => t.status === col.key.replace('-', '_'));
    return acc;
  }, {});
  const visibleCandidates = candidates.slice(0, INBOX_PREVIEW_LIMIT);
  const hiddenCandidateCount = Math.max(candidates.length - visibleCandidates.length, 0);

  if (loading) return <div className="loading">加载中...</div>;
  if (error) return <div className="error">{error}</div>;

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="meta" style={{ fontSize: '0.72rem', fontWeight: 800, textTransform: 'uppercase', marginBottom: 8 }}>Daily Closure</div>
          <h1>
            {stats?.today.exists
              ? '今天已写日记。'
              : '今天还没有写日记。'}
            {stats?.candidateCount === 0 ? ' AI Inbox 已清空。' : ''}
          </h1>
          <p style={{ color: 'var(--text)', margin: '8px 0 0', maxWidth: 760 }}>
            {stats?.todoCounts.active} 个 active，{stats?.todoCounts.in_progress} 个正在做，{stats?.todoCounts.done} 个已完成，{stats?.todoCounts.wont_do} 个不打算做，{stats?.todoCounts.archived} 个已归档。日记共 {stats?.diaryCount} 篇，长期记忆 {stats?.memoryCount} 条。
          </p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div className="sse-status" title={sseConnected ? '实时连接中' : '实时连接已断开'}>
            <span className={`sse-dot ${sseConnected ? 'sse-dot--connected' : 'sse-dot--disconnected'}`} />
            <span className="sse-label">{sseConnected ? '实时连接' : '已断开'}</span>
          </div>
          <button className="logout-button" onClick={handleLogout}>退出登录</button>
          <div className="meta">Generated {new Date().toLocaleString('zh-CN')}</div>
        </div>
      </div>

      <div className="metric-row">
        <div className="metric"><strong>{stats?.candidateCount}</strong><span>AI Inbox</span></div>
        <div className="metric"><strong>{stats?.todoCounts.active}</strong><span>Active todos</span></div>
        <div className="metric"><strong>{stats?.todoCounts.in_progress}</strong><span>In progress</span></div>
        <div className="metric"><strong>{stats?.todoCounts.done}</strong><span>Done todos</span></div>
        <div className="metric"><strong>{stats?.todoCounts.wont_do}</strong><span> Won't do</span></div>
        <div className="metric"><strong>{stats?.diaryCount}</strong><span>Diary entries</span></div>
      </div>

      <section className="board-section">
        <div className="board-grid">
          {/* Inbox column: candidates */}
          <div className="board-column board-inbox">
            <header>
              <h3>AI Inbox</h3>
              <span className="board-count">{candidates.length}</span>
            </header>
            <p style={{ fontSize: '0.78rem', color: 'var(--muted)', margin: '0 0 10px' }}>
              AI 提取出的建议，只在你提升后才进入可信 todo/memory。这里只展示最需要处理的前 {visibleCandidates.length} 条。
            </p>
            {candidates.length === 0 ? (
              <div className="empty">没有待提升候选。</div>
            ) : (
              <>
                {visibleCandidates.map((c) => (
                  <div className="board-card" key={c.id}>
                    <div className="card-meta">
                      <span>{c.type.toUpperCase()}</span>
                      <span className="card-id">#{c.id}</span>
                    </div>
                    <strong>{c.title || c.content.slice(0, 60)}</strong>
                    {c.evidenceText && <p>{c.evidenceText}</p>}
                    <div className="source">{c.sourceDate}</div>
                    <div className="actions">
                      <button onClick={() => handleAccept(c.id)}>提升</button>
                      <button onClick={() => handleReject(c.id)}>丢弃</button>
                    </div>
                  </div>
                ))}
                {hiddenCandidateCount > 0 && (
                  <div className="empty">还有 {hiddenCandidateCount} 条保留在后台，不需要一次性处理完。</div>
                )}
              </>
            )}
          </div>

          {/* Todo status columns */}
          {BOARD_COLUMNS.slice(1).map((col) => (
            <div className={`board-column ${col.color}`} key={col.key}>
              <header>
                <h3>{col.title}</h3>
                <span className="board-count">{groupedTodos[col.key]?.length ?? 0}</span>
              </header>
              <p style={{ fontSize: '0.78rem', color: 'var(--muted)', margin: '0 0 10px' }}>{col.desc}。</p>
              {(groupedTodos[col.key]?.length ?? 0) === 0 ? (
                <div className="empty">没有 {col.title.toLowerCase()} todo。</div>
              ) : (
                groupedTodos[col.key].map((t) => (
                  <div className="board-card" key={t.id}>
                    <div className="card-meta">
                      <span>TODO</span>
                      <div className="card-right">
                        {t.hasPriority && <span className="priority-badge">P{t.priority}</span>}
                        <span className="card-id">#{t.id}</span>
                      </div>
                    </div>
                    <strong>{t.text}</strong>
                    {t.evidenceText && <p>{t.evidenceText}</p>}
                    <div className="source">{t.sourceDate}</div>
                    <div className="actions">
                      {t.status !== 'done' && <button onClick={() => handleTodoStatus(t.id, 'done')}>完成</button>}
                      {t.status !== 'archived' && <button onClick={() => handleTodoStatus(t.id, 'archived')}>归档</button>}
                      {t.status !== 'wont_do' && <button onClick={() => handleTodoStatus(t.id, 'wont_do')}>不做</button>}
                    </div>
                  </div>
                ))
              )}
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
