import { useEffect, useState, useRef, type ReactNode } from 'react';
import {
  Archive,
  Ban,
  BookOpen,
  CalendarDays,
  Check,
  CheckCircle2,
  CircleDot,
  Clock3,
  EyeOff,
  GitMerge,
  Inbox,
  ListTodo,
  MoreHorizontal,
  Play,
  RotateCcw,
  Sparkles,
  Sprout,
  Upload,
  Wifi,
  WifiOff,
  type LucideIcon,
} from 'lucide-react';
import { getStats, getTodos, getCandidates, getDiaries, acceptCandidate, rejectCandidate, updateTodoStatus, mergeDuplicateCandidates, promoteAllCandidates, type Stats, type Todo, type Candidate, type MergeResult } from '../api';

const BOARD_COLUMNS: Array<{ key: string; title: string; desc: string; color: string; icon: LucideIcon }> = [
  { key: 'active', title: 'Active', desc: '已接收，尚未开始或未分类的事项', color: 'board-active', icon: CircleDot },
  { key: 'in-progress', title: 'In Progress', desc: '正在做，应该优先保持可见', color: 'board-in-progress', icon: Clock3 },
  { key: 'done', title: 'Done', desc: '最近完成的事项，用来看见闭环', color: 'board-done', icon: CheckCircle2 },
] as const;

