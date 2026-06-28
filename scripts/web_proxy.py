#!/usr/bin/env python3
"""
Dear Diary Web Proxy — 临时 Python 代理服务器。

提供与 Go 后端相同的 API，并 serve React SPA 静态文件。
用法:
    python3 scripts/web_proxy.py
    python3 scripts/web_proxy.py --calendar    # 打开日历
    python3 scripts/web_proxy.py --no-open     # 只启动服务

停止: Ctrl+C
"""

import argparse
import http.server
import json
import os
import sqlite3
import subprocess
import sys
from datetime import datetime
from urllib.parse import parse_qs, urlparse

# 配置
DIARY_DIR = os.path.expanduser("~/Documents/dear-diary")
DB_PATH = os.path.expanduser("~/.local/share/dear-diary/process.db")
STATIC_DIR = os.path.join(os.path.dirname(__file__), "..", "internal", "web", "dist")
PORT = 8765


def get_db():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def diary_path(date_str):
    t = datetime.strptime(date_str, "%Y-%m-%d")
    month = t.strftime("%Y-%m")
    return os.path.join(DIARY_DIR, month, f"{date_str}.md")


def is_diary_path(path):
    name = os.path.basename(path)
    if not name.endswith(".md") or len(name) != len("YYYY-MM-DD.md"):
        return False
    try:
        datetime.strptime(name[:-3], "%Y-%m-%d")
    except ValueError:
        return False
    return os.path.basename(os.path.dirname(path)) == name[:-3][:7]


def date_from_path(path):
    return os.path.basename(path)[:-3]


def render_markdown(text):
    import re
    text = re.sub(r'&', '&amp;', text)
    text = re.sub(r'<', '&lt;', text)
    text = re.sub(r'>', '&gt;', text)
    text = re.sub(r'^## (.+)$', r'<h2>\1</h2>', text, flags=re.M)
    text = re.sub(r'^# (.+)$', r'<h1>\1</h1>', text, flags=re.M)
    text = re.sub(r'\*\*(.+?)\*\*', r'<strong>\1</strong>', text)
    text = re.sub(r'\*(.+?)\*', r'<em>\1</em>', text)
    text = re.sub(r'`(.+?)`', r'<code>\1</code>', text)
    text = re.sub(r'\[(.+?)\]\((.+?)\)', r'<a href="\2">\1</a>', text)
    text = text.replace('\n\n', '</p><p>')
    text = '<p>' + text + '</p>'
    text = text.replace('\n', '<br>')
    return text


def summarize_diary(text):
    lines = text.split('\n')
    title = "Untitled diary"
    body_start = 0
    if lines and lines[0].strip().startswith('# '):
        title = lines[0].strip()[2:].strip()
        body_start = 1
    body = '\n'.join(lines[body_start:])
    plain = []
    for line in lines[body_start:]:
        s = line.strip()
        if s and not s.startswith('#'):
            s = s.lstrip('- ').lstrip('* ')
            if s:
                plain.append(s)
    excerpt = ' '.join(plain)[:150]
    if len(' '.join(plain)) > 150:
        excerpt += '...'
    html = render_markdown(body)
    return title, excerpt, html


class APIHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=STATIC_DIR, **kwargs)

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        if path.startswith('/api/'):
            return self.handle_api(path, parsed.query)
        if path != '/' and not os.path.exists(os.path.join(STATIC_DIR, path.lstrip('/'))):
            self.path = '/index.html'
        return super().do_GET()

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path
        if path.startswith('/api/'):
            return self.handle_api_post(path)
        self.send_error(404)

    def send_json(self, code, data):
        self.send_response(code)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        self.wfile.write(json.dumps(data, ensure_ascii=False, default=str).encode('utf-8'))

    def handle_api(self, path, query):
        try:
            if path == '/api/stats':
                return self.api_stats()
            elif path == '/api/todos':
                return self.api_todos()
            elif path == '/api/candidates':
                return self.api_candidates()
            elif path == '/api/memories':
                return self.api_memories()
            elif path == '/api/diaries':
                return self.api_diaries()
            elif path.startswith('/api/diaries/'):
                return self.api_diary(path[len('/api/diaries/'):])
            elif path == '/api/calendar':
                return self.api_calendar()
            elif path == '/api/search':
                return self.api_search(parse_qs(query).get('q', [''])[0])
            else:
                self.send_error(404)
        except Exception as e:
            self.send_json(500, {"error": str(e)})

    def handle_api_post(self, path):
        try:
            parts = path.split('/')
            if len(parts) >= 5 and parts[3] == 'todos' and parts[5] == 'status':
                return self.api_update_todo_status(int(parts[4]))
            elif len(parts) >= 5 and parts[3] == 'candidates':
                cid = int(parts[4])
                action = parts[5] if len(parts) > 5 else ''
                if action == 'accept':
                    return self.api_accept_candidate(cid)
                elif action == 'reject':
                    return self.api_reject_candidate(cid)
            self.send_error(404)
        except Exception as e:
            self.send_json(500, {"error": str(e)})

    def api_stats(self):
        now = datetime.now()
        path = diary_path(now.strftime("%Y-%m-%d"))
        exists = os.path.exists(path)
        sections = 0
        if exists:
            with open(path) as f:
                content = f.read()
                sections = content.count('\n## ')
                if content.startswith('## '):
                    sections += 1
        conn = get_db()
        cur = conn.cursor()
        c = cur.execute("SELECT COUNT(*) FROM ai_candidates WHERE status='pending'").fetchone()[0]
        tc = cur.execute("SELECT status, COUNT(*) FROM todos GROUP BY status").fetchall()
        todo_counts = {'active':0, 'in_progress':0, 'done':0, 'wont_do':0, 'archived':0, 'other':0}
        for row in tc:
            todo_counts[row[0]] = row[1]
        m = cur.execute("SELECT COUNT(*) FROM memories WHERE COALESCE(status,'active')='active'").fetchone()[0]
        conn.close()
        dcount = 0
        for root, dirs, files in os.walk(DIARY_DIR):
            for f in files:
                if is_diary_path(os.path.join(root, f)):
                    dcount += 1
        self.send_json(200, {
            "today": {"date": now.strftime("%Y-%m-%d"), "exists": exists, "sections": sections},
            "candidateCount": c, "todoCounts": todo_counts, "memoryCount": m, "diaryCount": dcount,
        })

    def api_todos(self):
        conn = get_db()
        rows = conn.execute("""
            SELECT id, text, status, source_file, source_date, source_hash, evidence_text, created_at, updated_at, priority
            FROM todos ORDER BY CASE WHEN priority IS NULL THEN 1 ELSE 0 END, priority ASC, updated_at DESC
        """).fetchall()
        conn.close()
        result = []
        for row in rows:
            result.append({
                "id": row[0], "text": row[1], "status": row[2],
                "sourceFile": row[3] or "", "sourceDate": row[4] or "", "sourceHash": row[5] or "",
                "evidenceText": row[6] or "", "createdAt": row[7], "updatedAt": row[8],
                "hasPriority": row[9] is not None, "priority": row[9] or 0,
            })
        self.send_json(200, result)

    def api_update_todo_status(self, todo_id):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        status = json.loads(body.decode()).get('status', '')
        conn = get_db()
        completed = datetime.now() if status == 'done' else None
        conn.execute("UPDATE todos SET status=?, completed_at=?, updated_at=datetime('now') WHERE id=?", (status, completed, todo_id))
        conn.commit()
        conn.close()
        self.send_json(200, {"ok": True})

    def api_candidates(self):
        conn = get_db()
        rows = conn.execute("""
            SELECT id, candidate_type, title, content, status, source_file, source_date, source_hash,
                   evidence_text, raw_ai_json, confidence, content_key, final_item_type, final_item_id,
                   created_at, updated_at
            FROM ai_candidates WHERE status='pending' ORDER BY created_at ASC
        """).fetchall()
        conn.close()
        result = []
        for row in rows:
            result.append({
                "id": row[0], "type": row[1], "title": row[2] or "", "content": row[3],
                "status": row[4], "sourceFile": row[5] or "", "sourceDate": row[6] or "",
                "sourceHash": row[7] or "", "evidenceText": row[8] or "",
                "rawAIJSON": row[9] or "", "confidence": row[10] or 0,
                "contentKey": row[11] or "", "finalItemType": row[12] or "",
                "finalItemID": row[13] or 0, "createdAt": row[14], "updatedAt": row[15],
            })
        self.send_json(200, result)

    def api_accept_candidate(self, cid):
        conn = get_db()
        row = conn.execute("SELECT candidate_type, title, content, source_file, source_date, source_hash, evidence_text FROM ai_candidates WHERE id=? AND status='pending'", (cid,)).fetchone()
        if not row:
            conn.close()
            return self.send_json(400, {"error": "not pending"})
        c_type, title, content, src_file, src_date, src_hash, evidence = row
        now = datetime.now()
        if c_type == 'todo':
            text = title or content
            conn.execute("INSERT INTO todos (text, status, source_file, source_date, source_hash, evidence_text, created_from_candidate_id, created_at, updated_at) VALUES (?, 'active', ?, ?, ?, ?, ?, ?, ?)", (text, src_file, src_date, src_hash, evidence, cid, now, now))
            final_id = conn.execute("SELECT last_insert_rowid()").fetchone()[0]
            conn.execute("UPDATE ai_candidates SET status='accepted', final_item_type='todo', final_item_id=?, updated_at=? WHERE id=?", (final_id, now, cid))
        elif c_type == 'memory':
            topic = title or "Untitled"
            conn.execute("INSERT INTO memories (topic, summary, status, source_file, source_date, source_hash, evidence_text, created_from_candidate_id, created_at, updated_at) VALUES (?, ?, 'active', ?, ?, ?, ?, ?, ?, ?)", (topic, content, src_file, src_date, src_hash, evidence, cid, now, now))
            final_id = conn.execute("SELECT last_insert_rowid()").fetchone()[0]
            conn.execute("UPDATE ai_candidates SET status='accepted', final_item_type='memory', final_item_id=?, updated_at=? WHERE id=?", (final_id, now, cid))
        conn.commit()
        conn.close()
        self.send_json(200, {"ok": True})

    def api_reject_candidate(self, cid):
        conn = get_db()
        conn.execute("UPDATE ai_candidates SET status='rejected', updated_at=datetime('now') WHERE id=? AND status='pending'", (cid,))
        conn.commit()
        conn.close()
        self.send_json(200, {"ok": True})

    def api_memories(self):
        conn = get_db()
        rows = conn.execute("SELECT id, topic, summary, status, source_file, source_date, source_hash, evidence_text, created_at, updated_at FROM memories WHERE COALESCE(status,'active')='active' ORDER BY created_at DESC").fetchall()
        conn.close()
        result = []
        for row in rows:
            result.append({
                "id": row[0], "topic": row[1], "summary": row[2], "status": row[3] or "active",
                "sourceFile": row[4] or "", "sourceDate": row[5] or "", "sourceHash": row[6] or "",
                "evidenceText": row[7] or "", "createdAt": row[8], "updatedAt": row[9],
            })
        self.send_json(200, result)

    def api_diaries(self):
        entries = []
        for root, dirs, files in os.walk(DIARY_DIR):
            for f in sorted(files, reverse=True):
                path = os.path.join(root, f)
                if not is_diary_path(path):
                    continue
                with open(path) as fh:
                    content = fh.read()
                date_str = date_from_path(path)
                title, excerpt, html = summarize_diary(content)
                entries.append({"date": date_str, "title": title, "excerpt": excerpt, "html": html, "month": os.path.basename(os.path.dirname(path))})
                if len(entries) >= 30:
                    break
            if len(entries) >= 30:
                break
        self.send_json(200, entries)

    def api_diary(self, date_str):
        path = diary_path(date_str)
        if not os.path.exists(path):
            return self.send_json(404, {"error": "not found"})
        with open(path) as f:
            content = f.read()
        title, excerpt, html = summarize_diary(content)
        self.send_json(200, {"date": date_str, "title": title, "excerpt": excerpt, "html": html, "month": os.path.basename(os.path.dirname(path))})

    def api_calendar(self):
        written = {}
        month_starts = {}
        for root, dirs, files in os.walk(DIARY_DIR):
            for f in files:
                path = os.path.join(root, f)
                if not is_diary_path(path):
                    continue
                d = date_from_path(path)
                written[d] = True
                month = d[:7]
                t = datetime.strptime(d, "%Y-%m-%d")
                month_starts[month] = datetime(t.year, t.month, 1)
        now = datetime.now()
        current_month = now.strftime("%Y-%m")
        if current_month not in month_starts:
            month_starts[current_month] = datetime(now.year, now.month, 1)
        months = sorted(month_starts.keys(), reverse=True)
        result = []
        for month in months:
            start = month_starts[month]
            if start.month == 12:
                next_month = datetime(start.year + 1, 1, 1)
            else:
                next_month = datetime(start.year, start.month + 1, 1)
            days_in_month = (next_month - start).days
            first_weekday = (start.weekday() + 6) % 7
            days = []
            for _ in range(first_weekday):
                days.append({"isPadding": True})
            count = 0
            for day in range(1, days_in_month + 1):
                date_obj = datetime(start.year, start.month, day)
                date_str = date_obj.strftime("%Y-%m-%d")
                is_written = date_str in written
                if is_written:
                    count += 1
                days.append({"day": day, "date": date_str, "isPadding": False, "isWritten": is_written, "isToday": date_str == now.strftime("%Y-%m-%d")})
            result.append({"month": month, "title": f"{start.year} 年 {start.month} 月", "count": count, "daysInMonth": days_in_month, "days": days})
        self.send_json(200, result)

    def api_search(self, q):
        if not q:
            return self.send_json(200, [])
        results = []
        kw = q.lower()
        for root, dirs, files in os.walk(DIARY_DIR):
            for f in files:
                path = os.path.join(root, f)
                if not is_diary_path(path):
                    continue
                with open(path) as fh:
                    lines = fh.readlines()
                matched = []
                for i, line in enumerate(lines, 1):
                    if kw in line.lower():
                        matched.append({"lineNum": i, "text": line.rstrip('\n')})
                if matched:
                    date_str = date_from_path(path)
                    results.append({"date": date_str, "title": date_str, "lines": matched})
        results.sort(key=lambda x: x["date"], reverse=True)
        self.send_json(200, results)


