import { useEffect, useState } from 'react';
import { getMemories, type Memory } from '../api';

export default function Memories() {
  const [memories, setMemories] = useState<Memory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    getMemories()
      .then((m) => { setMemories(m); setError(''); })
      .catch((e) => setError(e instanceof Error ? e.message : '加载失败'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="loading">加载中...</div>;
  if (error) return <div className="error">{error}</div>;

  return (
    <div>
      <div className="page-header">
        <h1>长期记忆</h1>
        <div className="meta">{memories.length} 条</div>
      </div>

      <div className="memory-list">
        {memories.map((m) => (
          <div className="memory-card" key={m.id}>
            <strong>{m.topic}</strong>
            <p>{m.summary}</p>
            <div className="source">{m.sourceDate}</div>
          </div>
        ))}
      </div>

      {memories.length === 0 && (
        <div className="card">
          <div className="empty">还没有长期记忆。</div>
        </div>
      )}
    </div>
  );
}