const INBOX_PREVIEW_LIMIT = 24;
type TodoCountKey = keyof Stats['todoCounts'];
const TODO_COUNT_KEYS: TodoCountKey[] = ['active', 'in_progress', 'done', 'wont_do', 'archived', 'other'];

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [todos, setTodos] = useState<Todo[]>([]);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [sseConnected, setSseConnected] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkMessage, setBulkMessage] = useState('');
  const [pendingTodoIds, setPendingTodoIds] = useState<Set<number>>(new Set());
  const esRef = useRef<EventSource | null>(null);

  async function loadAll(options: { showLoading?: boolean } = {}) {
    const showLoading = options.showLoading ?? false;
    try {
      if (showLoading) setLoading(true);
      const [s, t, c] = await Promise.all([getStats(), getTodos(), getCandidates()]);
      setStats(s);
      setTodos(t);
      setCandidates(c);
      setError('');
    } catch (e: unknown) {
      if (showLoading || !stats) {
        setError(e instanceof Error ? e.message : '加载失败');
      }
    } finally {
      if (showLoading) setLoading(false);
    }
  }

  useEffect(() => { loadAll({ showLoading: true }); }, []);

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
    void loadAll();
  }
  async function handleReject(id: number) {
    await rejectCandidate(id);
    setCandidates((prev) => prev.filter((c) => c.id !== id));
    void loadAll();
  }
  async function handleMergeDuplicates() {
    setBulkBusy(true);
    setBulkMessage('');
    try {
      const result = await mergeDuplicateCandidates();
      setBulkMessage(`已合并 ${mergeTotal(result.merged)} 条重复项。`);
      await loadAll();
    } finally {
      setBulkBusy(false);
    }
  }
  async function handlePromoteAll() {
    if (!window.confirm(`将 ${candidates.length} 条 AI Inbox 候选一次性提升为 Todo/Memory，并自动归档明显重复项。继续吗？`)) {
      return;
    }
    setBulkBusy(true);
    setBulkMessage('');
    try {
      const result = await promoteAllCandidates();
      setBulkMessage(`已提升 ${result.promotedTodos} 个 Todo、${result.promotedMemories} 条 Memory；合并 ${mergeTotal(result.merge)} 条重复项。`);
      await loadAll();
    } finally {
      setBulkBusy(false);
    }
  }
  async function handleTodoStatus(id: number, status: string) {
    const current = todos.find((t) => t.id === id);
    if (!current || pendingTodoIds.has(id)) {
      return;
    }

    const previousTodos = todos;
    const previousStats = stats;
    setTodoPending(id, true);
    setTodos((prev) => prev.map((t) => (t.id === id ? { ...t, status } : t)));
    setStats((prev) => adjustStatsForTodoStatus(prev, current.status, status));
    setBulkMessage('');

    try {
      await updateTodoStatus(id, status);
      void loadAll();
    } catch (e: unknown) {
      setTodos(previousTodos);
      setStats(previousStats);
      setBulkMessage(e instanceof Error ? `更新失败：${e.message}` : '更新失败');
    } finally {
      setTodoPending(id, false);
    }
  }

  function setTodoPending(id: number, pending: boolean) {
    setPendingTodoIds((prev) => {
      const next = new Set(prev);
      if (pending) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  const groupedTodos = BOARD_COLUMNS.reduce<Record<string, Todo[]>>((acc, col) => {
    acc[col.key] = (todos || []).filter((t) => todoBelongsToColumn(t, col.key));
    return acc;
  }, {});
  const visibleCandidates = candidates.slice(0, INBOX_PREVIEW_LIMIT);
  const hiddenCandidateCount = Math.max(candidates.length - visibleCandidates.length, 0);
  const lastSync = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });

  if (loading && !stats) return <div className="loading">加载中...</div>;
  if (error && !stats) return <div className="error">{error}</div>;

  return (
    <div>
      <section className="dashboard-hero">
        <div className="hero-copy">
          <div className="hero-eyebrow">Daily Closure</div>
          <h1>
            {stats?.today.exists
              ? '今天已写日记。'
              : '今天还没有写日记。'}
            {stats?.candidateCount === 0 ? ' AI Inbox 已清空。' : ''}
            <Sparkles className="hero-sparkle" size={22} strokeWidth={1.8} />
          </h1>
          <p>
            主看板只显示 Active / In Progress / Done；其他未分类会并入 Active。已收起 {closedTodoCount(stats)} 个：{stats?.todoCounts.archived} 个归档，{stats?.todoCounts.wont_do} 个不做。日记共 {stats?.diaryCount} 篇，长期记忆 {stats?.memoryCount} 条。
          </p>
          <div className="hero-meta-row">
            <span><CalendarDays size={15} />{formatHeroDate(stats?.today.date)}</span>
            <span>长期记忆 {stats?.memoryCount ?? 0} 条</span>
          </div>
        </div>
        <div className="hero-status">
          <div className="sse-status" title={sseConnected ? '实时连接中' : '实时连接已断开'}>
            {sseConnected ? <Wifi size={14} /> : <WifiOff size={14} />}
            <span className={`sse-dot ${sseConnected ? 'sse-dot--connected' : 'sse-dot--disconnected'}`} />
            <span className="sse-label">{sseConnected ? '实时连接' : '已断开'}</span>
          </div>
          <div className="sync-pill">最后同步 {lastSync}</div>
        </div>
      </section>

      <div className="metric-row">
        <MetricCard icon={<Inbox size={21} />} value={stats?.candidateCount ?? 0} label="AI Inbox" tone="inbox" />
        <MetricCard icon={<ListTodo size={21} />} value={openTodoCount(stats)} label="Open todos" tone="active" />
        <MetricCard icon={<Clock3 size={21} />} value={stats?.todoCounts.in_progress ?? 0} label="In progress" tone="progress" />
        <MetricCard icon={<CheckCircle2 size={21} />} value={stats?.todoCounts.done ?? 0} label="Done todos" tone="done" />
        <MetricCard icon={<EyeOff size={21} />} value={closedTodoCount(stats)} label="Hidden closed" tone="hidden" />
        <MetricCard icon={<BookOpen size={21} />} value={stats?.diaryCount ?? 0} label="Diary entries" tone="diary" />
      </div>

      <div className="dashboard-tools">
        <button onClick={handleMergeDuplicates} disabled={bulkBusy}><GitMerge size={15} />合并重复项</button>
        {candidates.length > 0 && <button className="bulk-primary" onClick={handlePromoteAll} disabled={bulkBusy}><Upload size={15} />全部提升 AI Inbox</button>}
        {bulkMessage && <div className="bulk-message">{bulkMessage}</div>}
      </div>

      {candidates.length > 0 && (
        <section className="inbox-section">
          <div className="section-title">
            <h2>AI Inbox</h2>
            <span>{candidates.length} 条候选</span>
          </div>
          <p className="section-note">AI 提取出的建议，只在你提升后才进入可信 todo/memory。这里只展示最需要处理的前 {visibleCandidates.length} 条。</p>
          <div className="inbox-grid">
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
          </div>
          {hiddenCandidateCount > 0 && (
            <div className="empty">还有 {hiddenCandidateCount} 条保留在后台，不需要一次性处理完。</div>
          )}
        </section>
      )}

      <section className="board-section">
        <div className="board-grid board-grid--primary">
          {BOARD_COLUMNS.map((col) => {
            const ColumnIcon = col.icon;
            return (
              <div className={`board-column ${col.color}`} key={col.key}>
                <header>
                  <h3>
                    <span className="column-icon"><ColumnIcon size={16} strokeWidth={2.3} /></span>
                    {col.title}
                  </h3>
                  <span className="board-count">{groupedTodos[col.key]?.length ?? 0}</span>
                </header>
                <p style={{ fontSize: '0.78rem', color: 'var(--muted)', margin: '0 0 10px' }}>{col.desc}。</p>
                {(groupedTodos[col.key]?.length ?? 0) === 0 ? (
                  <div className="empty empty-state">
                    <Sprout size={46} strokeWidth={1.5} />
                    <strong>{emptyColumnTitle(col.key)}</strong>
                    <span>{col.key === 'in-progress' ? '专注当下，一件一件来。' : '这里暂时很干净。'}</span>
                  </div>
                ) : (
                  groupedTodos[col.key].map((t) => (
                    <div className={`board-card ${pendingTodoIds.has(t.id) ? 'is-pending' : ''}`} key={t.id}>
                      <div className="card-meta">
                        <span className={`card-status-dot card-status-dot--${todoStatusTone(t.status)}`}>
                          {t.status === 'done' ? <CheckCircle2 size={14} /> : <CircleDot size={13} />}
                          #{t.id}
                        </span>
                        <div className="card-right">
                          {t.hasPriority && <span className="priority-badge">P{t.priority}</span>}
                          <span className="card-menu" aria-hidden="true"><MoreHorizontal size={16} /></span>
                        </div>
                      </div>
                      <strong>{t.text}</strong>
                      {t.evidenceText && <p>{t.evidenceText}</p>}
                      <div className="source">{t.sourceDate}</div>
                      <div className="actions">
                        {(t.status === 'active' || t.status === 'other') && <button disabled={pendingTodoIds.has(t.id)} onClick={() => handleTodoStatus(t.id, 'in_progress')}><Play size={13} />开始</button>}
                        {t.status === 'in_progress' && <button disabled={pendingTodoIds.has(t.id)} onClick={() => handleTodoStatus(t.id, 'active')}><RotateCcw size={13} />放回</button>}
                        {t.status !== 'done' && <button disabled={pendingTodoIds.has(t.id)} onClick={() => handleTodoStatus(t.id, 'done')}><Check size={13} />完成</button>}
                        {t.status !== 'archived' && <button disabled={pendingTodoIds.has(t.id)} onClick={() => handleTodoStatus(t.id, 'archived')}><Archive size={13} />归档</button>}
                        {t.status !== 'wont_do' && <button disabled={pendingTodoIds.has(t.id)} onClick={() => handleTodoStatus(t.id, 'wont_do')}><Ban size={13} />不做</button>}
                      </div>
                    </div>
                  ))
                )}
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}

function mergeTotal(result: MergeResult) {
  return result.candidateMerged + result.todoMerged + result.memoryMerged;
}

function MetricCard({ icon, value, label, tone }: { icon: ReactNode; value: number; label: string; tone: string }) {
  return (
    <div className={`metric metric--${tone}`}>
      <span className="metric-icon">{icon}</span>
      <div>
        <strong>{value}</strong>
        <span>{label}</span>
      </div>
    </div>
  );
}

function openTodoCount(stats: Stats | null) {
  if (!stats) return 0;
  return stats.todoCounts.active + stats.todoCounts.in_progress + stats.todoCounts.other;
}

function closedTodoCount(stats: Stats | null) {
  if (!stats) return 0;
  return stats.todoCounts.archived + stats.todoCounts.wont_do;
}

function todoBelongsToColumn(todo: Todo, columnKey: string) {
  if (columnKey === 'active') {
    return todo.status === 'active' || todo.status === 'other';
  }
  return todo.status === columnKey.replace('-', '_');
}

function todoStatusTone(status: string) {
  if (status === 'done') return 'done';
  if (status === 'in_progress') return 'progress';
  return 'active';
}

function emptyColumnTitle(columnKey: string) {
  if (columnKey === 'in-progress') return '没有进行中的事项';
  if (columnKey === 'done') return '没有已完成事项';
  return '没有待处理事项';
}

function formatHeroDate(date?: string) {
  if (!date) return '今天';
  const parsed = new Date(`${date}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) return date;
  return parsed.toLocaleDateString('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    weekday: 'short',
  });
}

function adjustStatsForTodoStatus(stats: Stats | null, fromStatus: string, toStatus: string): Stats | null {
  if (!stats) return stats;
  const from = normalizeTodoCountKey(fromStatus);
  const to = normalizeTodoCountKey(toStatus);
  if (from === to) return stats;
  return {
    ...stats,
    todoCounts: {
      ...stats.todoCounts,
      [from]: Math.max(0, stats.todoCounts[from] - 1),
      [to]: stats.todoCounts[to] + 1,
    },
  };
}

function normalizeTodoCountKey(status: string): TodoCountKey {
  const key = status.replace('-', '_') as TodoCountKey;
  return TODO_COUNT_KEYS.includes(key) ? key : 'other';
}