def open_browser(url):
    import platform
    if platform.system() == 'Darwin':
        subprocess.run(['open', url])
    elif platform.system() == 'Windows':
        subprocess.run(['cmd', '/c', 'start', url])
    else:
        subprocess.run(['xdg-open', url])


def main():
    parser = argparse.ArgumentParser(description='Dear Diary Web Proxy')
    parser.add_argument('--calendar', action='store_true', help='打开日历页面')
    parser.add_argument('--no-open', action='store_true', help='不自动打开浏览器')
    args = parser.parse_args()

    page = 'calendar' if args.calendar else 'dashboard'
    url = f"http://localhost:{PORT}/#/{page}"

    if not args.no_open:
        import urllib.request
        try:
            urllib.request.urlopen(f"http://localhost:{PORT}/api/stats", timeout=1)
            print(f"Web 服务已在运行: {url}")
            open_browser(url)
            return
        except Exception:
            pass

    if not args.no_open:
        import threading
        def delayed_open():
            import time
            time.sleep(0.8)
            open_browser(url)
        threading.Thread(target=delayed_open, daemon=True).start()

    print("正在启动 Dear Diary Web 服务...")
    print(f"  看板: http://localhost:{PORT}/#/dashboard")
    print(f"  日历: http://localhost:{PORT}/#/calendar")
    print(f"  搜索: http://localhost:{PORT}/#/search")
    print("按 Ctrl+C 停止服务")

    server = http.server.ThreadingHTTPServer(('', PORT), APIHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n服务已停止")


if __name__ == '__main__':
    main()
