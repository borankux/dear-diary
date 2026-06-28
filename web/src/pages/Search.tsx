import { useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { searchDiaries, type SearchResult } from '../api';

export default function Search() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [searched, setSearched] = useState(false);

  const doSearch = useCallback(async () => {
    if (!query.trim()) return;
    try {
      setLoading(true);
      setError('');
      const data = await searchDiaries(query.trim());
      setResults(data);
      setSearched(true);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '搜索失败');
    } finally {
      setLoading(false);
    }
  }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') doSearch();
  };

  return (
    <div>
      <div className="page-header">
        <h1>搜索</h1>
        <div className="meta">全文搜索所有日记</div>
      </div>

      <div className="search-box">
        <input
          type="text"
          placeholder="输入关键词..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <button onClick={doSearch} disabled={loading}>
          {loading ? '搜索中...' : '搜索'}
        </button>
      </div>

      {error && <div className="error">{error}</div>}

      {searched && results.length === 0 && !loading && (
        <div className="card">
          <div className="empty">没有找到匹配 "{query}" 的日记。</div>
        </div>
      )}

      {results.map((r) => (
        <div className="search-result" key={r.date}>
          <h3>
            <Link to={`/diary/${r.date}`}>{r.date} — {r.title}</Link>
          </h3>
          {r.lines.map((line, i) => (
            <div className="line" key={i}>{line.lineNum}: {line.text}</div>
          ))}
        </div>
      ))}
    </div>
  );
}
